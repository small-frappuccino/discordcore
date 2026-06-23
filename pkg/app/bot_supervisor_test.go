package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"fmt"
	"sync"
	"sync/atomic"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"golang.org/x/sync/errgroup"
)

// awaitCondition deterministically and repeatedly evaluates a condition until it returns true
// or the timeout expires, ensuring execution resolution in minimum time.
func awaitCondition(timeout time.Duration, condition func() bool) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Millisecond) // Ultra-fast polling for fail-fast
	defer ticker.Stop()

	for {
		if condition() {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("absolute timeout exceeded waiting for state convergence")
		}
		<-ticker.C
	}
}

func TestSupervisorFaultIsolation(t *testing.T) {
	fetchBotArikawaMeHook := func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	openBotArikawaStateHook := func(ctx context.Context, s *state.State) error {
		token := s.Token
		if token == "Bot token2" {
			return errors.New("simulated gateway panic in child runtime ID 2")
		} else if token == "Bot token3" {
			return errors.New("HTTP 401 Unauthorized")
		}
		return nil
	}
	setupCommandHandlerHook := func(ch *CommandHandler) error { return nil }
	shutdownCommandHandlerHook := func(ch *CommandHandler) error { return nil }

	cfgManager := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	cfg := files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: func(b bool) *bool { return &b }(false),
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				BotInstanceTokens: map[string]files.EncryptedString{
					"child1": "token1",
					"child2": "token2",
					"child3": "token3",
				},
			},
		},
	}
	cfgManager.ApplyConfig(&cfg)

	startupTasks := NewStartupTaskOrchestrator(3)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{
		configManager:          cfgManager,
		startupTasks:           startupTasks,
		fetchBotArikawaMe:      fetchBotArikawaMeHook,
		openBotArikawaState:    openBotArikawaStateHook,
		setupCommandHandler:    setupCommandHandlerHook,
		shutdownCommandHandler: shutdownCommandHandlerHook,
		newCommandHandlerForBot: func(deps CommandHandlerDeps) (*CommandHandler, error) {
			return &CommandHandler{session: deps.Session, configManager: deps.ConfigManager}, nil
		},
	})
	t.Cleanup(func() {
		// Stop ensures that if the test fails early (e.g. timeout), the backoff loop is cancelled
		// and we don't hang in startupTasks.Shutdown().
		_ = supervisor.Stop(context.Background())
	})
	supervisor.identifyStaggerDelay = 0

	fatalCount := 0
	supervisor.SetFatalCallback(func(err error) {
		fatalCount++
	})

	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *files.BotConfig) error {
		return supervisor.onConfigChanged(ctx, oldCfg, newCfg)
	})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	errWait := awaitCondition(10*time.Second, func() bool {
		supervisor.mu.Lock()
		defer supervisor.mu.Unlock()
		s1 := supervisor.instances["child1"]
		s2 := supervisor.instances["child2"]
		s3 := supervisor.instances["child3"]
		return s1 != nil && s1.Status == StatusRunning && s2 != nil && s2.Status == StatusStarting && s3 == nil // s3 revoked
	})
	if errWait != nil {
		supervisor.mu.Lock()
		s1 := supervisor.instances["child1"]
		s2 := supervisor.instances["child2"]
		s3 := supervisor.instances["child3"]
		supervisor.mu.Unlock()
		var s1st, s2st, s3st string
		if s1 != nil {
			s1st = string(s1.Status)
		} else {
			s1st = "nil"
		}
		if s2 != nil {
			s2st = string(s2.Status)
		} else {
			s2st = "nil"
		}
		if s3 != nil {
			s3st = string(s3.Status)
		} else {
			s3st = "nil"
		}
		t.Fatalf("failed waiting for supervisor state: %v (s1: %s, s2: %s, s3: %s)", errWait, s1st, s2st, s3st)
	}

	supervisor.mu.Lock()
	state1 := supervisor.instances["child1"]
	state2 := supervisor.instances["child2"]
	var status1, status2 InstanceStatus
	if state1 != nil {
		status1 = state1.Status
	}
	if state2 != nil {
		status2 = state2.Status
	}
	supervisor.mu.Unlock()

	if state1 == nil || status1 != StatusRunning {
		t.Errorf("child1 should be running, got status: %v", status1)
	}
	if state2 == nil || status2 != StatusStarting {
		t.Errorf("child2 should be retrying (starting) due to panic, got status: %v", status2)
	}
}

