package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestSupervisorFaultIsolation(t *testing.T) {
	origFetchBotArikawaMe := fetchBotArikawaMe
	t.Cleanup(func() {
		fetchBotArikawaMe = origFetchBotArikawaMe
	})
	fetchBotArikawaMe = func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	origOpenBotArikawaState := openBotArikawaState
	t.Cleanup(func() {
		openBotArikawaState = origOpenBotArikawaState
	})
	openBotArikawaState = func(ctx context.Context, s *state.State) error {
		token := s.Token
		if token == "Bot token2" {
			return errors.New("simulated gateway panic in child runtime ID 2")
		} else if token == "Bot token3" {
			return errors.New("HTTP 401 Unauthorized")
		}
		return nil
	}
	origSetupCommandHandler := setupCommandHandler
	origShutdownCommandHandler := shutdownCommandHandler
	t.Cleanup(func() {
		setupCommandHandler = origSetupCommandHandler
		shutdownCommandHandler = origShutdownCommandHandler
		identifyStaggerDelay = 5 * time.Second
	})

	identifyStaggerDelay = 0
	setupCommandHandler = func(ch *CommandHandler) error { return nil }
	shutdownCommandHandler = func(ch *CommandHandler) error { return nil }

	cfgManager := files.NewConfigManagerWithStore(nil, nil)
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

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{configManager: cfgManager})

	fatalCount := 0
	supervisor.SetFatalCallback(func(err error) {
		fatalCount++
	})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	// Wait for instances to reach final state
	for i := 0; i < 40; i++ {
		supervisor.mu.Lock()
		s1 := supervisor.instances["child1"]
		s2 := supervisor.instances["child2"]
		s3 := supervisor.instances["child3"]
		ready := s1 != nil && s1.Status == StatusRunning && s2 != nil && s2.Status == StatusStarting && s3 == nil // s3 revoked
		supervisor.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(50 * time.Millisecond)
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

	// Stop supervisor gracefully
	if err := supervisor.Stop(context.Background()); err != nil {
		t.Fatalf("supervisor stop: %v", err)
	}
}

func TestZeroStateIdling(t *testing.T) {
	cfgManager := files.NewConfigManagerWithStore(nil, nil)
	cfg := files.BotConfig{
		Guilds: []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg)

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{configManager: cfgManager})

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
	origFetchBotArikawaMe := fetchBotArikawaMe
	t.Cleanup(func() {
		fetchBotArikawaMe = origFetchBotArikawaMe
	})
	fetchBotArikawaMe = func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	origOpenBotArikawaState := openBotArikawaState
	t.Cleanup(func() {
		openBotArikawaState = origOpenBotArikawaState
	})
	openBotArikawaState = func(ctx context.Context, s *state.State) error { return nil }
	origSetupCommandHandler := setupCommandHandler
	origShutdownCommandHandler := shutdownCommandHandler
	t.Cleanup(func() {
		setupCommandHandler = origSetupCommandHandler
		shutdownCommandHandler = origShutdownCommandHandler
		identifyStaggerDelay = 5 * time.Second
	})

	identifyStaggerDelay = 0
	setupCommandHandler = func(ch *CommandHandler) error { return nil }
	shutdownCommandHandler = func(ch *CommandHandler) error { return nil }

	cfgManager := files.NewConfigManagerWithStore(nil, nil)

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

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{configManager: cfgManager})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	for i := 0; i < 50; i++ {
		supervisor.mu.Lock()
		count := 0
		for _, state := range supervisor.instances {
			if state.Status == StatusRunning {
				count++
			}
		}
		supervisor.mu.Unlock()
		if count == 10 {
			break
		}
		time.Sleep(50 * time.Millisecond)
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
	origFetchBotArikawaMe := fetchBotArikawaMe
	t.Cleanup(func() {
		fetchBotArikawaMe = origFetchBotArikawaMe
	})
	fetchBotArikawaMe = func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	origOpenBotArikawaState := openBotArikawaState
	t.Cleanup(func() {
		openBotArikawaState = origOpenBotArikawaState
	})
	openBotArikawaState = func(ctx context.Context, s *state.State) error { return nil }
	origSetupCommandHandler := setupCommandHandler
	origShutdownCommandHandler := shutdownCommandHandler
	t.Cleanup(func() {
		setupCommandHandler = origSetupCommandHandler
		shutdownCommandHandler = origShutdownCommandHandler
		identifyStaggerDelay = 5 * time.Second
	})

	identifyStaggerDelay = 0
	setupCommandHandler = func(ch *CommandHandler) error { return nil }
	shutdownCommandHandler = func(ch *CommandHandler) error { return nil }

	cfgManager := files.NewConfigManagerWithStore(nil, nil)
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

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{configManager: cfgManager})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	for i := 0; i < 50; i++ {
		supervisor.mu.Lock()
		state1 := supervisor.instances["child1"]
		ready := state1 != nil && state1.Status == StatusRunning
		supervisor.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(50 * time.Millisecond)
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

	for i := 0; i < 50; i++ {
		supervisor.mu.Lock()
		state1 := supervisor.instances["child1"]
		ready := state1 != nil && state1.Token == "token2" && state1.Status == StatusRunning
		supervisor.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cfg3 := files.BotConfig{
		Features: cfg.Features,
		Guilds:   []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg3)
	supervisor.onConfigChanged(context.Background(), nil, &cfg3)

	for i := 0; i < 50; i++ {
		supervisor.mu.Lock()
		state1 := supervisor.instances["child1"]
		supervisor.mu.Unlock()
		if state1 == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
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
