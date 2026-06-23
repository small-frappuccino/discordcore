package app

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"golang.org/x/sync/errgroup"
)

// InstanceStatus represents the lifecycle state of a managed bot instance.
type InstanceStatus string

const (
	// StatusStarting indicates the bot instance is initializing.
	StatusStarting InstanceStatus = "starting"
	// StatusRunning indicates the bot instance is active and connected.
	StatusRunning InstanceStatus = "running"
	// StatusStopping indicates the bot instance is in the process of shutting down.
	StatusStopping InstanceStatus = "stopping"
	// StatusError indicates the bot instance encountered an irreversible failure.
	StatusError InstanceStatus = "error"
)

type botInstanceState struct {
	Token         string
	DiscordStatus string
	Status        InstanceStatus
}

// BotSupervisor manages the lifecycle, configuration synchronization, and background state of all active Discord bot instances.
type BotSupervisor struct {
	configManager  *files.ConfigManager
	resolver       *botRuntimeResolver
	serviceManager *service.Manager
	opts           botRuntimeOptions

	ctx    context.Context
	cancel context.CancelFunc
	bgWG   sync.WaitGroup
	logger *slog.Logger

	mu           sync.Mutex
	instances    map[string]*botInstanceState // botInstanceID -> state
	nextIdentify time.Time

	fatalCallback        func(error)
	identifyStaggerDelay time.Duration
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
	supervisor := &BotSupervisor{
		configManager:        configManager,
		resolver:             newBotRuntimeResolver(configManager, make(map[string]*botRuntime)),
		serviceManager:       service.NewManager(),
		opts:                 opts,
		logger:               opts.logger,
		ctx:                  ctx,
		cancel:               cancel,
		instances:            make(map[string]*botInstanceState),
		identifyStaggerDelay: 5 * time.Second,
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fatalCallback = cb
}

// Start triggers the initial configuration resolution and boots up required bot instances.
func (s *BotSupervisor) Start() error {
	s.log().Info("Initializing primary routines of BotSupervisor", slog.String("component", "BotSupervisor"))
	s.onConfigChanged(context.Background(), nil, nil) // trigger initial resolution
	return nil
}

// Stop initiates a graceful shutdown of all managed bot instances and waits for background processes to terminate.
func (s *BotSupervisor) Stop(ctx context.Context) error {
	s.log().Info("Triggering planned shutdown of main BotSupervisor instances")
	s.cancel() // signal background goroutines to abort

	eg, egCtx := errgroup.WithContext(ctx)

	s.mu.Lock()
	for id, state := range s.instances {
		if state.Status != StatusStopping {
			state.Status = StatusStopping
			s.bgWG.Add(1)

			instanceID := id
			instanceState := state
			eg.Go(func() error {
				defer s.bgWG.Done()
				return s.executeStopAndRemove(egCtx, instanceID, instanceState)
			})
		}
	}
	s.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		err := eg.Wait()
		s.bgWG.Wait() // Secondary barrier: ensures bg processes like retries clean up memory
		done <- err
	}()

	select {
	case err := <-done:
		return err
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

func (s *BotSupervisor) onConfigChanged(ctx context.Context, oldCfg, newCfg *files.BotConfig) error {
	if newCfg == nil {
		snap := s.configManager.SnapshotConfig()
		newCfg = &snap
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// 1. COMPUTE PHASE (Locked)
	s.mu.Lock()
	currentTokens := make(map[string]string)
	currentStatuses := make(map[string]string)

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
		}
	}

	type statusUpdate struct {
		instanceID string
		status     string
		state      *state.State
	}
	var gatewayUpdates []statusUpdate

	toStart := make(map[string]string)
	toStop := make([]string, 0, len(s.instances))

	for id, token := range currentTokens {
		oldState, exists := s.instances[id]
		var capsChanged bool
		if exists && oldCfg != nil {
			oldCaps := resolveBotRuntimeCapabilities(oldCfg, id)
			newCaps := resolveBotRuntimeCapabilities(newCfg, id)
			if oldCaps != newCaps {
				capsChanged = true
			}
		}

		if !exists || oldState.Token != token || capsChanged {
			toStart[id] = token
		} else if oldState.DiscordStatus != currentStatuses[id] {
			oldState.DiscordStatus = currentStatuses[id]
			if runtime, ok := s.resolver.getRuntimes()[id]; ok && runtime.arikawaState != nil {
				gatewayUpdates = append(gatewayUpdates, statusUpdate{
					instanceID: id,
					status:     currentStatuses[id],
					state:      runtime.arikawaState,
				})
			}
		}
	}

	for id := range s.instances {
		if _, exists := currentTokens[id]; !exists {
			toStop = append(toStop, id)
		}
	}
	s.mu.Unlock() // LOCK RELEASED IMMEDIATELY

	// 2. ACT PHASE (Unlocked & Concurrent)

	// Dispatch Gateway presence updates explicitly via errgroup
	// O for-loop não itera sobre arrays de len==0 naturalmente. Sem "if" aninhado.
	for _, update := range gatewayUpdates {
		u := update
		s.opts.startupTasks.GoHeavy("presence_"+u.instanceID, func(taskCtx context.Context) error {
			updateCtx, cancel := context.WithTimeout(taskCtx, 5*time.Second)
			defer cancel()
			err := u.state.Gateway().Send(updateCtx, &gateway.UpdatePresenceCommand{
				Status: discord.Status(u.status),
			})
			if err != nil {
				s.log().Warn("Failed to update discord status for instance",
					slog.String("botInstanceID", u.instanceID),
					slog.String("mitigation", "operation ignored to protect main flow"),
					slog.Any("error", err),
				)
			}
			return nil
		})
	}

	// Phase 3: Initiate shutdown sequence for instances whose credentials have been revoked or removed.
	for _, id := range toStop {
		s.mu.Lock()
		state, exists := s.instances[id]
		if exists && state.Status != StatusStopping {
			s.log().Info("Planned instance shutdown triggered by token removal", slog.String("botInstanceID", id))
			state.Status = StatusStopping
			s.bgWG.Add(1)
			s.mu.Unlock()

			idCopy := id
			stateCopy := state
			s.opts.startupTasks.GoHeavy("stop_"+idCopy, func(ctx context.Context) error {
				defer s.bgWG.Done()
				if ctx.Err() != nil {
					return nil
				}
				return s.executeStopAndRemove(ctx, idCopy, stateCopy)
			})
		} else {
			s.mu.Unlock()
		}
	}

	// Phase 4: Execute startup pipeline for new or updated instances, blocking on prior shutdown completion.
	var startWG sync.WaitGroup
	for id, token := range toStart {
		var oldState *botInstanceState
		var scheduleStop bool

		s.mu.Lock()
		if state, exists := s.instances[id]; exists {
			if state.Status != StatusStopping {
				if state.Token != token {
					s.log().Info("Planned instance shutdown triggered by token update", slog.String("botInstanceID", id))
				} else {
					s.log().Info("Planned instance shutdown triggered by capability change", slog.String("botInstanceID", id))
				}
				state.Status = StatusStopping
				s.bgWG.Add(1)
				scheduleStop = true
			}
			oldState = state
		}
		s.mu.Unlock()

		if scheduleStop {
			idCopy := id
			stateCopy := oldState
			s.opts.startupTasks.GoHeavy("stop_"+idCopy, func(ctx context.Context) error {
				defer s.bgWG.Done()
				if ctx.Err() != nil {
					return nil
				}
				return s.executeStopAndRemove(ctx, idCopy, stateCopy)
			})
		}

		s.bgWG.Add(1)
		startWG.Add(1)
		idCopy := id
		tokenCopy := token
		statusCopy := currentStatuses[id]

		s.opts.startupTasks.GoHeavy("start_"+idCopy, func(ctx context.Context) error {
			defer s.bgWG.Done()
			defer startWG.Done()
			if ctx.Err() != nil {
				return nil
			}
			s.awaitStopAndStart(idCopy, tokenCopy, statusCopy, oldState)
			return nil
		})
	}

	// Phase 5: Enqueue asynchronous command catalog synchronization to reconcile Discord API state with local feature flags.
	type syncTask struct {
		guildID   string
		instances []string
	}
	var syncTasks []syncTask

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
					syncTasks = append(syncTasks, syncTask{
						guildID:   newGuild.GuildID,
						instances: activeInstances,
					})
				}
			}
		}
	}

	s.bgWG.Add(1)
	s.opts.startupTasks.GoLight("catalog_sync_waiter", func(ctx context.Context) error {
		defer s.bgWG.Done()

		waitChan := make(chan struct{})
		go func() {
			startWG.Wait()
			close(waitChan)
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-waitChan:
		}

		s.resolver.markReady()

		if len(syncTasks) == 0 {
			return nil
		}

		s.bgWG.Add(1)
		s.opts.startupTasks.GoHeavy("catalog_sync", func(heavyCtx context.Context) error {
			defer s.bgWG.Done()

			eg, egCtx := errgroup.WithContext(heavyCtx)
			// Bounded limit to prevent OOM
			eg.SetLimit(10)

			for _, task := range syncTasks {
				t := task
				eg.Go(func() error {
					// Small debounce jitter
					select {
					case <-egCtx.Done():
						return egCtx.Err()
					case <-time.After(time.Duration(rand.Float64()*500) * time.Millisecond):
					}

					currentRuntimes := s.resolver.getRuntimes()
					for _, instanceID := range t.instances {
						if egCtx.Err() != nil {
							return egCtx.Err()
						}
						runtime, ok := currentRuntimes[instanceID]
						if !ok || runtime == nil || runtime.commandHandler == nil {
							continue
						}
						if syncer := runtime.commandHandler.GetSyncer(); syncer != nil {
							appIDInt, _ := strconv.ParseInt(t.guildID, 10, 64)
							if syncErr := syncer.SyncBulkOverwrite(discord.GuildID(appIDInt), runtime.commandHandler.GetRouter().Registry()); syncErr != nil {
								if strings.Contains(syncErr.Error(), "403") {
									s.log().Warn("Dynamic command synchronization ignored due to authorization barrier",
										slog.String("guildID", t.guildID),
										slog.String("botInstanceID", instanceID),
										slog.String("mitigation", "permission bypass"),
										slog.Any("error", syncErr),
									)
								} else {
									s.log().Error("Structural failure synchronizing guild commands",
										slog.String("request_id", "sync_"+t.guildID+"_"+instanceID),
										slog.String("guildID", t.guildID),
										slog.String("botInstanceID", instanceID),
										slog.Any("error", syncErr),
									)
									return fmt.Errorf("sync bulk overwrite for guild %s: %w", t.guildID, syncErr)
								}
							} else {
								s.log().Info("Dynamic guild command synchronization completed", slog.String("guildID", t.guildID), slog.String("botInstanceID", instanceID))
							}
						}
					}
					return nil
				})
			}
			return eg.Wait()
		})
		return nil
	})

	return nil
}

