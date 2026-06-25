package app

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"golang.org/x/sync/errgroup"
)

// managedInstance maintains the lifecycle isolation boundary of an active goroutine.
type managedInstance struct {
	CancelContext context.CancelFunc
	Token         string
	Status        string
	Capabilities  botRuntimeCapabilities
}

type GatewayUpdateIntent struct {
	InstanceID string
	Status     string
}

type SyncTaskIntent struct {
	GuildID    string
	InstanceID string
}

// TopologyDelta transmits state reconciliation vectors down into the hardware ring.
type TopologyDelta struct {
	ActiveTokens   map[string]string
	ActiveStatus   map[string]string
	Capabilities   map[string]botRuntimeCapabilities
	GatewayUpdates []GatewayUpdateIntent
	SyncTasks      []SyncTaskIntent
}

// BotSupervisor manages the lifecycle, configuration synchronization, and background state of all active Discord bot instances via CSP loop.
type BotSupervisor struct {
	trackedInstances map[string]*managedInstance

	configManager *files.ConfigManager
	resolver      *botRuntimeResolver
	opts          botRuntimeOptions

	ctx      context.Context
	cancel   context.CancelFunc
	group    *errgroup.Group
	groupCtx context.Context
	logger   *slog.Logger

	fatalCallback func(error)

	commandCh   chan TopologyDelta
	telemetryCh chan RuntimeTelemetryEvent
}

// NewBotSupervisor initializes a new BotSupervisor to manage bot runtimes.
func NewBotSupervisor(configManager *files.ConfigManager, opts botRuntimeOptions) *BotSupervisor {
	ctx, cancel := context.WithCancel(context.Background())
	group, groupCtx := errgroup.WithContext(ctx)

	resolver := newBotRuntimeResolver(configManager, make(map[string]*botRuntime))
	if opts.logger == nil {
		opts.logger = slog.Default()
	}

	supervisor := &BotSupervisor{
		trackedInstances: make(map[string]*managedInstance),
		configManager:    configManager,
		resolver:         resolver,
		opts:             opts,
		ctx:              ctx,
		cancel:           cancel,
		group:            group,
		groupCtx:         groupCtx,
		logger:           opts.logger,
		commandCh:        make(chan TopologyDelta, 1),
		telemetryCh:      make(chan RuntimeTelemetryEvent, 64),
	}

	return supervisor
}

func (s *BotSupervisor) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// SetFatalCallback configures a callback to be invoked when a critical background failure occurs.
func (s *BotSupervisor) SetFatalCallback(cb func(error)) {
	s.fatalCallback = cb
}

// Start triggers the initial configuration resolution and boots up required bot instances.
func (s *BotSupervisor) Start() error {
	s.log().Info("Initializing primary routines of BotSupervisor", slog.String("component", "BotSupervisor"))
	s.group.Go(func() error {
		return s.executionRing()
	})
	s.onConfigChanged(context.Background(), nil, nil) // trigger initial resolution
	return nil
}

func (s *BotSupervisor) executionRing() error {
	s.log().Info("Architectural state transition: Hardware execution ring active")
	for {
		select {
		case <-s.groupCtx.Done():
			return s.groupCtx.Err()
		case cmd := <-s.commandCh:
			s.handleTopologyDelta(cmd)
		case event := <-s.telemetryCh:
			s.handleTelemetryEvent(event)
		}
	}
}

func (s *BotSupervisor) handleTelemetryEvent(event RuntimeTelemetryEvent) {
	s.log().Info("Telemetry cycle received", slog.String("botInstanceID", event.InstanceID), slog.String("state", string(event.State)))
	switch event.State {
	case TelemetryStateCriticalFailure:
		if s.fatalCallback != nil {
			s.fatalCallback(event.Error)
		}
	case TelemetryStateShuttingDown:
		s.resolver.removeRuntime(event.InstanceID)
	case TelemetryStateConnected:
		s.log().Info("Bot instance achieved runtime connectivity via CSP cycle", slog.String("botInstanceID", event.InstanceID))
	}
}

// Stop initiates a graceful shutdown of all managed bot instances and waits for background processes to terminate.
func (s *BotSupervisor) Stop(ctx context.Context) error {
	s.log().Info("Triggering planned shutdown of main BotSupervisor instances")
	s.cancel() // signal background goroutines to abort

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.group.Wait()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case <-ctx.Done():
		s.log().Error("BotSupervisor stop timeout exceeded before background task completion",
			slog.String("request_id", "supervisor_shutdown"),
			slog.Any("error", ctx.Err()),
		)
		return ctx.Err()
	}
}

// GetResolver returns the internal runtime resolver responsible for routing requests to active bot instances.
func (s *BotSupervisor) GetResolver() *botRuntimeResolver {
	return s.resolver
}

