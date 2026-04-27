package app

import (
	"reflect"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestInitializeBotRuntimeEnforcesGuildAllowlistOnStartup(t *testing.T) {
	t.Parallel()

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	state := discordgo.NewState()
	state.User = &discordgo.User{ID: "bot-id", Username: "alice", Discriminator: "0001", Bot: true}
	state.Ready.Guilds = []*discordgo.Guild{
		{ID: "1375650791251120179"},
		{ID: "guild-denied"},
		{ID: " guild-denied "},
		{ID: "1390069056530419823"},
	}
	session.State = state

	origLeaveRuntimeGuild := leaveRuntimeGuild
	leftGuilds := make([]string, 0, 1)
	leaveRuntimeGuild = func(session *discordgo.Session, guildID string) error {
		leftGuilds = append(leftGuilds, guildID)
		return nil
	}
	t.Cleanup(func() {
		leaveRuntimeGuild = origLeaveRuntimeGuild
	})

	runtime := &botRuntime{
		instanceID: "alice",
		session:    session,
	}
	if err := initializeBotRuntime(runtime, botRuntimeOptions{
		defaultBotInstanceID: "alice",
		runtimeCount:         1,
		configManager:        files.NewMemoryConfigManager(),
	}); err != nil {
		t.Fatalf("initialize bot runtime: %v", err)
	}
	t.Cleanup(func() {
		if runtime.cleanupStop != nil {
			close(runtime.cleanupStop)
			runtime.cleanupStop = nil
		}
	})

	if want := []string{"guild-denied"}; !reflect.DeepEqual(leftGuilds, want) {
		t.Fatalf("unexpected left guilds: got=%v want=%v", leftGuilds, want)
	}

	gotGuildIDs, err := listBotGuildIDsFromSessionState(session)
	if err != nil {
		t.Fatalf("list guild ids after startup allowlist: %v", err)
	}
	if want := []string{"1375650791251120179", "1390069056530419823"}; !reflect.DeepEqual(gotGuildIDs, want) {
		t.Fatalf("unexpected startup guild ids after allowlist: got=%v want=%v", gotGuildIDs, want)
	}
	if runtime.cleanupStop == nil {
		t.Fatal("expected runtime cleanup channel after guild allowlist handler registration")
	}
}

func TestHandleRuntimeGuildCreateLeavesUnauthorizedGuild(t *testing.T) {
	t.Parallel()

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}

	origLeaveRuntimeGuild := leaveRuntimeGuild
	leftGuilds := make([]string, 0, 1)
	leaveRuntimeGuild = func(session *discordgo.Session, guildID string) error {
		leftGuilds = append(leftGuilds, guildID)
		return nil
	}
	t.Cleanup(func() {
		leaveRuntimeGuild = origLeaveRuntimeGuild
	})

	handleRuntimeGuildCreate(session, "alice", &discordgo.GuildCreate{
		Guild: &discordgo.Guild{ID: "guild-denied"},
	})
	handleRuntimeGuildCreate(session, "alice", &discordgo.GuildCreate{
		Guild: &discordgo.Guild{ID: "1375650791251120179"},
	})

	if want := []string{"guild-denied"}; !reflect.DeepEqual(leftGuilds, want) {
		t.Fatalf("unexpected guild create leaves: got=%v want=%v", leftGuilds, want)
	}
}

func TestHandleRuntimeGuildDeleteRemovesDisallowedGuildConfig(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	if _, err := configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{GuildID: "1375650791251120179"},
			{GuildID: "guild-denied"},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	handleRuntimeGuildDelete("alice", &discordgo.GuildDelete{
		Guild: &discordgo.Guild{ID: "guild-denied"},
	}, configManager, nil)

	got := configManager.SnapshotConfig()
	if len(got.Guilds) != 1 || got.Guilds[0].GuildID != "1375650791251120179" {
		t.Fatalf("unexpected config after guild delete cleanup: %+v", got.Guilds)
	}

	handleRuntimeGuildDelete("alice", &discordgo.GuildDelete{
		Guild: &discordgo.Guild{ID: "1375650791251120179"},
	}, configManager, nil)
	handleRuntimeGuildDelete("alice", &discordgo.GuildDelete{
		Guild: &discordgo.Guild{ID: "guild-unavailable", Unavailable: true},
	}, configManager, nil)

	got = configManager.SnapshotConfig()
	if len(got.Guilds) != 1 || got.Guilds[0].GuildID != "1375650791251120179" {
		t.Fatalf("expected allowed and unavailable deletes to be ignored, got %+v", got.Guilds)
	}
}