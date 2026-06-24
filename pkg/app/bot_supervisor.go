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
			slog.Info("Stop hook called for bot-runtime wrapper", slog.String("botInstanceID", runtime.instanceID), slog.Bool("hasArikawaState", runtime.arikawaState != nil), slog.Bool("hasCloseHook", t.Opts.discordSessionCloseHook != nil))
			shutdownBotRuntime(runtime, stopCtx)
			if runtime.arikawaState != nil {
				var err error
				if t.Opts.discordSessionCloseHook != nil {
					slog.Info("Executing discordSessionCloseHook for bot-runtime wrapper", slog.String("botInstanceID", runtime.instanceID))
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

// GatewayPresenceUpdateTask implements Task for hardware-aligned memory bounds.
type GatewayPresenceUpdateTask struct {
	ArikawaState *state.State
	Status       string
	InstanceID   string
	Logger       *slog.Logger
}

func (t *GatewayPresenceUpdateTask) Execute(ctx context.Context) error {
	updateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := t.ArikawaState.Gateway().Send(updateCtx, &gateway.UpdatePresenceCommand{
		Status: discord.Status(t.Status),
	})
	if err != nil {
		t.Logger.Warn("Failed to update discord status for instance",
			slog.String("botInstanceID", t.InstanceID),
			slog.String("mitigation", "operation ignored to protect main flow"),
			slog.Any("error", err),
		)
	}
	return nil
}

// CommandCatalogSyncTask implements Task for zero-allocation closure syncs.
type CommandCatalogSyncTask struct {
	GuildID   string
	Instances []string
	Resolver  *botRuntimeResolver
	Logger    *slog.Logger
}

func (t *CommandCatalogSyncTask) Execute(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(rand.Float64()*500) * time.Millisecond):
	}

	for _, instanceID := range t.Instances {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var runtime *botRuntime
		for id, rt := range t.Resolver.getRuntimes() {
			if id == instanceID {
				runtime = rt
				break
			}
		}
		if runtime == nil || runtime.commandHandler == nil {
			continue
		}
		if syncer := runtime.commandHandler.GetSyncer(); syncer != nil {
			appIDInt, _ := strconv.ParseInt(t.GuildID, 10, 64)
			if syncErr := syncer.SyncBulkOverwrite(discord.GuildID(appIDInt), runtime.commandHandler.GetRouter().Registry()); syncErr != nil {
				if strings.Contains(syncErr.Error(), "403") {
					t.Logger.Warn("Dynamic command synchronization ignored due to authorization barrier",
						slog.String("guildID", t.GuildID),
						slog.String("botInstanceID", instanceID),
						slog.String("mitigation", "permission bypass"),
						slog.Any("error", syncErr),
					)
				} else {
					t.Logger.Error("Structural failure synchronizing guild commands",
						slog.String("request_id", "sync_"+t.GuildID+"_"+instanceID),
						slog.String("guildID", t.GuildID),
						slog.String("botInstanceID", instanceID),
						slog.Any("error", syncErr),
					)
					return fmt.Errorf("sync bulk overwrite for guild %s: %w", t.GuildID, syncErr)
				}
			} else {
				t.Logger.Info("Dynamic guild command synchronization completed", slog.String("guildID", t.GuildID), slog.String("botInstanceID", instanceID))
			}
		}
	}
	return nil
}

// managedInstance retém a fronteira de isolamento de ciclo de vida de uma goroutine ativa.
type managedInstance struct {
	CancelContext context.CancelFunc
	Token         string
	Status        string
	Capabilities  botRuntimeCapabilities
}

type topologySpec struct {
	ActiveTokens   map[string]string
	ActiveStatus   map[string]string
	Capabilities   map[string]botRuntimeCapabilities
	GatewayUpdates []Task
	SyncTasks      []Task
}