func (s *BotSupervisor) reconcileTopology(parentCtx context.Context, cmd TopologyDelta) error {
	select {
	case <-parentCtx.Done():
		return parentCtx.Err()
	case s.commandCh <- cmd:
		return nil
	}
}

func (s *BotSupervisor) handleTopologyDelta(cmd TopologyDelta) {
	desiredTokens := cmd.ActiveTokens
	desiredStatus := cmd.ActiveStatus
	desiredCaps := cmd.Capabilities

	for _, intent := range cmd.GatewayUpdates {
		s.opts.startupTasks.Go(SupervisorGatewayUpdateTask{
			Supervisor: s,
			Intent:     intent,
		})
	}

	// Phase 1: Purge removed or modified instances to maintain valid architectural state.
	for id, current := range s.trackedInstances {
		newToken, exists := desiredTokens[id]
		newCaps := desiredCaps[id]
		if !exists || newToken != current.Token || !reflect.DeepEqual(current.Capabilities, newCaps) {
			s.log().Info("Architectural state transition: Actively canceling compromised or obsolete configuration",
				slog.String("botInstanceID", id),
			)
			current.CancelContext()
			delete(s.trackedInstances, id)
			s.resolver.removeRuntime(id)
		}
	}

	// Phase 2: Initialize new execution pipelines.
	isReady := false
	if s.resolver != nil {
		select {
		case <-s.resolver.readyCh:
			isReady = true
		default:
		}
	}
	var pendingCount int

	for id, token := range desiredTokens {
		if _, active := s.trackedInstances[id]; !active {
			instanceCtx, instanceCancel := context.WithCancel(s.groupCtx)
			pendingCount++

			s.trackedInstances[id] = &managedInstance{
				CancelContext: instanceCancel,
				Token:         token,
				Status:        desiredStatus[id],
				Capabilities:  desiredCaps[id],
			}

			// Capture local variables for goroutine
			localID := id
			localToken := token
			localCaps := desiredCaps[id]

			s.group.Go(func() error {
				s.log().Debug("Tracking complex conditional branch: Starting isolated hardware pipeline for bot instance",
					slog.String("botInstanceID", localID),
				)

				runtime, err := NewBotRuntime(instanceCtx, resolvedBotInstance{ID: localID, Token: localToken, DiscordStatus: desiredStatus[localID]}, localCaps, s.opts)
				if err != nil {
					s.log().Error("Structural execution failure during bot startup sequence", slog.Any("error", err))
					return nil // Localize error to avoid collapsing the entire ring immediately if it's transient
				}
				s.resolver.addRuntime(localID, runtime)

				err = runtime.Run(instanceCtx, s.telemetryCh, s.opts)
				if err != nil {
					s.log().Error("Runtime execution exited with failure", slog.String("botInstanceID", localID), slog.Any("error", err))
				}
				return nil
			})
		}
	}

	if !isReady {
		// Assuming we mark ready immediately if we scheduled them,
		// or maybe mark ready when pendingCount handles it.
		// For simplicity, we just mark ready directly in this iteration
		// after spawning all goroutines to maintain non-blocking behavior.
		go s.resolver.markReady()
	}

	// Phase 3: Synchronize command catalogs.
	if len(cmd.SyncTasks) > 0 {
		s.opts.startupTasks.Go(SupervisorCatalogSyncTask{
			Supervisor: s,
			SyncTasks:  cmd.SyncTasks,
		})
	}
}

type SupervisorGatewayUpdateTask struct {
	Supervisor *BotSupervisor
	Intent     GatewayUpdateIntent
}

func (t SupervisorGatewayUpdateTask) Execute(ctx context.Context) error {
	return t.Supervisor.executeGatewayUpdate(ctx, t.Intent)
}

func (t SupervisorGatewayUpdateTask) Name() string {
	return "presence_update_" + t.Intent.InstanceID
}

type SupervisorCatalogSyncTask struct {
	Supervisor *BotSupervisor
	SyncTasks  []SyncTaskIntent
}

func (t SupervisorCatalogSyncTask) Execute(ctx context.Context) error {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(10)
	for _, intent := range t.SyncTasks {
		localIntent := intent
		eg.Go(func() error {
			return t.Supervisor.executeSyncTask(egCtx, localIntent)
		})
	}
	return eg.Wait()
}

func (t SupervisorCatalogSyncTask) Name() string {
	return "catalog_sync"
}

