package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestSupervisorFaultIsolation(t *testing.T) {
	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origOpenBotDiscordSession := openBotDiscordSession
	origSetupCommandHandler := setupCommandHandler
	origShutdownCommandHandler := shutdownCommandHandler
	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		openBotDiscordSession = origOpenBotDiscordSession
		setupCommandHandler = origSetupCommandHandler
		shutdownCommandHandler = origShutdownCommandHandler
		identifyStaggerDelay = 5 * time.Second
	})

	identifyStaggerDelay = 0
	setupCommandHandler = func(ch *commands.CommandHandler) error { return nil }
	shutdownCommandHandler = func(ch *commands.CommandHandler) error { return nil }

	session1, _ := discordgo.New("Bot token1")
	session1.State.User = &discordgo.User{ID: "child1"}
	session2, _ := discordgo.New("Bot token2")
	session2.State.User = &discordgo.User{ID: "child2"}

	newDiscordSession = func(token string) (*discordgo.Session, error) {
		if token == "token1" {
			return session1, nil
		}
		if token == "token2" {
			return session2, nil
		}
		return nil, errors.New("unknown token")
	}
	newDiscordSessionWithIntents = func(token string, _ discordgo.Intent) (*discordgo.Session, error) {
		if token == "token1" {
			return session1, nil
		}
		if token == "token2" {
			return session2, nil
		}
		return nil, errors.New("unknown token")
	}

	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {
		if s == session2 {
			return errors.New("simulated gateway panic in child runtime ID 2")
		}
		return nil
	}

	cfgManager := files.NewConfigManagerWithStore(nil)
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
				},
			},
		},
	}
	cfgManager.ApplyConfig(&cfg)

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{defaultBotInstanceID: "child1", configManager: cfgManager})

	fatalCount := 0
	supervisor.SetFatalCallback(func(err error) {
		fatalCount++
	})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	// Wait for instances to reach final state
	for i := 0; i < 20; i++ {
		supervisor.mu.Lock()
		s1 := supervisor.instances["child1"]
		s2 := supervisor.instances["child2"]
		ready := s1 != nil && s1.Status == StatusRunning && s2 != nil && s2.Status == StatusStarting
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
	cfgManager := files.NewConfigManagerWithStore(nil)
	cfg := files.BotConfig{
		Guilds: []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg)

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{defaultBotInstanceID: "child1", configManager: cfgManager})

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
	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origOpenBotDiscordSession := openBotDiscordSession
	origSetupCommandHandler := setupCommandHandler
	origShutdownCommandHandler := shutdownCommandHandler
	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		openBotDiscordSession = origOpenBotDiscordSession
		setupCommandHandler = origSetupCommandHandler
		shutdownCommandHandler = origShutdownCommandHandler
		identifyStaggerDelay = 5 * time.Second
	})

	identifyStaggerDelay = 0
	setupCommandHandler = func(ch *commands.CommandHandler) error { return nil }
	shutdownCommandHandler = func(ch *commands.CommandHandler) error { return nil }

	newDiscordSession = func(token string) (*discordgo.Session, error) {
		s, _ := discordgo.New(token)
		s.State.User = &discordgo.User{ID: token}
		return s, nil
	}
	newDiscordSessionWithIntents = func(token string, _ discordgo.Intent) (*discordgo.Session, error) {
		s, _ := discordgo.New(token)
		s.State.User = &discordgo.User{ID: token}
		return s, nil
	}
	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {
		return nil
	}

	cfgManager := files.NewConfigManagerWithStore(nil)

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

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{defaultBotInstanceID: "child1", configManager: cfgManager})

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
	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origOpenBotDiscordSession := openBotDiscordSession
	origSetupCommandHandler := setupCommandHandler
	origShutdownCommandHandler := shutdownCommandHandler
	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		openBotDiscordSession = origOpenBotDiscordSession
		setupCommandHandler = origSetupCommandHandler
		shutdownCommandHandler = origShutdownCommandHandler
		identifyStaggerDelay = 5 * time.Second
	})

	identifyStaggerDelay = 0
	setupCommandHandler = func(ch *commands.CommandHandler) error { return nil }
	shutdownCommandHandler = func(ch *commands.CommandHandler) error { return nil }

	session1, _ := discordgo.New("token1")
	session1.State.User = &discordgo.User{ID: "child1"}

	newDiscordSession = func(token string) (*discordgo.Session, error) {
		if token == "token1" || token == "token2" {
			return session1, nil
		}
		return nil, errors.New("unknown token")
	}
	newDiscordSessionWithIntents = func(token string, _ discordgo.Intent) (*discordgo.Session, error) {
		if token == "token1" || token == "token2" {
			return session1, nil
		}
		return nil, errors.New("unknown token")
	}
	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {
		return nil
	}

	cfgManager := files.NewConfigManagerWithStore(nil)
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

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{defaultBotInstanceID: "child1", configManager: cfgManager})

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
	supervisor.onConfigChanged(nil, &cfg2)

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
	supervisor.onConfigChanged(nil, &cfg3)

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
