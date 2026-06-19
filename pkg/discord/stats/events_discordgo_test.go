package stats

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	domain "github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordgo"
)

func TestRegisterDiscordGoEventHandlers(t *testing.T) {
	session := &discordgo.Session{}
	logger := slog.Default()

	// Should not panic
	RegisterDiscordGoEventHandlers(session, nil, logger)
}

func TestHandleDiscordGoGuildMemberAdd(t *testing.T) {
	// Nil checks
	handleDiscordGoGuildMemberAdd(nil, nil)

	store, db, cleanup := setupTestDB(t)
	if store == nil {
		t.Skip("skipping db tests")
	}
	defer cleanup()

	cm := newTestConfigManager(t)
	_, _ = cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{{GuildID: "g1", BotInstanceTokens: map[string]files.EncryptedString{"test": "token"}, FeatureRouting: map[string]string{"stats": "test"}, Features: files.FeatureToggles{}, Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}}}
		return nil
	})

	svc := domain.NewStatsService(nil, cm, store, slog.Default(), "test")
	if store != nil {
		_, _ = db.Exec(context.Background(), "INSERT INTO guilds (id) VALUES ('g1') ON CONFLICT DO NOTHING")
	}

	m := &discordgo.GuildMemberAdd{
		Member: &discordgo.Member{
			User: &discordgo.User{
				ID:  "u1",
				Bot: true,
			},
			JoinedAt: time.Now(),
			Roles:    []string{"r1", "r2"},
		},
	}
	m.GuildID = "g1"

	// Should not panic, hits business logic which drops the event without store
	handleDiscordGoGuildMemberAdd(svc, m)
}

func TestHandleDiscordGoGuildMemberRemove(t *testing.T) {
	handleDiscordGoGuildMemberRemove(nil, nil)

	store, db, cleanup := setupTestDB(t)
	if store == nil {
		t.Skip("skipping db tests")
	}
	defer cleanup()
	cm := newTestConfigManager(t)
	_, _ = cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{{GuildID: "g1", BotInstanceTokens: map[string]files.EncryptedString{"test": "token"}, FeatureRouting: map[string]string{"stats": "test"}, Features: files.FeatureToggles{}, Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}}}
		return nil
	})

	svc := domain.NewStatsService(nil, cm, store, slog.Default(), "test")
	if store != nil {
		_, _ = db.Exec(context.Background(), "INSERT INTO guilds (id) VALUES ('g1') ON CONFLICT DO NOTHING")
	}

	m := &discordgo.GuildMemberRemove{
		Member: &discordgo.Member{
			User: &discordgo.User{
				ID: "u1",
			},
		},
	}
	m.GuildID = "g1"

	// Should not panic
	handleDiscordGoGuildMemberRemove(svc, m)
}

func TestHandleDiscordGoGuildMemberUpdate(t *testing.T) {
	handleDiscordGoGuildMemberUpdate(nil, nil)

	store, db, cleanup := setupTestDB(t)
	if store == nil {
		t.Skip("skipping db tests")
	}
	defer cleanup()
	cm := newTestConfigManager(t)
	_, _ = cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{{GuildID: "g1", BotInstanceTokens: map[string]files.EncryptedString{"test": "token"}, FeatureRouting: map[string]string{"stats": "test"}, Features: files.FeatureToggles{}, Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}}}
		return nil
	})

	svc := domain.NewStatsService(nil, cm, store, slog.Default(), "test")
	if store != nil {
		_, _ = db.Exec(context.Background(), "INSERT INTO guilds (id) VALUES ('g1') ON CONFLICT DO NOTHING")
	}

	m := &discordgo.GuildMemberUpdate{
		Member: &discordgo.Member{
			User: &discordgo.User{
				ID:  "u1",
				Bot: false,
			},
			Roles: []string{"r1"},
		},
	}
	m.GuildID = "g1"

	// Should not panic
	handleDiscordGoGuildMemberUpdate(svc, m)
}
