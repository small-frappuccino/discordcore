package qotd

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

const (
	runtimePublishInterval   = time.Minute
	runtimeReconcileInterval = 15 * time.Minute
	runtimeOperationTimeout  = 45 * time.Second
)

type GuildLifecycleService interface {
	PublishScheduledIfDue(ctx context.Context, guildID string, session *discordgo.Session) (bool, error)
	ReconcileGuild(ctx context.Context, guildID string, session *discordgo.Session) error
}

type RuntimeService struct {
	session          *discordgo.Session
	configManager    *files.ConfigManager
	lifecycleService GuildLifecycleService
	botInstanceID    string
	defaultBotID     string
	now              func() time.Time
	publishInterval  time.Duration
	reconcileEvery   time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup

	mu      sync.RWMutex
	running bool
}

func NewRuntimeService(session *discordgo.Session, configManager *files.ConfigManager, lifecycleService GuildLifecycleService) *RuntimeService {
	return NewRuntimeServiceForBot(session, configManager, lifecycleService, "", "")
}

func NewRuntimeServiceForBot(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	lifecycleService GuildLifecycleService,
	botInstanceID string,
	defaultBotInstanceID string,
) *RuntimeService {
	return &RuntimeService{
		session:          session,
		configManager:    configManager,
		lifecycleService: lifecycleService,
		botInstanceID:    files.NormalizeBotInstanceID(botInstanceID),
		defaultBotID:     files.NormalizeBotInstanceID(defaultBotInstanceID),
		now: func() time.Time {
			return time.Now().UTC()
		},
		publishInterval: runtimePublishInterval,
		reconcileEvery:  runtimeReconcileInterval,
		stopCh:          make(chan struct{}),
	}
}

func (s *RuntimeService) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.loop()
}

func (s *RuntimeService) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func (s *RuntimeService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *RuntimeService) loop() {
	defer s.wg.Done()

	now := s.clock()
	s.runPublishCycle(now)
	s.runReconcileCycle(now)

	publishTicker := time.NewTicker(s.publishInterval)
	reconcileTicker := time.NewTicker(s.reconcileEvery)
	defer publishTicker.Stop()
	defer reconcileTicker.Stop()

	for {
		select {
		case tick := <-publishTicker.C:
			s.runPublishCycle(tick.UTC())
		case tick := <-reconcileTicker.C:
			s.runReconcileCycle(tick.UTC())
		case <-s.stopCh:
			return
		}
	}
}

func (s *RuntimeService) runPublishCycle(now time.Time) {
	for _, guildID := range s.configuredGuildIDs(true) {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeOperationTimeout)
		published, err := s.lifecycleService.PublishScheduledIfDue(ctx, guildID, s.session)
		cancel()
		if err != nil {
			log.ApplicationLogger().Warn(
				"QOTD scheduled publish failed",
				"guildID", guildID,
				"botInstanceID", s.botInstanceID,
				"at", now.UTC(),
				"err", err,
			)
			continue
		}
		if published {
			log.ApplicationLogger().Info(
				"QOTD scheduled publish completed",
				"guildID", guildID,
				"botInstanceID", s.botInstanceID,
				"at", now.UTC(),
			)
		}
	}
}

func (s *RuntimeService) runReconcileCycle(now time.Time) {
	for _, guildID := range s.configuredGuildIDs(false) {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeOperationTimeout)
		err := s.lifecycleService.ReconcileGuild(ctx, guildID, s.session)
		cancel()
		if err != nil {
			log.ApplicationLogger().Warn(
				"QOTD reconcile failed",
				"guildID", guildID,
				"botInstanceID", s.botInstanceID,
				"at", now.UTC(),
				"err", err,
			)
		}
	}
}

func (s *RuntimeService) configuredGuildIDs(requireEnabled bool) []string {
	if s == nil || s.configManager == nil || s.lifecycleService == nil || s.session == nil {
		return nil
	}
	cfg := s.configManager.Config()
	if cfg == nil {
		return nil
	}

	guilds := cfg.GuildsForBotInstanceForDomain(files.BotDomainQOTD, s.botInstanceID, s.defaultBotID)
	if len(guilds) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(guilds))
	ids := make([]string, 0, len(guilds))
	for _, guild := range guilds {
		guildID := strings.TrimSpace(guild.GuildID)
		if guildID == "" {
			continue
		}
		if requireEnabled {
			deck, ok := guild.QOTD.ActiveDeck()
			if !ok || !deck.Enabled {
				continue
			}
		} else if guild.QOTD.IsZero() {
			continue
		}
		if _, ok := seen[guildID]; ok {
			continue
		}
		seen[guildID] = struct{}{}
		ids = append(ids, guildID)
	}
	return ids
}

func (s *RuntimeService) clock() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
