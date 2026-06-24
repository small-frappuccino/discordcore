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
	"github.com/small-frappuccino/discordcore/pkg/service"
	"golang.org/x/sync/errgroup"
	"sync"
)

// InstanceStatus represents the lifecycle state of a managed bot instance.
type InstanceStatus string

const (
	StatusStarting InstanceStatus = "starting"
	StatusRunning  InstanceStatus = "running"
	StatusStopping InstanceStatus = "stopping"
	StatusError    InstanceStatus = "error"
)

// Task define o contrato estrito para execução assíncrona sem coerção de tipo ou buffers ocultos.
type Task interface {
	Execute(ctx context.Context) error
}

type startTaskResult struct {
	runtime *botRuntime
	err     error
}

// InstanceStartTask encapsula os parâmetros invariantes necessários para acoplamento de sockets.
type InstanceStartTask struct {
	InstanceID    string
	Token         string
	DiscordStatus string
	Capabilities  botRuntimeCapabilities
	Opts          botRuntimeOptions
	Resolver      *botRuntimeResolver
	SvcMgr        *service.Manager
	ResultCh      chan<- startTaskResult
}

var (
	baseBackoffDelay = float64(2 * time.Second)
	maxBackoffDelay  = float64(30 * time.Second)
)

// Execute executa a inicialização alocando dados estritamente dentro de seu próprio stack frame.
func (t InstanceStartTask) Execute(ctx context.Context) error {
	t.Opts.logger.Debug("Tracking complex conditional branch: Starting isolated hardware pipeline for bot instance",
		slog.String("botInstanceID", t.InstanceID),
	)

	var runtime *botRuntime
	var err error

	for attempt := 0; attempt < 5; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		runtime, err = openBotRuntimeFn(resolvedBotInstance{ID: t.InstanceID, Token: t.Token, DiscordStatus: t.DiscordStatus}, t.Capabilities, t.Opts)
		if err == nil {
			break
		}

		if checkTokenRevocationError(err.Error()) {
			t.Opts.logger.Warn("Instance authentication compromised, triggering token revocation",
				slog.String("botInstanceID", t.InstanceID),
				slog.Any("error", err),
			)
			_ = t.Opts.configManager.RevokeBotInstance(t.InstanceID, t.Token)
			break
		}

		delay := baseBackoffDelay * float64(uint(1)<<attempt)
		if delay > maxBackoffDelay {
			delay = maxBackoffDelay
		}
		sleepTime := time.Duration(delay + (rand.Float64() * delay * 0.2))

		timer := time.NewTimer(sleepTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	if err != nil {
		t.ResultCh <- startTaskResult{err: err}
		return err
	}

	if ctx.Err() != nil {
		shutdownBotRuntime(runtime, context.Background())
		return ctx.Err()
	}

	if err := initializeBotRuntime(ctx, runtime, t.Opts); err != nil {
		shutdownBotRuntime(runtime, context.Background())
		t.ResultCh <- startTaskResult{err: err}
		return err
	}

	serviceName := "bot-runtime-" + t.InstanceID
	wrapper := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     serviceName,
		Type:     service.TypeMonitoring,
		Priority: service.PriorityNormal,
		Start: func(startCtx context.Context) error {
			if err := runtime.serviceManager.StartAll(); err != nil {
				return fmt.Errorf("start services for %s: %w", runtime.instanceID, err)
			}
			scheduleRuntimeConfiguredGuildLogging(runtime, t.Opts.configManager, t.Opts.startupTasks)
			scheduleRuntimeWarmup(startCtx, runtime, t.Opts.store, t.Opts.startupTasks)
			return nil
		},
		Stop: func(stopCtx context.Context) error {
			shutdownBotRuntime(runtime, stopCtx)
			if runtime.arikawaState != nil {
				var err error
				if t.Opts.discordSessionCloseHook != nil {
					err = t.Opts.discordSessionCloseHook(runtime.arikawaState)
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

	if err := t.SvcMgr.RegisterAndStart(serviceName, wrapper); err != nil {
		if strings.Contains(err.Error(), "already registered") {
			t.SvcMgr.ForceRemove(serviceName)
			_ = t.SvcMgr.RegisterAndStart(serviceName, wrapper)
		} else {
			t.Opts.logger.Error("Fatal failure coupling interface with Service Manager", slog.Any("error", err))
			t.ResultCh <- startTaskResult{err: err}
			return err
		}
	}

	t.ResultCh <- startTaskResult{runtime: runtime, err: nil}
	return nil
}

// InstanceStopTask gerencia o desmembramento determinístico de recursos de rede.
type InstanceStopTask struct {
	InstanceID string
	SvcMgr     *service.Manager
	Logger     *slog.Logger
	Resolver   *botRuntimeResolver
}

func (t InstanceStopTask) Execute(ctx context.Context) error {
	t.Logger.Info("Executing structural teardown payload for instance", slog.String("botInstanceID", t.InstanceID))

	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	svcName := "bot-runtime-" + t.InstanceID
	err := t.SvcMgr.StopAndRemove(stopCtx, svcName)

	if err != nil && strings.Contains(err.Error(), "not found") {
		err = nil
	}

	if err != nil {
		t.SvcMgr.ForceRemove(svcName)
		t.Logger.Error("Failed to purge I/O, escalated to ForceRemove",
			slog.String("botInstanceID", t.InstanceID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// CommandType define os discriminadores de instrução para o anel de execução.
type CommandType int

const (
	ApplyTopology CommandType = iota
	TerminateSupervisor
)

// TopologyCommand carrega o manifesto de configuração atualizado de forma atômica.
type TopologyCommand struct {
	Type           CommandType
	ActiveTokens   map[string]string
	ActiveStatus   map[string]string
	Capabilities   map[string]botRuntimeCapabilities
	GatewayUpdates []func(context.Context) error
	SyncTasks      []func(context.Context) error
	ExecutionError chan error
}

// managedInstance retém a fronteira de isolamento de ciclo de vida de uma goroutine ativa.
type managedInstance struct {
	CancelContext context.CancelFunc
	Token         string
	Status        string
	Capabilities  botRuntimeCapabilities
}

// SupervisorActor centraliza a mutação de estado em uma única linha de execução sequencial.
type SupervisorActor struct {
	mailbox          chan TopologyCommand
	trackedInstances map[string]*managedInstance
	logger           *slog.Logger
	opts             botRuntimeOptions
	resolver         *botRuntimeResolver
	serviceManager   *service.Manager
	routinesWg       sync.WaitGroup
}

// NewSupervisorActor garante a inicialização fail-fast do orquestrador de topologia.
func NewSupervisorActor(bufferSize int, logger *slog.Logger, opts botRuntimeOptions, resolver *botRuntimeResolver, serviceManager *service.Manager) (*SupervisorActor, error) {
	if logger == nil {
		return nil, errors.New("initialization failure: logger dependency injection is strictly required")
	}
	if bufferSize <= 0 {
		bufferSize = 10
	}

	return &SupervisorActor{
		mailbox:          make(chan TopologyCommand, bufferSize),
		trackedInstances: make(map[string]*managedInstance),
		logger:           logger,
		opts:             opts,
		resolver:         resolver,
		serviceManager:   serviceManager,
	}, nil
}

// SubmitTopology encaminha uma nova instrução de roteamento para o loop do Ator com bloqueio guiado por contexto.
func (s *SupervisorActor) SubmitTopology(ctx context.Context, cmd TopologyCommand) {
	select {
	case s.mailbox <- cmd:
	case <-ctx.Done():
		s.logger.Warn("Shedding load: topology command mailbox timed out or cancelled")
		if cmd.ExecutionError != nil {
			cmd.ExecutionError <- fmt.Errorf("supervisor mailbox saturation: %w", ctx.Err())
		}
	}
}

// RunLoop inicializa o anel de processamento linear do Ator. Deve ser invocado em uma goroutine dedicada.
func (s *SupervisorActor) RunLoop(ctx context.Context) error {
	s.logger.Info("Architectural state transition: SupervisorActor loop initialized on hardware execution ring")

	for {
		select {
		case <-ctx.Done():
			s.executeEmergencyTeardown()
			return ctx.Err()

		case cmd := <-s.mailbox:
			switch cmd.Type {
			case TerminateSupervisor:
				s.executeEmergencyTeardown()
				if cmd.ExecutionError != nil {
					close(cmd.ExecutionError)
				}
				return nil

			case ApplyTopology:
				err := s.reconcileTopology(ctx, cmd)
				if cmd.ExecutionError != nil {
					cmd.ExecutionError <- err
				}
			}
		}
	}
}

// reconcileTopology resolve desvios de configuração comparando o estado desejado com o atual.
func (s *SupervisorActor) reconcileTopology(parentCtx context.Context, cmd TopologyCommand) error {
	desiredTokens := cmd.ActiveTokens
	desiredStatus := cmd.ActiveStatus
	desiredCaps := cmd.Capabilities

	for _, fn := range cmd.GatewayUpdates {
		s.opts.startupTasks.GoHeavy("presence_update", fn)
	}

	// Fase 1: Expurgar instâncias removidas ou alteradas (Cancelamento Determinístico Directo)
	for id, current := range s.trackedInstances {
		newToken, exists := desiredTokens[id]
		newCaps := desiredCaps[id]
		if !exists || newToken != current.Token || !reflect.DeepEqual(current.Capabilities, newCaps) {
			s.logger.Info("Architectural state transition: Actively canceling compromised or obsolete configuration",
				slog.String("botInstanceID", id),
			)
			current.CancelContext()
			delete(s.trackedInstances, id)

			s.resolver.removeRuntime(id)
		}
	}

	// Fase 2: Inicializar novas vias de execução sem micro-race conditions
	for id, token := range desiredTokens {
		if _, active := s.trackedInstances[id]; !active {
			instanceCtx, instanceCancel := context.WithCancel(parentCtx)

			s.trackedInstances[id] = &managedInstance{
				CancelContext: instanceCancel,
				Token:         token,
				Status:        desiredStatus[id],
				Capabilities:  desiredCaps[id],
			}

			resultCh := make(chan startTaskResult, 1)
			task := InstanceStartTask{
				InstanceID:    id,
				Token:         token,
				DiscordStatus: desiredStatus[id],
				Capabilities:  desiredCaps[id],
				Opts:          s.opts,
				Resolver:      s.resolver,
				SvcMgr:        s.serviceManager,
				ResultCh:      resultCh,
			}

			go s.launchInstanceRoutine(instanceCtx, task, resultCh)
		}
	}

	// Fase 3: Sync Commands
	if len(cmd.SyncTasks) > 0 {
		s.opts.startupTasks.GoHeavy("catalog_sync", func(heavyCtx context.Context) error {
			eg, egCtx := errgroup.WithContext(heavyCtx)
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

	return nil
}

// [CORREÇÃO] Acoplamento do WaitGroup na inicialização
func (s *SupervisorActor) launchInstanceRoutine(ctx context.Context, task InstanceStartTask, resultCh <-chan startTaskResult) {
	s.routinesWg.Add(1)
	go func() {
		defer s.routinesWg.Done()

		s.logger.Debug("Tracking complex conditional branch: Starting isolated hardware pipeline for bot instance",
			slog.String("botInstanceID", task.InstanceID),
		)

		go func() {
			_ = task.Execute(ctx)
		}()

		select {
		case <-ctx.Done():
			return
		case res := <-resultCh:
			if res.err != nil {
				s.logger.Error("Structural execution failure during bot startup sequence", slog.Any("error", res.err))
				return
			}
			s.resolver.addRuntime(task.InstanceID, res.runtime)
		}
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("Deterministic lifecycle orchestration: Context cancellation verified; freeing stack resources")
				cleanupTask := InstanceStopTask{
					InstanceID: task.InstanceID,
					SvcMgr:     s.serviceManager,
					Logger:     s.logger,
					Resolver:   s.resolver,
				}
				_ = cleanupTask.Execute(context.Background())
				return
			}
		}
	}()
}

// [CORREÇÃO] O teardown agora aguarda o esvaziamento determinístico do hardware execution ring.
func (s *SupervisorActor) executeEmergencyTeardown() {
	s.logger.Warn("Planned instance shutdown: Emergency teardown initiated across all managed contexts")
	for id, instance := range s.trackedInstances {
		instance.CancelContext()
		delete(s.trackedInstances, id)
	}
	// Bloqueia a execução de saída até que I/O e I/O cleanup estejam resolvidos
	s.logger.Info("Awaiting strict boundary teardown of all instance routines")
	s.routinesWg.Wait()
	s.logger.Info("All hardware execution rings flushed. Teardown complete.")
}

// BotSupervisor manages the lifecycle, configuration synchronization, and background state of all active Discord bot instances.
type BotSupervisor struct {
	configManager  *files.ConfigManager
	resolver       *botRuntimeResolver
	serviceManager *service.Manager
	opts           botRuntimeOptions

	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger

	actor *SupervisorActor

	fatalCallback func(error)
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
	resolver := newBotRuntimeResolver(configManager, make(map[string]*botRuntime))
	svcMgr := service.NewManager()
	if opts.logger == nil {
		opts.logger = slog.Default()
	}

	actor, _ := NewSupervisorActor(50, opts.logger, opts, resolver, svcMgr)

	supervisor := &BotSupervisor{
		configManager:  configManager,
		resolver:       resolver,
		serviceManager: svcMgr,
		opts:           opts,
		logger:         opts.logger,
		ctx:            ctx,
		cancel:         cancel,
		actor:          actor,
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
	go s.actor.RunLoop(s.ctx)
	s.onConfigChanged(context.Background(), nil, nil) // trigger initial resolution
	return nil
}

// Stop initiates a graceful shutdown of all managed bot instances and waits for background processes to terminate.
func (s *BotSupervisor) Stop(ctx context.Context) error {
	s.log().Info("Triggering planned shutdown of main BotSupervisor instances")

	errCh := make(chan error, 1)
	s.actor.SubmitTopology(ctx, TopologyCommand{
		Type:           TerminateSupervisor,
		ExecutionError: errCh,
	})

	s.cancel() // signal background goroutines to abort

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
		return s.serviceManager.StopAll(ctx)
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
		oldRuntimes := s.resolver.getRuntimes()
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
					if rt, ok := oldRuntimes[id]; ok && rt.arikawaState != nil {
						st := currentStatuses[id]
						gwState := rt.arikawaState
						instanceID := id
						gatewayUpdates = append(gatewayUpdates, func(taskCtx context.Context) error {
							updateCtx, cancel := context.WithTimeout(taskCtx, 5*time.Second)
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
					gID := newGuild.GuildID
					instances := activeInstances
					syncTasks = append(syncTasks, func(egCtx context.Context) error {
						select {
						case <-egCtx.Done():
							return egCtx.Err()
						case <-time.After(time.Duration(rand.Float64()*500) * time.Millisecond):
						}

						currentRuntimes := s.resolver.getRuntimes()
						for _, instanceID := range instances {
							if egCtx.Err() != nil {
								return egCtx.Err()
							}
							runtime, ok := currentRuntimes[instanceID]
							if !ok || runtime == nil || runtime.commandHandler == nil {
								continue
							}
							if syncer := runtime.commandHandler.GetSyncer(); syncer != nil {
								appIDInt, _ := strconv.ParseInt(gID, 10, 64)
								if syncErr := syncer.SyncBulkOverwrite(discord.GuildID(appIDInt), runtime.commandHandler.GetRouter().Registry()); syncErr != nil {
									if strings.Contains(syncErr.Error(), "403") {
										s.log().Warn("Dynamic command synchronization ignored due to authorization barrier",
											slog.String("guildID", gID),
											slog.String("botInstanceID", instanceID),
											slog.String("mitigation", "permission bypass"),
											slog.Any("error", syncErr),
										)
									} else {
										s.log().Error("Structural failure synchronizing guild commands",
											slog.String("request_id", "sync_"+gID+"_"+instanceID),
											slog.String("guildID", gID),
											slog.String("botInstanceID", instanceID),
											slog.Any("error", syncErr),
										)
										return fmt.Errorf("sync bulk overwrite for guild %s: %w", gID, syncErr)
									}
								} else {
									s.log().Info("Dynamic guild command synchronization completed", slog.String("guildID", gID), slog.String("botInstanceID", instanceID))
								}
							}
						}
						return nil
					})
				}
			}
		}
	}

	errCh := make(chan error, 1)
	s.actor.SubmitTopology(ctx, TopologyCommand{
		Type:           ApplyTopology,
		ActiveTokens:   currentTokens,
		ActiveStatus:   currentStatuses,
		Capabilities:   currentCaps,
		GatewayUpdates: gatewayUpdates,
		SyncTasks:      syncTasks,
		ExecutionError: errCh,
	})

	err := <-errCh
	if err != nil {
		s.log().Error("Topology reconciliation failed", slog.Any("error", err))
	}
	return err
}

// checkTokenRevocationError validates if an external string strictly matches auth failure invariants.
func checkTokenRevocationError(errStr string) bool {
	lowerErr := strings.ToLower(errStr)
	return strings.Contains(lowerErr, "4004") ||
		strings.Contains(lowerErr, "authentication failed") ||
		(strings.Contains(lowerErr, "401") && !strings.Contains(lowerErr, "4014"))
}