func (s *BotSupervisor) executeStopAndRemove(ctx context.Context, id string, state *botInstanceState) error {
	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	svcName := "bot-runtime-" + id
	err := s.serviceManager.StopAndRemove(stopCtx, svcName)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Conditional deletion: ensures overwritten pointers are not deleted
	if current, exists := s.instances[id]; exists {
		if current == state {
			s.log().Info("executeStopAndRemove DELETING instance", slog.String("id", id))
			delete(s.instances, id)
		} else {
			s.log().Info("executeStopAndRemove SKIPPING deletion: pointer mismatch", slog.String("id", id))
		}
	} else {
		s.log().Info("executeStopAndRemove SKIPPING deletion: not found", slog.String("id", id))
	}

	// Remove from resolver so new events don't route here
	currentRuntimes := s.resolver.getRuntimes()
	newRuntimes := make(map[string]*botRuntime, len(currentRuntimes))
	for k, v := range currentRuntimes {
		newRuntimes[k] = v
	}
	delete(newRuntimes, id)
	s.resolver.swapRuntimes(newRuntimes)

	if err != nil && strings.Contains(err.Error(), "not found") {
		err = nil
	}

	if err != nil {
		s.serviceManager.ForceRemove(svcName)
		state.Status = StatusError
		s.log().Error("Failed to purge I/O, escalated to ForceRemove",
			slog.String("request_id", "stop_remove_"+id),
			slog.String("botInstanceID", id),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

func (s *BotSupervisor) awaitStopAndStart(id, token, status string, oldState *botInstanceState) {
	if oldState != nil {
		// Padrão de Epochs Imutáveis: Eliminamos o bloqueio de espera.
		// Permitimos que a instância antiga seja desligada em segundo plano.
		if oldState.Status == StatusError {
			s.log().Warn("Previous instance lifecycle terminated in degraded state; forcefully recovering execution pipeline",
				slog.String("request_id", "await_start_recovery_"+id),
				slog.String("botInstanceID", id),
			)
		}
	}

	s.mu.Lock()
	newState := &botInstanceState{
		Token:         token,
		DiscordStatus: status,
		Status:        StatusStarting,
	}
	s.instances[id] = newState
	s.mu.Unlock()

	s.startBotInstanceBackground(id, token, status, newState)
}

// isObsolete verifica de forma segura se o ponteiro de estado da goroutine atual
// foi sobrescrito por uma mutação de configuração mais recente.
func (s *BotSupervisor) isObsolete(instanceID string, state *botInstanceState) bool {
	s.mu.Lock()
	isOld := s.instances[instanceID] != state
	s.mu.Unlock()
	return isOld
}

// checkTokenRevocationError validates if an external string strictly matches auth failure invariants.
func checkTokenRevocationError(errStr string) bool {
	lowerErr := strings.ToLower(errStr)
	return strings.Contains(lowerErr, "4004") ||
		strings.Contains(lowerErr, "authentication failed") ||
		(strings.Contains(lowerErr, "401") && !strings.Contains(lowerErr, "4014"))
}

func (s *BotSupervisor) startBotInstanceBackground(instanceID, token, status string, state *botInstanceState) {
	capabilities := resolveBotRuntimeCapabilities(s.configManager.Config(), instanceID)

	var runtime *botRuntime
	var err error

	baseDelay := float64(2 * time.Second)
	maxDelayFloat := float64(30 * time.Second)

	for attempt := 0; attempt < 5; attempt++ {
		if s.isObsolete(instanceID, state) {
			s.log().Debug("Execution pipeline aborted: newer configuration state detected before sleep", slog.String("botInstanceID", instanceID))
			return
		}

		s.mu.Lock()
		now := time.Now()
		var sleepDur time.Duration
		if s.nextIdentify.After(now) {
			sleepDur = s.nextIdentify.Sub(now)
			s.nextIdentify = s.nextIdentify.Add(s.identifyStaggerDelay)
		} else {
			s.nextIdentify = now.Add(s.identifyStaggerDelay)
		}
		s.mu.Unlock()

		if sleepDur > 0 {
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(sleepDur):
			}
		}

		if s.isObsolete(instanceID, state) {
			return
		}

		runtime, err = openBotRuntimeFn(resolvedBotInstance{ID: instanceID, Token: token, DiscordStatus: status}, capabilities, s.opts)
		if err == nil {
			break
		}

		if checkTokenRevocationError(err.Error()) {
			s.log().Warn("Instance authentication compromised, triggering token revocation",
				slog.String("botInstanceID", instanceID),
				slog.Any("error", err),
			)
			_ = s.configManager.RevokeBotInstance(instanceID, token)
			break
		}

		// Achatamento matemático do Backoff
		delay := baseDelay * float64(uint(1)<<attempt)
		if delay > maxDelayFloat {
			delay = maxDelayFloat
		}
		sleepTime := time.Duration(delay + (rand.Float64() * delay * 0.2))

		timer := time.NewTimer(sleepTime)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}

	if err != nil {
		s.mu.Lock()
		if s.instances[instanceID] == state {
			state.Status = StatusError
		}
		s.mu.Unlock()
		return
	}

	// Barreira 3: Aborta antes de inicializar serviços na memória RAM
	if s.isObsolete(instanceID, state) {
		shutdownBotRuntime(runtime, s.ctx)
		return
	}

	if err := initializeBotRuntime(s.ctx, runtime, s.opts); err != nil {
		s.mu.Lock()
		if s.instances[instanceID] == state {
			state.Status = StatusError
		}
		s.mu.Unlock()
		return
	}

	// Nome do serviço DEVE ser estático. Nada de timestamps.
	serviceName := "bot-runtime-" + instanceID

	wrapper := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     serviceName,
		Type:     service.TypeMonitoring,
		Priority: service.PriorityNormal,
		Start: func(ctx context.Context) error {
			if err := runtime.serviceManager.StartAll(); err != nil {
				return fmt.Errorf("start services for %s: %w", runtime.instanceID, err)
			}
			scheduleRuntimeConfiguredGuildLogging(runtime, s.opts.configManager, s.opts.startupTasks)
			scheduleRuntimeWarmup(ctx, runtime, s.opts.store, s.opts.startupTasks)
			return nil
		},
		Stop: func(ctx context.Context) error {
			shutdownBotRuntime(runtime, ctx)
			if runtime.arikawaState != nil {
				var err error
				if s.opts.discordSessionCloseHook != nil {
					err = s.opts.discordSessionCloseHook(runtime.arikawaState)
				} else {
					err = runtime.arikawaState.Close()
				}
				if err != nil && strings.Contains(err.Error(), "Session is closed") {
					return nil
				}
				return err
			}
			return nil
		},
		Logger: slog.Default(),
	})

	// Barreira 4: Registro com resolução graciosa de Micro-Race Conditions
	if err := s.serviceManager.RegisterAndStart(serviceName, wrapper); err != nil {
		if strings.Contains(err.Error(), "already registered") {
			if s.isObsolete(instanceID, state) {
				// A goroutine anterior foi tão rápida que registrou e nos deixou para trás. Morremos silenciosamente.
				shutdownBotRuntime(runtime, s.ctx)
				return
			}
			// Se chegamos aqui, somos a goroutine oficial, mas o StopAndRemove da anterior não limpou a memória a tempo.
			// Expurgamos o detrito e tomamos o lugar à força.
			s.log().Warn("Service manager memory conflict detected; executing forceful override", slog.String("botInstanceID", instanceID))
			s.serviceManager.ForceRemove(serviceName)
			_ = s.serviceManager.RegisterAndStart(serviceName, wrapper)
		} else {
			s.log().Error("Fatal failure coupling interface with Service Manager", slog.Any("error", err))
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.instances[instanceID] == state {
		state.Status = StatusRunning
		currentRuntimes := s.resolver.getRuntimes()
		newRuntimes := make(map[string]*botRuntime)
		for k, v := range currentRuntimes {
			newRuntimes[k] = v
		}
		newRuntimes[instanceID] = runtime
		s.resolver.swapRuntimes(newRuntimes)
	} else {
		// Se uma alteração incrivelmente rápida ocorreu exatamente após o registro, recuamos.
		s.serviceManager.StopAndRemove(context.Background(), serviceName)
	}
}
