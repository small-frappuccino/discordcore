package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"golang.org/x/sync/errgroup"
)

// managedInstance retém a fronteira de isolamento de ciclo de vida de uma goroutine ativa.
type managedInstance struct {
	CancelContext context.CancelFunc
	Token         string
	Status        string
	Capabilities  botRuntimeCapabilities
}

// TopologyDelta transmits state reconciliation vectors down into the hardware ring.
type TopologyDelta struct {
	ActiveTokens   map[string]string
	ActiveStatus   map[string]string
	Capabilities   map[string]botRuntimeCapabilities
	GatewayUpdates []func(context.Context) error
	SyncTasks      []func(context.Context) error
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
	if opts.openBotArikawaState == nil {
		opts.openBotArikawaState = func(ctx context.Context, s *state.State) error { return s.Open(ctx) }
	}
	if opts.fetchBotArikawaMe == nil {
		opts.fetchBotArikawaMe = func(s *state.State) (*discord.User, error) { return s.Me() }
	}
	if opts.newCommandHandlerForBot == nil {
		opts.newCommandHandlerForBot = NewCommandHandlerForBot
	}
	if opts.newCommandHandler == nil {
		opts.newCommandHandler = NewCommandHandler
	}
	if opts.setupCommandHandler == nil {
		opts.setupCommandHandler = func(ch *CommandHandler) error { return ch.SetupCommands() }
	}
	if opts.shutdownCommandHandler == nil {
		opts.shutdownCommandHandler = func(ch *CommandHandler) error { return ch.Shutdown() }
	}
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

	for _, taskFn := range cmd.GatewayUpdates {
		s.opts.startupTasks.Go("presence_update", taskFn)
	}

	// Fase 1: Expurgar instâncias removidas ou alteradas
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

	// Fase 2: Inicializar novas vias de execução
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

				runtime, err := NewBotRuntime(resolvedBotInstance{ID: localID, Token: localToken, DiscordStatus: desiredStatus[localID]}, localCaps, s.opts)
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

	// Fase 3: Sync Commands
	if len(cmd.SyncTasks) > 0 {
		s.opts.startupTasks.Go("catalog_sync", func(ctx context.Context) error {
			eg, egCtx := errgroup.WithContext(ctx)
			eg.SetLimit(10)
			for _, taskFn := range cmd.SyncTasks {
				tFn := taskFn
				eg.Go(func() error {
					return tFn(egCtx)
				})
			}
			return eg.Wait()
		})
	}
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

	var gatewayUpdates []func(context.Context) error

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
						gwState := rt.arikawaState
						instanceID := id

						gatewayUpdates = append(gatewayUpdates, func(ctx context.Context) error {
							updateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
							defer cancel()
							err := gwState.Gateway().Send(updateCtx, &gateway.UpdatePresenceCommand{
								Status: discord.Status(st),
							})
							if err != nil {
								s.log().Warn("Failed to update discord status for instance",
									slog.String("botInstanceID", instanceID),
									slog.String("mitigation", "operation ignored to protect main flow"),
									slog.Any("error", err),
								)
							}
							return nil
						})
					}
				}
			}
		}
	}

	var syncTasks []func(context.Context) error
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
					syncTasks = append(syncTasks, func(ctx context.Context) error {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(time.Duration(rand.Float64()*500) * time.Millisecond):
						}

						for _, instanceID := range activeInstances {
							if ctx.Err() != nil {
								return ctx.Err()
							}
							var runtime *botRuntime
							for id, rt := range s.resolver.getRuntimes() {
								if id == instanceID {
									runtime = rt
									break
								}
							}
							if runtime == nil || runtime.commandHandler == nil {
								continue
							}
							if syncer := runtime.commandHandler.GetSyncer(); syncer != nil {
								appIDInt, _ := strconv.ParseInt(newGuild.GuildID, 10, 64)
								if syncErr := syncer.SyncBulkOverwrite(discord.GuildID(appIDInt), runtime.commandHandler.GetRouter().Registry()); syncErr != nil {
									if strings.Contains(syncErr.Error(), "403") {
										s.log().Warn("Dynamic command synchronization ignored due to authorization barrier",
											slog.String("guildID", newGuild.GuildID),
											slog.String("botInstanceID", instanceID),
											slog.String("mitigation", "permission bypass"),
											slog.Any("error", syncErr),
										)
									} else {
										s.log().Error("Structural failure synchronizing guild commands",
											slog.String("request_id", "sync_"+newGuild.GuildID+"_"+instanceID),
											slog.String("guildID", newGuild.GuildID),
											slog.String("botInstanceID", instanceID),
											slog.Any("error", syncErr),
										)
										return fmt.Errorf("sync bulk overwrite for guild %s: %w", newGuild.GuildID, syncErr)
									}
								} else {
									s.log().Info("Dynamic guild command synchronization completed", slog.String("guildID", newGuild.GuildID), slog.String("botInstanceID", instanceID))
								}
							}
						}
						return nil
					})
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
