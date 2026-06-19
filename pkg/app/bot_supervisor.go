package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordgo"
	"golang.org/x/sync/errgroup"
)

var identifyStaggerDelay = 5 * time.Second

type InstanceStatus string

const (
	StatusStarting InstanceStatus = "starting"
	StatusRunning  InstanceStatus = "running"
	StatusStopping InstanceStatus = "stopping"
	StatusError    InstanceStatus = "error"
)

type botInstanceState struct {
	Token         string
	DiscordStatus string
	Status        InstanceStatus
	StopWG        *sync.WaitGroup
}

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

	fatalCallback func(error)
}

func NewBotSupervisor(configManager *files.ConfigManager, opts botRuntimeOptions) *BotSupervisor {
	ctx, cancel := context.WithCancel(context.Background())
	supervisor := &BotSupervisor{
		configManager:  configManager,
		resolver:       newBotRuntimeResolver(configManager, make(map[string]*botRuntime)),
		serviceManager: service.NewManager(),
		opts:           opts,
		logger:         opts.logger,
		ctx:            ctx,
		cancel:         cancel,
		instances:      make(map[string]*botInstanceState),
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

func (s *BotSupervisor) Start() error {
	s.log().Info("Initializing primary routines of BotSupervisor", slog.String("component", "BotSupervisor"))
	_ = s.onConfigChanged(context.Background(), nil, nil) // trigger initial resolution
	return nil
}

func (s *BotSupervisor) Stop(ctx context.Context) error {
	s.log().Info("Triggering planned shutdown of main BotSupervisor instances")
	s.cancel() // signal background goroutines to abort

	var globalWG sync.WaitGroup
	errsCh := make(chan error, len(s.instances))

	s.mu.Lock()
	for id, state := range s.instances {
		if state.Status != StatusStopping {
			state.Status = StatusStopping
			state.StopWG.Add(1)
			globalWG.Add(1)
			s.bgWG.Add(1)
			go func(id string, state *botInstanceState) {
				defer s.bgWG.Done()
				if err := s.executeStopAndRemove(ctx, id, state, &globalWG); err != nil {
					errsCh <- err
				}
			}(id, state)
		}
	}
	s.mu.Unlock()

	// Mandatory barrier: prevents process shutdown while I/O is pending
	done := make(chan struct{})
	go func() {
		globalWG.Wait()
		s.bgWG.Wait() // Secondary barrier: ensures bg processes like retries clean up memory
		close(errsCh)
		close(done)
	}()

	var stopErrors []error

	select {
	case <-done:
		for err := range errsCh {
			stopErrors = append(stopErrors, err)
		}
	case <-ctx.Done():
		s.log().Error("BotSupervisor stop timeout exceeded before background task completion",
			slog.String("request_id", "supervisor_shutdown"),
			slog.Int("synthetic_status_code", 500),
			slog.String("stacktrace", string(debug.Stack())),
			slog.Any("error", ctx.Err()),
		)
		stopErrors = append(stopErrors, ctx.Err())
	}

	if len(stopErrors) > 0 {
		return errors.Join(stopErrors...)
	}
	return nil
}

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

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Gather all tokens from all guilds
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

	fallbackToken := os.Getenv("BOT_TOKEN")
	if fallbackToken == "" {
		fallbackToken = os.Getenv("DISCORD_TOKEN")
	}
	fallbackToken = strings.TrimSpace(fallbackToken)

	if fallbackToken != "" {
		if currentTokens[""] == "" {
			currentTokens[""] = fallbackToken
			currentStatuses[""] = "online"
		}
	}

	// 2. Compute differences
	toStart := make(map[string]string)
	toStop := make([]string, 0, len(s.instances))
	s.log().Debug("Tracking configuration deltas",
		slog.Int("current_tokens", len(currentTokens)),
		slog.Int("current_instances", len(s.instances)),
	)

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
			if runtime, ok := s.resolver.getRuntimes()[id]; ok && runtime.session != nil {
				if err := runtime.session.UpdateStatusComplex(discordgo.UpdateStatusData{
					Status: currentStatuses[id],
				}); err != nil {
					s.log().Warn("Failed to update discord status for instance",
						slog.String("botInstanceID", id),
						slog.String("mitigation", "operation ignored to protect main flow"),
						slog.Any("error", err),
					)
				}
			}
		}
	}

	for id := range s.instances {
		if _, exists := currentTokens[id]; !exists {
			toStop = append(toStop, id)
		}
	}

	// 3. Trigger Stops
	for _, id := range toStop {
		if state, exists := s.instances[id]; exists && state.Status != StatusStopping {
			s.log().Info("Planned instance shutdown triggered by token removal", slog.String("botInstanceID", id))
			state.Status = StatusStopping
			state.StopWG.Add(1)
			s.bgWG.Add(1)
			go func(id string, state *botInstanceState) {
				defer s.bgWG.Done()
				_ = s.executeStopAndRemove(context.Background(), id, state, nil)
			}(id, state)
		}
	}

	// 4. Trigger Starts (with Stop barrier)
	var startWG sync.WaitGroup
	for id, token := range toStart {
		var oldState *botInstanceState
		if state, exists := s.instances[id]; exists {
			if state.Status != StatusStopping {
				if state.Token != token {
					s.log().Info("Planned instance shutdown triggered by token update", slog.String("botInstanceID", id))
				} else {
					s.log().Info("Planned instance shutdown triggered by capability change", slog.String("botInstanceID", id))
				}
				state.Status = StatusStopping
				state.StopWG.Add(1)
				s.bgWG.Add(1)
				go func(id string, state *botInstanceState) {
					defer s.bgWG.Done()
					_ = s.executeStopAndRemove(context.Background(), id, state, nil)
				}(id, state)
			}
			oldState = state
		}

		s.bgWG.Add(1)
		startWG.Add(1)
		go func(id, token, status string, oldState *botInstanceState) {
			defer s.bgWG.Done()
			defer startWG.Done()
			s.awaitStopAndStart(id, token, status, oldState)
		}(id, token, currentStatuses[id], oldState)
	}

	// 5. Trigger dynamic command syncs for feature changes
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
	go func() {
		defer s.bgWG.Done()

		// Wait for starts before synchronizing commands
		startWG.Wait()
		s.resolver.markReady()

		if len(syncTasks) == 0 {
			return
		}

		eg, egCtx := errgroup.WithContext(ctx)
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
					if cm := runtime.commandHandler.GetCommandManager(); cm != nil {
						if syncErr := cm.SyncGuildCommands(t.guildID); syncErr != nil {
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
									slog.Int("synthetic_status_code", 500),
									slog.String("stacktrace", string(debug.Stack())),
									slog.String("guildID", t.guildID),
									slog.String("botInstanceID", instanceID),
									slog.Any("error", syncErr),
								)
							}
						} else {
							s.log().Info("Dynamic guild command synchronization completed", slog.String("guildID", t.guildID), slog.String("botInstanceID", instanceID))
						}
					}
				}
				return nil
			})
		}
		_ = eg.Wait()
	}()

	return nil
}

