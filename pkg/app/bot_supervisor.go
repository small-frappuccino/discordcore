package app

import (
	"context"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"golang.org/x/sync/semaphore"
)

type BotSupervisor struct {
	configManager   *files.ConfigManager
	resolver        *botRuntimeResolver
	serviceManager  *service.Manager
	opts            botRuntimeOptions

	// identifySemaphore limits concurrent Discord Identify calls
	identifySemaphore *semaphore.Weighted

	mu           sync.Mutex
	knownTokens  map[string]string // botInstanceID -> token
}

func NewBotSupervisor(configManager *files.ConfigManager, opts botRuntimeOptions) *BotSupervisor {
	supervisor := &BotSupervisor{
		configManager:     configManager,
		resolver:          newBotRuntimeResolver(configManager, make(map[string]*botRuntime), opts.defaultBotInstanceID),
		serviceManager:    service.NewManager(),
		opts:              opts,
		identifySemaphore: semaphore.NewWeighted(1), // Rate limit Identify
		knownTokens:       make(map[string]string),
	}

	configManager.AddWatcher(supervisor.onConfigChanged)

	return supervisor
}

func (s *BotSupervisor) Start() error {
	s.onConfigChanged(nil) // trigger initial resolution
	return nil
}

func (s *BotSupervisor) GetResolver() *botRuntimeResolver {
	return s.resolver
}

func (s *BotSupervisor) onConfigChanged(cfg *files.BotConfig) {
	if cfg == nil {
		snap := s.configManager.SnapshotConfig()
		cfg = &snap
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Gather all tokens from all guilds
	currentTokens := make(map[string]string)
	for _, guild := range cfg.Guilds {
		for instanceID, encryptedToken := range guild.BotInstanceTokens {
			token := string(encryptedToken)
			if token == "" {
				continue
			}
			currentTokens[instanceID] = token
		}
	}

	// 2. Compute differences
	toStart := make(map[string]string)
	toStop := make([]string, 0)

	for id, token := range currentTokens {
		oldToken, exists := s.knownTokens[id]
		if !exists || oldToken != token {
			toStart[id] = token
		}
	}

	for id := range s.knownTokens {
		if _, exists := currentTokens[id]; !exists {
			toStop = append(toStop, id)
		}
	}

	// 3. Stop removed/changed bots
	for _, id := range toStop {
		log.ApplicationLogger().Info("Stopping bot instance due to token removal", "botInstanceID", id)
		s.stopBotInstanceLocked(id)
	}

	for id := range toStart {
		if _, exists := s.knownTokens[id]; exists {
			log.ApplicationLogger().Info("Stopping bot instance due to token update", "botInstanceID", id)
			s.stopBotInstanceLocked(id)
		}
	}

	// 4. Start new/updated bots
	for id, token := range toStart {
		s.knownTokens[id] = token
		go s.startBotInstanceBackground(id, token)
	}
}

func (s *BotSupervisor) stopBotInstanceLocked(instanceID string) {
	delete(s.knownTokens, instanceID)
	
	// Remove from resolver so new events don't route here
	currentRuntimes := s.resolver.getRuntimes()
	newRuntimes := make(map[string]*botRuntime)
	for k, v := range currentRuntimes {
		if k != instanceID {
			newRuntimes[k] = v
		}
	}
	s.resolver.swapRuntimes(newRuntimes)

	// Issue graceful stop
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.serviceManager.StopAndRemove(ctx, "bot-runtime-"+instanceID); err != nil {
		log.ApplicationLogger().Warn("Failed to cleanly stop bot instance", "botInstanceID", instanceID, "error", err)
	}
}

func (s *BotSupervisor) startBotInstanceBackground(instanceID, token string) {
	log.ApplicationLogger().Info("Acquiring Discord Identify semaphore", "botInstanceID", instanceID)
	
	if err := s.identifySemaphore.Acquire(context.Background(), 1); err != nil {
		log.ApplicationLogger().Error("identify semaphore error", "botInstanceID", instanceID, "error", err)
		return
	}
	defer s.identifySemaphore.Release(1)

	// Small delay to ensure we don't spam identify even after acquiring semaphore
	time.Sleep(5 * time.Second)

	capabilities := resolveBotRuntimeCapabilities(s.configManager.Config(), instanceID, s.opts.defaultBotInstanceID)

	runtime, err := openBotRuntime(resolvedBotInstance{ID: instanceID, Token: token}, capabilities)
	if err != nil {
		log.ApplicationLogger().Error("failed to open bot runtime", "botInstanceID", instanceID, "error", err)
		s.mu.Lock()
		delete(s.knownTokens, instanceID)
		s.mu.Unlock()
		return
	}

	if err := initializeBotRuntime(runtime, s.opts); err != nil {
		log.ApplicationLogger().Error("failed to initialize bot runtime", "botInstanceID", instanceID, "error", err)
		s.mu.Lock()
		delete(s.knownTokens, instanceID)
		s.mu.Unlock()
		return
	}

	// Register in dynamic service manager to drain connections safely later
	wrapper := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     "bot-runtime-" + instanceID,
		Type:     service.TypeMonitoring,
		Priority: service.PriorityNormal,
		Start: func(ctx context.Context) error {
			if err := runtime.session.Open(); err != nil {
				return err
			}
			<-ctx.Done()
			return nil
		},
		Stop: func(context.Context) error {
			shutdownBotRuntime(runtime, context.Background())
			return runtime.session.Close()
		},
	})

	if err := s.serviceManager.RegisterAndStart("bot-runtime-"+instanceID, wrapper); err != nil {
		log.ApplicationLogger().Error("failed to register bot runtime service", "botInstanceID", instanceID, "error", err)
		return
	}

	// Make visible in resolver
	s.mu.Lock()
	defer s.mu.Unlock()

	// Only add if it wasn't removed while we were initializing
	if _, exists := s.knownTokens[instanceID]; exists {
		currentRuntimes := s.resolver.getRuntimes()
		newRuntimes := make(map[string]*botRuntime)
		for k, v := range currentRuntimes {
			newRuntimes[k] = v
		}
		newRuntimes[instanceID] = runtime
		s.resolver.swapRuntimes(newRuntimes)
		log.ApplicationLogger().Info("Bot instance dynamically started and registered", "botInstanceID", instanceID)
	}
}