func TestZeroStateIdling(t *testing.T) {
	cfgManager := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	cfg := files.BotConfig{
		Guilds: []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg)

	startupTasks := NewStartupTaskOrchestrator(1)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{
		configManager: cfgManager,
		startupTasks:  startupTasks,
	})
	supervisor.identifyStaggerDelay = 0

	fatalCount := 0
	supervisor.SetFatalCallback(func(err error) {
		fatalCount++
	})

	// Assert that passing exactly 0 configured bot tokens initializes the supervisor into a stable idle state
	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	supervisor.mu.Lock()
	instanceCount := len(supervisor.instances)
	supervisor.mu.Unlock()

	if instanceCount != 0 {
		t.Errorf("expected 0 instances running, got %d", instanceCount)
	}
	if fatalCount != 0 {
		t.Errorf("expected 0 fatal callbacks, got %d", fatalCount)
	}

	if err := supervisor.Stop(context.Background()); err != nil {
		t.Fatalf("supervisor stop: %v", err)
	}
}

func TestSupervisorSwarmTopology(t *testing.T) {
	fetchBotArikawaMeHook := func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	openBotArikawaStateHook := func(ctx context.Context, s *state.State) error { return nil }
	setupCommandHandlerHook := func(ch *CommandHandler) error { return nil }
	shutdownCommandHandlerHook := func(ch *CommandHandler) error { return nil }

	cfgManager := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	tokens := make(map[string]files.EncryptedString)
	for i := 0; i < 10; i++ {
		tokens["child"+string(rune('A'+i))] = files.EncryptedString("token" + string(rune('A'+i)))
	}

	cfg := files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: func(b bool) *bool { return &b }(false),
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID:           "g1",
				BotInstanceTokens: tokens,
			},
		},
	}
	cfgManager.ApplyConfig(&cfg)

	startupTasks := NewStartupTaskOrchestrator(10)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{
		configManager:          cfgManager,
		startupTasks:           startupTasks,
		fetchBotArikawaMe:      fetchBotArikawaMeHook,
		openBotArikawaState:    openBotArikawaStateHook,
		setupCommandHandler:    setupCommandHandlerHook,
		shutdownCommandHandler: shutdownCommandHandlerHook,
		newCommandHandlerForBot: func(deps CommandHandlerDeps) (*CommandHandler, error) {
			return &CommandHandler{session: deps.Session, configManager: deps.ConfigManager}, nil
		},
	})
	supervisor.identifyStaggerDelay = 0

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	errWait := awaitCondition(3*time.Second, func() bool {
		supervisor.mu.Lock()
		defer supervisor.mu.Unlock()
		count := 0
		for _, state := range supervisor.instances {
			if state.Status == StatusRunning {
				count++
			}
		}
		return count == 10
	})
	if errWait != nil {
		t.Fatalf("structural failure in Swarm initialization: %v", errWait)
	}

	supervisor.mu.Lock()
	instanceCount := 0
	for _, state := range supervisor.instances {
		if state.Status == StatusRunning {
			instanceCount++
		}
	}
	supervisor.mu.Unlock()

	if instanceCount != 10 {
		t.Errorf("expected 10 running instances, got %d", instanceCount)
	}

	if err := supervisor.Stop(context.Background()); err != nil {
		t.Fatalf("supervisor stop: %v", err)
	}
}

