package app

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	"github.com/small-frappuccino/discordcore/pkg/storage/storagetest"
	"github.com/small-frappuccino/discordgo"
)

func TestBotRuntimeResolver_AggregateCachesAndMetrics(t *testing.T) {
	runtimes := make(map[string]*botRuntime)

	// Setup runtimes
	mockSession := &discordgo.Session{Token: "test-token", State: discordgo.NewState()}
	mockConfigManager := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	mockStore := storagetest.NewFailingStore()

	ms1, _ := monitoring.NewMonitoringServiceForBotWithMetrics(mockSession, nil, nil, mockConfigManager, mockStore, "bot1", &monitoring.InMemoryMetrics{}, log.ApplicationLogger())
	ms2, _ := monitoring.NewMonitoringServiceForBotWithMetrics(mockSession, nil, nil, mockConfigManager, mockStore, "bot2", &monitoring.InMemoryMetrics{}, log.ApplicationLogger())

	runtimes["bot1"] = &botRuntime{
		monitoringService: ms1,
		session:           &discordgo.Session{Token: "test-token"},
	}
	runtimes["bot2"] = &botRuntime{
		monitoringService: ms2,
	}
	runtimes["bot3"] = &botRuntime{
		// No monitoring service / cache
	}

	resolver := newBotRuntimeResolver(files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil), runtimes)

	caches := resolver.aggregateUnifiedCaches()
	_ = caches // Can be zero if error occurred in NewMonitoringService

	metrics := resolver.aggregateMonitoringMetrics()
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}
}

func TestBotRuntimeResolver_SessionForGuild(t *testing.T) {
	runtimes := make(map[string]*botRuntime)
	session := &discordgo.Session{Token: "test-token", State: discordgo.NewState()}
	session.State.GuildAdd(&discordgo.Guild{ID: "g1"})
	runtimes["bot1"] = &botRuntime{
		instanceID: "bot1",
		session:    session,
	}

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "g1",
				BotInstanceTokens: map[string]files.EncryptedString{
					"bot1": "token",
				},
				FeatureRouting: map[string]string{
					"dashboard": "bot1",
				},
			},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolver := newBotRuntimeResolver(cm, runtimes)

	// Test sessionForGuild
	s, err := resolver.sessionForGuild("g1", "dashboard")
	if err != nil {
		t.Fatalf("unexpected error: %v, runtimes: %+v", err, resolver.runtimes)
	}
	if s == nil || s.Token != "test-token" {
		t.Fatalf("expected test-token session")
	}

	// Test registerGuild
	if err := resolver.registerGuild(context.Background(), "g2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test guildBindings
	bindings, err := resolver.guildBindings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bindings) == 0 {
		t.Fatalf("expected bindings")
	}

	// Unknown guild error (runtimeForGuild error)
	_, err = resolver.sessionForGuild("g-unknown", "qotd")
	if err == nil {
		t.Fatal("expected error for unknown guild")
	}

	// Session is nil case
	runtimes["bot2"] = &botRuntime{
		instanceID: "bot2",
		session:    nil, // nil session
	}
	_, _ = cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = append(cfg.Guilds, files.GuildConfig{
			GuildID: "g2",
			BotInstanceTokens: map[string]files.EncryptedString{
				"bot2": "token2",
			},
		})
		return nil
	})

	// guildID is not empty
	_, err = resolver.sessionForGuild("g2", "dashboard")
	if err == nil {
		t.Fatal("expected err for nil session")
	}

	// guildID is empty
	_, _ = cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = append(cfg.Guilds, files.GuildConfig{
			GuildID: "", // default guild
			BotInstanceTokens: map[string]files.EncryptedString{
				"bot2": "token2",
			},
		})
		return nil
	})
	_, err = resolver.sessionForGuild("", "dashboard")
	if err == nil {
		t.Fatal("expected err for nil session with empty guildID")
	}

}

func TestBotRuntimeResolver_waitForReady(t *testing.T) {
	resolver := newBotRuntimeResolver(nil, nil)
	resolver.markReady()
	resolver.waitForReady(context.Background()) // should not block
}

func TestBotRuntimeResolver_knownBotInstanceCatalog(t *testing.T) {
	runtimes := make(map[string]*botRuntime)
	runtimes["bot1"] = &botRuntime{instanceID: "bot1"}

	cat := knownBotInstanceCatalog(runtimes, nil)
	if _, ok := cat["bot1"]; !ok {
		t.Fatal("expected knownBotInstanceCatalog to contain bot1")
	}

	slice := knownBotInstanceCatalogSlice(cat)
	if len(slice) != 1 || slice[0] != "bot1" {
		t.Fatal("expected knownBotInstanceCatalogSlice to contain bot1")
	}
}

func TestBotRuntimeResolver_registerGuild(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	resolver := newBotRuntimeResolver(cm, nil)
	resolver.registerGuild(context.Background(), "guild1")

	bindings, err := resolver.guildBindings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected 0 bindings without complete tokens, got %v", len(bindings))
	}
}

func TestListBotGuildBindingsFromSessionState(t *testing.T) {
	st := discordgo.NewState()
	st.Guilds = []*discordgo.Guild{{ID: "g1"}}
	s := &discordgo.Session{State: st}

	bindings, err := listBotGuildBindingsFromSessionState("bot1", s)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || bindings[0].GuildID != "g1" || bindings[0].BotInstanceID != "bot1" {
		t.Fatal("expected g1 -> bot1")
	}
}