func (s *BotSupervisor) executeStopAndRemove(ctx context.Context, id string, state *botInstanceState, wgGlobal *sync.WaitGroup) error {
	if wgGlobal != nil {
		defer wgGlobal.Done()
	}
	defer state.StopWG.Done()

	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	err := s.serviceManager.StopAndRemove(stopCtx, "bot-runtime-"+id)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Conditional deletion: ensures overwritten pointers are not deleted
	if current, exists := s.instances[id]; exists && current == state {
		delete(s.instances, id)
	}

	// Remove from resolver so new events don't route here
	currentRuntimes := s.resolver.getRuntimes()
	newRuntimes := make(map[string]*botRuntime)
	for k, v := range currentRuntimes {
		if k != id {
			newRuntimes[k] = v
		}
	}
	s.resolver.swapRuntimes(newRuntimes)

	if err != nil && strings.Contains(err.Error(), "not found") {
		err = nil
	}

	if err != nil {
		s.serviceManager.ForceRemove("bot-runtime-" + id)
		state.Status = StatusError
		s.log().Error("Failed to purge I/O, escalated to ForceRemove",
			slog.String("request_id", "stop_remove_"+id),
			slog.Int("synthetic_status_code", 500),
			slog.String("stacktrace", string(debug.Stack())),
			slog.String("botInstanceID", id),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

func (s *BotSupervisor) awaitStopAndStart(id, token, status string, oldState *botInstanceState) {
	if oldState != nil {
		waitCh := make(chan struct{})
		go func() {
			oldState.StopWG.Wait()
			close(waitCh)
		}()
		select {
		case <-s.ctx.Done():
			return
		case <-waitCh:
		}
		if oldState.Status == StatusError {
			s.log().Error("Aborting secondary process initialization due to irreversible zombie state",
				slog.String("request_id", "await_start_"+id),
				slog.Int("synthetic_status_code", 500),
				slog.String("stacktrace", string(debug.Stack())),
				slog.String("botInstanceID", id),
			)
			return
		}
	}

	s.mu.Lock()
	newState := &botInstanceState{
		Token:         token,
		DiscordStatus: status,
		Status:        StatusStarting,
		StopWG:        &sync.WaitGroup{},
	}
	s.instances[id] = newState
	s.mu.Unlock()

	s.startBotInstanceBackground(id, token, status, newState)
}

func (s *BotSupervisor) startBotInstanceBackground(instanceID, token, status string, state *botInstanceState) {
	capabilities := resolveBotRuntimeCapabilities(s.configManager.Config(), instanceID)

	var runtime *botRuntime
	var err error

	baseDelay := 2 * time.Second
	maxDelay := 30 * time.Second
	maxRetries := 5

	for attempt := 0; attempt < maxRetries; attempt++ {
		s.mu.Lock()
		now := time.Now()
		var sleepDur time.Duration
		if s.nextIdentify.After(now) {
			sleepDur = s.nextIdentify.Sub(now)
			s.nextIdentify = s.nextIdentify.Add(identifyStaggerDelay)
		} else {
			s.nextIdentify = now.Add(identifyStaggerDelay)
		}
		s.mu.Unlock()

		if sleepDur > 0 {
			s.log().Debug("Transient interruption to mitigate identify rate limits",
				slog.String("botInstanceID", instanceID),
				slog.Duration("delay", sleepDur),
			)
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(sleepDur):
			}
		}

		runtime, err = openBotRuntime(resolvedBotInstance{ID: instanceID, Token: token, DiscordStatus: status}, capabilities)

		if err == nil {
			break
		}

		errStr := err.Error()
		isAuthFail := strings.Contains(errStr, "4004") ||
			strings.Contains(strings.ToLower(errStr), "authentication failed") ||
			(strings.Contains(errStr, "401") && !strings.Contains(errStr, "4014"))

		if isAuthFail {
			s.log().Error("Instance authentication compromised, triggering token revocation in configuration",
				slog.String("request_id", "auth_bot_"+instanceID),
				slog.Int("synthetic_status_code", 500),
				slog.String("stacktrace", string(debug.Stack())),
				slog.String("botInstanceID", instanceID),
				slog.Any("error", err),
			)
			if revokeErr := s.configManager.RevokeBotInstance(instanceID, token); revokeErr != nil {
				s.log().Error("Structural failure revoking instance credentials in central base",
					slog.String("request_id", "revoke_bot_"+instanceID),
					slog.Int("synthetic_status_code", 500),
					slog.String("stacktrace", string(debug.Stack())),
					slog.String("botInstanceID", instanceID),
					slog.Any("error", revokeErr),
				)
			}
			break
		}

		s.log().Warn("Interference in bot runtime initialization, triggering compensatory branch",
			slog.String("botInstanceID", instanceID),
			slog.Int("attempt", attempt+1),
			slog.String("mitigation", "executing exponential backoff algorithm"),
			slog.Any("error", err),
		)

		delay := float64(baseDelay) * float64(uint(1)<<attempt)
		if delay > float64(maxDelay) {
			delay = float64(maxDelay)
		}

		jitter := time.Duration(rand.Float64() * delay * 0.2)
		sleepTime := time.Duration(delay) + jitter

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(sleepTime):
		}
	}

	if err != nil {
		s.log().Error("Blocking failure after mitigation routine exhaustion on runtime start",
			slog.String("request_id", "start_exhaust_"+instanceID),
			slog.Int("synthetic_status_code", 500),
			slog.String("stacktrace", string(debug.Stack())),
			slog.String("botInstanceID", instanceID),
			slog.Any("error", err),
		)
		s.mu.Lock()
		if s.instances[instanceID] == state {
			state.Status = StatusError
		}
		s.mu.Unlock()

		return
	}

	if err := initializeBotRuntime(s.ctx, runtime, s.opts); err != nil {
		s.log().Error("Structural failure registering runtime operational components",
			slog.String("request_id", "init_components_"+instanceID),
			slog.Int("synthetic_status_code", 500),
			slog.String("stacktrace", string(debug.Stack())),
			slog.String("botInstanceID", instanceID),
			slog.Any("error", err),
		)
		s.mu.Lock()
		if s.instances[instanceID] == state {
			state.Status = StatusError
		}
		s.mu.Unlock()

		return
	}

	// Register in dynamic service manager to drain connections safely later
	wrapper := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     "bot-runtime-" + instanceID,
		Type:     service.TypeMonitoring,
		Priority: service.PriorityNormal,
		Start: func(ctx context.Context) error {
			s.log().Info("Architectural state transition: Executing StartAll across service manager instances",
				slog.String("botInstanceID", runtime.instanceID),
			)
			if err := runtime.serviceManager.StartAll(); err != nil {
				errWrap := fmt.Errorf("start services for %s: %w", runtime.instanceID, err)
				s.log().Error("Blocking structural failure: Service manager execution sequence aborted",
					slog.String("request_id", "startall_"+instanceID),
					slog.Int("synthetic_status_code", 500),
					slog.String("botInstanceID", instanceID),
					slog.Any("error", errWrap),
				)
				return errWrap
			}
			scheduleRuntimeConfiguredGuildLogging(runtime, s.opts.configManager, s.opts.startupTasks)
			scheduleRuntimeWarmup(ctx, runtime, s.opts.store, s.opts.startupTasks)

			s.log().Info("Runtime do bot totalmente operacional",
				slog.String("botInstanceID", runtime.instanceID),
			)
			return nil
		},
		Stop: func(ctx context.Context) error {
			shutdownBotRuntime(runtime, ctx)
			return closeDiscordSession(runtime.session)
		},
		Logger: slog.Default(),
	})

	if err := s.serviceManager.RegisterAndStart("bot-runtime-"+instanceID, wrapper); err != nil {
		s.log().Error("Failure in coupling interface with dynamic Service Manager",
			slog.String("request_id", "register_svc_"+instanceID),
			slog.Int("synthetic_status_code", 500),
			slog.String("stacktrace", string(debug.Stack())),
			slog.String("botInstanceID", instanceID),
			slog.Any("error", err),
		)
		return
	}

	// Make visible in resolver
	s.mu.Lock()
	defer s.mu.Unlock()

	// Only add if it wasn't removed while we were initializing
	if s.instances[instanceID] == state {
		state.Status = StatusRunning
		currentRuntimes := s.resolver.getRuntimes()
		newRuntimes := make(map[string]*botRuntime)
		for k, v := range currentRuntimes {
			newRuntimes[k] = v
		}
		newRuntimes[instanceID] = runtime
		s.resolver.swapRuntimes(newRuntimes)
		s.log().Info("Primary instance coupling and socket registration completed successfully", slog.String("botInstanceID", instanceID))
	}
}