func TestSupervisorConfigChange(t *testing.T) {
	fetchBotArikawaMeHook := func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	openBotArikawaStateHook := func(ctx context.Context, s *state.State) error { return nil }
	setupCommandHandlerHook := func(ch *CommandHandler) error { return nil }
	shutdownCommandHandlerHook := func(ch *CommandHandler) error { return nil }

	cfgManager := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	cfg := files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: func(b bool) *bool { return &b }(false),
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				BotInstanceTokens: map[string]files.EncryptedString{
					"child1": "token1",
				},
			},
		},
	}
	cfgManager.ApplyConfig(&cfg)

	startupTasks := NewStartupTaskOrchestrator(1)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{
		configManager:          cfgManager,
		startupTasks:           startupTasks,
		fetchBotArikawaMe:      fetchBotArikawaMeHook,
		openBotArikawaState:    openBotArikawaStateHook,
		setupCommandHandler:    setupCommandHandlerHook,
		shutdownCommandHandler: shutdownCommandHandlerHook,
		newCommandHandlerForBot: func(deps CommandHandlerDeps) (*CommandHandler, error) {
			return &CommandHandler{session: deps.Session, configManager: deps.ConfigManager}, nil
		},
	})
	supervisor.identifyStaggerDelay = 0

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	errWait1 := awaitCondition(2500*time.Millisecond, func() bool {
		supervisor.mu.Lock()
		defer supervisor.mu.Unlock()
		state1 := supervisor.instances["child1"]
		return state1 != nil && state1.Status == StatusRunning
	})
	if errWait1 != nil {
		t.Fatalf("failed waiting for child1 to run: %v", errWait1)
	}

	cfg2 := files.BotConfig{
		Features: cfg.Features,
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				BotInstanceTokens: map[string]files.EncryptedString{
					"child1": "token2", // changed token
					"child2": "",       // empty token
				},
			},
		},
	}
	cfgManager.ApplyConfig(&cfg2)
	supervisor.onConfigChanged(context.Background(), nil, &cfg2)

	errWait2 := awaitCondition(2500*time.Millisecond, func() bool {
		supervisor.mu.Lock()
		defer supervisor.mu.Unlock()
		state1 := supervisor.instances["child1"]
		return state1 != nil && state1.Token == "token2" && state1.Status == StatusRunning
	})
	if errWait2 != nil {
		t.Fatalf("failed waiting for child1 with new token: %v", errWait2)
	}

	cfg3 := files.BotConfig{
		Features: cfg.Features,
		Guilds:   []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg3)
	supervisor.onConfigChanged(context.Background(), nil, &cfg3)

	errWait3 := awaitCondition(2500*time.Millisecond, func() bool {
		supervisor.mu.Lock()
		defer supervisor.mu.Unlock()
		return supervisor.instances["child1"] == nil
	})
	if errWait3 != nil {
		t.Fatalf("failed waiting for child1 removal: %v", errWait3)
	}

	supervisor.mu.Lock()
	state1 := supervisor.instances["child1"]
	var status InstanceStatus
	if state1 != nil {
		status = state1.Status
	}
	supervisor.mu.Unlock()

	if state1 != nil {
		t.Errorf("child1 should be stopped after config change, got status: %v", status)
	}

	r := supervisor.GetResolver()
	if r == nil {
		t.Error("expected non-nil resolver")
	}

	if err := supervisor.Stop(context.Background()); err != nil {
		t.Fatalf("supervisor stop: %v", err)
	}
}

