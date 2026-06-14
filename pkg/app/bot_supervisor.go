package app

import (
	"context"
	"math/rand/v2"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordgo"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/service"
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
		ctx:            ctx,
		cancel:         cancel,
		instances:      make(map[string]*botInstanceState),
	}

	return supervisor
}

// SetFatalCallback configures a callback to be invoked when a critical background failure occurs.
func (s *BotSupervisor) SetFatalCallback(cb func(error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fatalCallback = cb
}

func (s *BotSupervisor) Start() error {
	s.onConfigChanged(nil, nil) // trigger initial resolution
	return nil
}

func (s *BotSupervisor) Stop(ctx context.Context) error {
	s.cancel() // signal background goroutines to abort

	var globalWG sync.WaitGroup

	s.mu.Lock()
	for id, state := range s.instances {
		if state.Status != StatusStopping {
			state.Status = StatusStopping
			state.StopWG.Add(1)
			globalWG.Add(1)
			s.bgWG.Add(1)
			go func(id string, state *botInstanceState) {
				defer s.bgWG.Done()
				s.executeStopAndRemove(ctx, id, state, &globalWG)
			}(id, state)
		}
	}
	s.mu.Unlock()

	// Barreira obrigatória: impede encerramento do processo enquanto há I/O pendente
	globalWG.Wait()
	// Barreira secundária: garante que processos em bg como retries limpem a memória
	s.bgWG.Wait()
	return nil
}

func (s *BotSupervisor) GetResolver() *botRuntimeResolver {
	return s.resolver
}

func (s *BotSupervisor) onConfigChanged(oldCfg, newCfg *files.BotConfig) {
	if newCfg == nil {
		snap := s.configManager.SnapshotConfig()
		newCfg = &snap
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

	// 2. Compute differences
	toStart := make(map[string]string)
	toStop := make([]string, 0)
	capsChangedMap := make(map[string]bool)

	for id, token := range currentTokens {
		oldState, exists := s.instances[id]
		var capsChanged bool
		if exists && oldCfg != nil {
			oldCaps := resolveBotRuntimeCapabilities(oldCfg, id)
			newCaps := resolveBotRuntimeCapabilities(newCfg, id)
			if oldCaps != newCaps {
				capsChanged = true
				capsChangedMap[id] = true
			}
		}

		if !exists || oldState.Token != token || capsChanged {
			toStart[id] = token
		} else if oldState.DiscordStatus != currentStatuses[id] {
			oldState.DiscordStatus = currentStatuses[id]
			if runtime, ok := s.resolver.getRuntimes()[id]; ok && runtime.session != nil {
				_ = runtime.session.UpdateStatusComplex(discordgo.UpdateStatusData{
					Status: currentStatuses[id],
				})
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
			log.ApplicationLogger().Info("Stopping bot instance due to token removal", "botInstanceID", id)
			state.Status = StatusStopping
			state.StopWG.Add(1)
			s.bgWG.Add(1)
			go func(id string, state *botInstanceState) {
				defer s.bgWG.Done()
				s.executeStopAndRemove(context.Background(), id, state, nil)
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
					log.ApplicationLogger().Info("Stopping bot instance due to token update", "botInstanceID", id)
				} else {
					log.ApplicationLogger().Info("Stopping bot instance due to capability change", "botInstanceID", id)
				}
				state.Status = StatusStopping
				state.StopWG.Add(1)
				s.bgWG.Add(1)
				go func(id string, state *botInstanceState) {
					defer s.bgWG.Done()
					s.executeStopAndRemove(context.Background(), id, state, nil)
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
	if oldCfg != nil {
		for _, newGuild := range newCfg.Guilds {
			var oldGuild *files.GuildConfig
			for i := range oldCfg.Guilds {
				if oldCfg.Guilds[i].GuildID == newGuild.GuildID {
					oldGuild = &oldCfg.Guilds[i]
					break
				}
			}
			if oldGuild == nil {
				continue
			}
			if !reflect.DeepEqual(oldGuild.FeatureRouting, newGuild.FeatureRouting) ||
				!reflect.DeepEqual(oldGuild.Features, newGuild.Features) {

				guildID := newGuild.GuildID
				var activeInstances []string
				for instanceID, token := range newGuild.BotInstanceTokens {
					if string(token) != "" {
						activeInstances = append(activeInstances, instanceID)
					}
				}

				s.bgWG.Add(1)
				go func(gID string, instances []string) {
					defer s.bgWG.Done()
					// Small debounce jitter
					time.Sleep(time.Duration(rand.Float64()*500) * time.Millisecond)

					currentRuntimes := s.resolver.getRuntimes()
					for _, instanceID := range instances {
						runtime, ok := currentRuntimes[instanceID]
						if !ok || runtime == nil || runtime.commandHandler == nil {
							continue
						}
						if cm := runtime.commandHandler.GetCommandManager(); cm != nil {
							if syncErr := cm.SyncGuildCommands(gID); syncErr != nil {
								if strings.Contains(syncErr.Error(), "403") {
									log.ApplicationLogger().Warn("dynamic command sync forbidden (missing scope?)", "guildID", gID, "botInstanceID", instanceID, "error", syncErr)
								} else {
									log.ApplicationLogger().Error("failed dynamic guild command sync", "guildID", gID, "botInstanceID", instanceID, "error", syncErr)
								}
							} else {
								log.ApplicationLogger().Info("Completed dynamic guild command sync", "guildID", gID, "botInstanceID", instanceID)
							}
						}
					}
				}(guildID, activeInstances)
			}
		}
	}

	go func() {
		startWG.Wait()
		s.resolver.markReady()
	}()
}

func (s *BotSupervisor) executeStopAndRemove(ctx context.Context, id string, state *botInstanceState, wgGlobal *sync.WaitGroup) {
	if wgGlobal != nil {
		defer wgGlobal.Done()
	}
	defer state.StopWG.Done()

	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	err := s.serviceManager.StopAndRemove(stopCtx, "bot-runtime-"+id)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.serviceManager.ForceRemove("bot-runtime-" + id)
		state.Status = StatusError
		log.ApplicationLogger().Error("falha no expurgo I/O, escalado para ForceRemove", "botInstanceID", id, "error", err)
		return
	}

	// Deleção condicional: garante que não apague ponteiros sobrescritos
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
			log.ApplicationLogger().Error("abortando startup devido a estado zumbi não resolvido", "botInstanceID", id)
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
		if strings.Contains(errStr, "401") || strings.Contains(errStr, "4004") || strings.Contains(strings.ToLower(errStr), "authentication failed") {
			log.ApplicationLogger().Warn("Bot authentication failed permanently, revoking token from configuration", "botInstanceID", instanceID, "error", err)
			_ = s.configManager.RevokeBotInstance(instanceID, token)
			break
		}

		log.ApplicationLogger().Warn("failed to open bot runtime, retrying", "botInstanceID", instanceID, "attempt", attempt+1, "error", err)

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
		log.ApplicationLogger().Error("failed to open bot runtime after retries", "botInstanceID", instanceID, "error", err)
		s.mu.Lock()
		if s.instances[instanceID] == state {
			state.Status = StatusError
		}
		s.mu.Unlock()

		return
	}

	if err := initializeBotRuntime(s.ctx, runtime, s.opts); err != nil {
		log.ApplicationLogger().Error("failed to initialize bot runtime", "botInstanceID", instanceID, "error", err)
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
			return nil
		},
		Stop: func(context.Context) error {
			shutdownBotRuntime(runtime, context.Background())
			return closeDiscordSession(runtime.session)
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
	if s.instances[instanceID] == state {
		state.Status = StatusRunning
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
