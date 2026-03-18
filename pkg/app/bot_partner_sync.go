package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/partners"
)

type botPartnerSyncDispatcher struct {
	configManager        *files.ConfigManager
	syncService          *partners.BoardSyncService
	runtimes             map[string]*botRuntime
	defaultBotInstanceID string
	mu                   sync.Mutex
	running              bool
	coordinators         map[string]*partners.AutoSyncCoordinator
}

func newBotPartnerSyncDispatcher(
	configManager *files.ConfigManager,
	syncService *partners.BoardSyncService,
	runtimes map[string]*botRuntime,
	defaultBotInstanceID string,
) *botPartnerSyncDispatcher {
	return &botPartnerSyncDispatcher{
		configManager:        configManager,
		syncService:          syncService,
		runtimes:             runtimes,
		defaultBotInstanceID: strings.TrimSpace(defaultBotInstanceID),
		coordinators:         make(map[string]*partners.AutoSyncCoordinator, len(runtimes)),
	}
}

func (d *botPartnerSyncDispatcher) Start() error {
	if d == nil {
		return fmt.Errorf("partner sync dispatcher is nil")
	}
	if d.syncService == nil {
		return fmt.Errorf("partner sync dispatcher: sync service is nil")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return nil
	}
	d.running = true
	return nil
}

func (d *botPartnerSyncDispatcher) Stop(ctx context.Context) error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	coordinators := d.coordinators
	d.coordinators = make(map[string]*partners.AutoSyncCoordinator, len(d.runtimes))
	d.running = false
	d.mu.Unlock()

	var stopErrs []error
	for botInstanceID, coordinator := range coordinators {
		if coordinator == nil {
			continue
		}
		if err := coordinator.Stop(ctx); err != nil {
			stopErrs = append(stopErrs, fmt.Errorf("stop partner auto-sync coordinator for %s: %w", botInstanceID, err))
		}
	}
	if len(stopErrs) > 0 {
		return fmt.Errorf("stop partner sync dispatcher: %w", errors.Join(stopErrs...))
	}
	return nil
}

func (d *botPartnerSyncDispatcher) Notify(guildID string) error {
	if d == nil {
		return fmt.Errorf("partner sync dispatcher is nil")
	}
	coordinator, botInstanceID, err := d.ensureCoordinatorForGuild(guildID)
	if err != nil {
		return err
	}
	if coordinator == nil {
		return fmt.Errorf("partner sync dispatcher: coordinator for %s is unavailable", botInstanceID)
	}
	return coordinator.Notify(guildID)
}

func (d *botPartnerSyncDispatcher) SyncGuild(ctx context.Context, guildID string) error {
	if d == nil {
		return fmt.Errorf("partner sync dispatcher is nil")
	}
	runtime, botInstanceID, err := d.runtimeForGuild(guildID)
	if err != nil {
		return err
	}
	if runtime == nil || runtime.session == nil {
		return fmt.Errorf("partner sync dispatcher: bot runtime %q is unavailable for guild %s", botInstanceID, guildID)
	}
	return d.syncService.SyncGuild(ctx, runtime.session, guildID)
}

func (d *botPartnerSyncDispatcher) ensureCoordinatorForGuild(guildID string) (*partners.AutoSyncCoordinator, string, error) {
	runtime, botInstanceID, err := d.runtimeForGuild(guildID)
	if err != nil {
		return nil, "", err
	}
	if runtime == nil {
		return nil, "", fmt.Errorf("partner sync dispatcher: no runtime for guild %s", guildID)
	}
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return nil, "", fmt.Errorf("partner sync dispatcher: dispatcher is not running")
	}
	if coordinator := d.coordinators[botInstanceID]; coordinator != nil {
		d.mu.Unlock()
		return coordinator, botInstanceID, nil
	}
	d.mu.Unlock()

	if runtime.session == nil {
		return nil, "", fmt.Errorf("partner sync dispatcher: bot runtime %q has no discord session", botInstanceID)
	}
	coordinator := partners.NewAutoSyncCoordinator(
		partners.NewSessionBoundBoardSyncExecutor(d.syncService, runtime.session),
		partners.AutoSyncCoordinatorOptions{},
	)
	if err := coordinator.Start(); err != nil {
		return nil, "", fmt.Errorf("start partner auto-sync coordinator for %s: %w", botInstanceID, err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = coordinator.Stop(stopCtx)
		return nil, "", fmt.Errorf("partner sync dispatcher: dispatcher is not running")
	}
	if existing := d.coordinators[botInstanceID]; existing != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = coordinator.Stop(stopCtx)
		return existing, botInstanceID, nil
	}
	d.coordinators[botInstanceID] = coordinator
	return coordinator, botInstanceID, nil
}

func (d *botPartnerSyncDispatcher) runtimeForGuild(guildID string) (*botRuntime, string, error) {
	if d == nil || d.configManager == nil {
		return nil, "", fmt.Errorf("partner sync dispatcher: config manager is unavailable")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, "", fmt.Errorf("partner sync dispatcher: guild_id is required")
	}
	guild := d.configManager.GuildConfig(guildID)
	if guild == nil {
		return nil, "", fmt.Errorf("partner sync dispatcher: guild %s is not configured", guildID)
	}
	botInstanceID := guild.EffectiveBotInstanceID(d.defaultBotInstanceID)
	runtime := d.runtimes[botInstanceID]
	if runtime == nil {
		return nil, botInstanceID, fmt.Errorf("partner sync dispatcher: bot runtime %q is unavailable", botInstanceID)
	}
	return runtime, botInstanceID, nil
}