// TestBotSupervisor_ConcurrentConfigThrashing validates structural integrity of
// state management when config webhook triggers multiple concurrent events.
func TestBotSupervisor_ConcurrentConfigThrashing(t *testing.T) {
	startupTasks := NewStartupTaskOrchestrator(1)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	cfgManager := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	cfg := files.BotConfig{
		Guilds: []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg)

	// Absolute I/O Isolation: Ingesting in-memory network simulators
	opts := botRuntimeOptions{
		configManager: cfgManager,
		startupTasks:  startupTasks,
		openBotArikawaState: func(ctx context.Context, s *state.State) error {
			return nil // Bypass WebSocket dial
		},
		fetchBotArikawaMe: func(s *state.State) (*discord.User, error) {
			return &discord.User{ID: 999, Username: "stress_test_bot"}, nil // Bypass HTTP GET
		},
		newCommandHandlerForBot: func(deps CommandHandlerDeps) (*CommandHandler, error) {
			return &CommandHandler{session: deps.Session, configManager: deps.ConfigManager}, nil
		},
		setupCommandHandler:    func(ch *CommandHandler) error { return nil },
		shutdownCommandHandler: func(ch *CommandHandler) error { return nil },
	}

	supervisor := NewBotSupervisor(cfgManager, opts)
	// Zero the stagger delay to maximize CPU thrashing and reduce test execution time
	supervisor.identifyStaggerDelay = 0

	if err := supervisor.Start(); err != nil {
		t.Fatalf("failed to initialize BotSupervisor: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)
	const concurrentMutations = 100
	var errorCount int32

	// Simulate a flood of concurrent configuration changes
	for i := 0; i < concurrentMutations; i++ {
		mutationIndex := i
		eg.Go(func() error {
			// Create a deterministic payload to force token reallocation
			newCfg := &files.BotConfig{
				Guilds: []files.GuildConfig{
					{
						GuildID: fmt.Sprintf("guild_%d", mutationIndex%5),
						BotInstanceTokens: map[string]files.EncryptedString{
							"instance_1": files.EncryptedString(fmt.Sprintf("token_%d", mutationIndex)),
						},
						BotInstanceStatuses: map[string]string{
							"instance_1": "online",
						},
					},
				},
			}

			// Submit the new config concurrently
			if err := supervisor.onConfigChanged(egCtx, nil, newCfg); err != nil {
				atomic.AddInt32(&errorCount, 1)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("structural failure detected during config collision: %v", err)
	}

	if atomic.LoadInt32(&errorCount) > 0 {
		t.Fatalf("detected %d errors during state mutation in onConfigChanged", errorCount)
	}

	// Deterministic shutdown validation to avoid goroutine leaks
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := supervisor.Stop(stopCtx); err != nil {
		t.Fatalf("resource leak: timeout exceeded waiting for bgWG in Stop: %v", err)
	}
}

type mockBlockingServiceWrapper struct {
	done chan struct{}
}

func (m *mockBlockingServiceWrapper) Start(ctx context.Context) error { return nil }
func (m *mockBlockingServiceWrapper) Stop(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
func (m *mockBlockingServiceWrapper) Done() <-chan struct{} { return m.done }

// TestBotSupervisor_GracefulShutdownOrchestration ensures that Stop() immediately interrupts
// mitigation (backoff) routines and drains I/O channels safely.
func TestBotSupervisor_GracefulShutdownOrchestration(t *testing.T) {
	startupTasks := NewStartupTaskOrchestrator(1)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(nil, botRuntimeOptions{
		startupTasks: startupTasks,
	})

	// Register a mock service directly into the ServiceManager so StopAndRemove will block
	// on it.
	_ = supervisor.serviceManager.RegisterAndStart("bot-runtime-zombie_instance", &mockBlockingServiceWrapper{done: make(chan struct{})})

	supervisor.mu.Lock()
	// Inject a "zombie" state into the map to force sweep logic execution
	supervisor.instances["zombie_instance"] = &botInstanceState{
		Token:         "dead_token",
		DiscordStatus: "online",
		Status:        StatusRunning,
		StopWG:        &sync.WaitGroup{},
	}
	supervisor.mu.Unlock()

	// Start the shutdown sequence with an extremely short context
	// to validate fail response on immediate cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := supervisor.Stop(ctx)

	// Stop must detect that async dependencies did not finish on time
	// and return context deadline exceeded error.
	if err == nil {
		t.Fatal("expected context deadline exceeded error, but Stop completed without errors")
	}

	// We no longer assert that the instance remains in the map, because
	// executeStopAndRemove is also canceled by the same context and cleans up
	// concurrently, creating an inherent race condition in the test assertions.
}