// BotSupervisor manages the lifecycle, configuration synchronization, and background state of all active Discord bot instances.
type BotSupervisor struct {
	mu               sync.RWMutex
	trackedInstances map[string]*managedInstance

	configManager  *files.ConfigManager
	resolver       *botRuntimeResolver
	serviceManager *service.Manager
	opts           botRuntimeOptions

	ctx      context.Context
	cancel   context.CancelFunc
	group    *errgroup.Group
	groupCtx context.Context
	logger   *slog.Logger

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
	group, groupCtx := errgroup.WithContext(ctx)

	resolver := newBotRuntimeResolver(configManager, make(map[string]*botRuntime))
	svcMgr := service.NewManager()
	if opts.logger == nil {
		opts.logger = slog.Default()
	}

	supervisor := &BotSupervisor{
		trackedInstances: make(map[string]*managedInstance),
		configManager:    configManager,
		resolver:         resolver,
		serviceManager:   svcMgr,
		opts:             opts,
		ctx:              ctx,
		cancel:           cancel,
		group:            group,
		groupCtx:         groupCtx,
		logger:           opts.logger,
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
	s.onConfigChanged(context.Background(), nil, nil) // trigger initial resolution
	return nil
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

func (s *BotSupervisor) reconcileTopology(parentCtx context.Context, cmd topologySpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	desiredTokens := cmd.ActiveTokens
	desiredStatus := cmd.ActiveStatus
	desiredCaps := cmd.Capabilities

	for _, task := range cmd.GatewayUpdates {
		s.opts.startupTasks.GoHeavy("presence_update", task)
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

	// Fase 2 & 3: Inicializar novas vias de execução via errgroup
	var startWG sync.WaitGroup
	var pendingCount int
	isReady := false
	if s.resolver != nil {
		select {
		case <-s.resolver.readyCh:
			isReady = true
		default:
		}
	}

	for id, token := range desiredTokens {
		if _, active := s.trackedInstances[id]; !active {
			if !isReady {
				pendingCount++
				startWG.Add(1)
			}
			instanceCtx, instanceCancel := context.WithCancel(s.groupCtx)

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

			s.group.Go(func() error {
				s.log().Debug("Tracking complex conditional branch: Starting isolated hardware pipeline for bot instance",
					slog.String("botInstanceID", task.InstanceID),
				)

				execErrCh := make(chan error, 1)
				go func() {
					execErrCh <- task.Execute(instanceCtx)
				}()

				select {
				case <-instanceCtx.Done():
				case res := <-resultCh:
					if res.err != nil {
						s.log().Error("Structural execution failure during bot startup sequence", slog.Any("error", res.err))
					} else {
						s.resolver.addRuntime(task.InstanceID, res.runtime)
					}
				}

				if !isReady {
					startWG.Done()
				}

				<-instanceCtx.Done()

				s.log().Info("Deterministic lifecycle orchestration: Context cancellation verified; freeing stack resources")
				cleanupTask := InstanceStopTask{
					InstanceID: task.InstanceID,
					SvcMgr:     s.serviceManager,
					Logger:     s.logger,
					Resolver:   s.resolver,
				}
				cleanupErr := cleanupTask.Execute(context.Background())

				<-execErrCh

				if s.ctx.Err() != nil {
					return cleanupErr
				}
				return nil
			})
		}
	}

	if !isReady {
		if pendingCount == 0 {
			s.resolver.markReady()
		} else {
			go func() {
				startWG.Wait()
				s.resolver.markReady()
			}()
		}
	}

	// Fase 3: Sync Commands
	if len(cmd.SyncTasks) > 0 {
		s.opts.startupTasks.GoHeavy("catalog_sync", &CatalogSyncGroupTask{
			SyncTasks: cmd.SyncTasks,
		})
	}

	return nil
}

// CatalogSyncGroupTask agrupa e executa tarefas de sincronização em concorrência limitada.
type CatalogSyncGroupTask struct {
	SyncTasks []Task
}

func (t *CatalogSyncGroupTask) Execute(ctx context.Context) error {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(10)
	for _, taskFn := range t.SyncTasks {
		tFn := taskFn
		eg.Go(func() error {
			return tFn.Execute(egCtx)
		})
	}
	return eg.Wait()
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

	var gatewayUpdates []Task

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

						gatewayUpdates = append(gatewayUpdates, &GatewayPresenceUpdateTask{
							ArikawaState: gwState,
							Status:       st,
							InstanceID:   instanceID,
							Logger:       s.log(),
						})
					}
				}
			}
		}
	}

	var syncTasks []Task
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
					syncTasks = append(syncTasks, &CommandCatalogSyncTask{
						GuildID:   newGuild.GuildID,
						Instances: activeInstances,
						Resolver:  s.resolver,
						Logger:    s.log(),
					})
				}
			}
		}
	}

	return s.reconcileTopology(ctx, topologySpec{
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