func (s *BotSupervisor) onConfigChanged(ctx context.Context, oldCfg, newCfg *files.BotConfig) error {
	if newCfg == nil {
		snap := s.configManager.SnapshotConfig()
		newCfg = &snap
	}

	currentTokens := make(map[string]string)
	currentStatuses := make(map[string]string)
	currentCaps := make(map[string]botRuntimeCapabilities)

	for _, guild := range newCfg.Guilds {
		for instanceID, encryptedToken := range guild.BotInstanceTokens {
			token := string(encryptedToken)
			if token == "" {
				continue
			}
			status := guild.BotInstanceStatuses[instanceID]
			if status == "disabled" {
				continue
			}
			currentTokens[instanceID] = token
			if status == "" {
				status = "online"
			}
			currentStatuses[instanceID] = status
			currentCaps[instanceID] = resolveBotRuntimeCapabilities(newCfg, instanceID)
		}
	}

	var gatewayUpdates []GatewayUpdateIntent

	if oldCfg != nil {
		for id, token := range currentTokens {
			oldToken := ""
			for _, g := range oldCfg.Guilds {
				if t, ok := g.BotInstanceTokens[id]; ok && string(t) != "" {
					oldToken = string(t)
				}
			}
			if oldToken == token {
				oldStatus := ""
				for _, g := range oldCfg.Guilds {
					if st, ok := g.BotInstanceStatuses[id]; ok {
						oldStatus = st
					}
				}
				if oldStatus == "" {
					oldStatus = "online"
				}
				if oldStatus != currentStatuses[id] {
					var rt *botRuntime
					for rtID, runtime := range s.resolver.getRuntimes() {
						if rtID == id {
							rt = runtime
							break
						}
					}
					if rt != nil && rt.arikawaState != nil {
						st := currentStatuses[id]
						gatewayUpdates = append(gatewayUpdates, GatewayUpdateIntent{InstanceID: id, Status: st})
					}
				}
			}
		}
	}

	var syncTasks []SyncTaskIntent
	if oldCfg != nil {
		s.log().Debug("Evaluating conditional feature routing routines")
		for _, newGuild := range newCfg.Guilds {
			var oldGuild *files.GuildConfig
			for i := range oldCfg.Guilds {
				if oldCfg.Guilds[i].GuildID == newGuild.GuildID {
					oldGuild = &oldCfg.Guilds[i]
					break
				}
			}

			needsSync := false
			if oldGuild == nil {
				needsSync = true
			} else if !reflect.DeepEqual(oldGuild.FeatureRouting, newGuild.FeatureRouting) ||
				!reflect.DeepEqual(oldGuild.Features, newGuild.Features) ||
				!reflect.DeepEqual(oldGuild.BotInstanceTokens, newGuild.BotInstanceTokens) ||
				!reflect.DeepEqual(oldGuild.BotInstanceStatuses, newGuild.BotInstanceStatuses) {
				needsSync = true
			}

			if needsSync {
				var activeInstances []string
				for instanceID, token := range newGuild.BotInstanceTokens {
					if string(token) != "" {
						activeInstances = append(activeInstances, instanceID)
					}
				}
				if len(activeInstances) > 0 {
					for _, instanceID := range activeInstances {
						syncTasks = append(syncTasks, SyncTaskIntent{GuildID: newGuild.GuildID, InstanceID: instanceID})
					}
				}
			}
		}
	}

	return s.reconcileTopology(ctx, TopologyDelta{
		ActiveTokens:   currentTokens,
		ActiveStatus:   currentStatuses,
		Capabilities:   currentCaps,
		GatewayUpdates: gatewayUpdates,
		SyncTasks:      syncTasks,
	})
}

// checkTokenRevocationError validates if an external string strictly matches auth failure invariants.
func checkTokenRevocationError(errStr string) bool {
	lowerErr := strings.ToLower(errStr)
	return strings.Contains(lowerErr, "4004") ||
		strings.Contains(lowerErr, "authentication failed") ||
		(strings.Contains(lowerErr, "401") && !strings.Contains(lowerErr, "4014"))
}

func (s *BotSupervisor) executeGatewayUpdate(ctx context.Context, intent GatewayUpdateIntent) error {
	var rt *botRuntime
	for rtID, runtime := range s.resolver.getRuntimes() {
		if rtID == intent.InstanceID {
			rt = runtime
			break
		}
	}
	if rt == nil || rt.arikawaState == nil {
		return nil
	}

	updateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := rt.arikawaState.Gateway().Send(updateCtx, &gateway.UpdatePresenceCommand{
		Status: discord.Status(intent.Status),
	})
	if err != nil {
		s.log().Warn("Failed to update discord status for instance",
			slog.String("botInstanceID", intent.InstanceID),
			slog.String("mitigation", "operation ignored to protect main flow"),
			slog.Any("error", err),
		)
	}
	return nil
}

func (s *BotSupervisor) executeSyncTask(ctx context.Context, intent SyncTaskIntent) error {
	// Syncing is now handled strictly via O(1) hashing inside CommandRegistrar during SetupCommands.
	return nil
}
