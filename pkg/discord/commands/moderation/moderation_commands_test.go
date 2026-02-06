package moderation

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestBuildMassBanLogDetails(t *testing.T) {
	t.Parallel()

	got := buildMassBanLogDetails(10, 7, []string{"a"}, []string{"b"}, []string{"c", "d"})
	if got == "" {
		t.Fatal("expected details string")
	}
	if !containsAll(got, []string{"Total: 10", "Banned: 7", "Invalid: 1", "Skipped: 1", "Failed: 2"}) {
		t.Fatalf("unexpected details: %q", got)
	}
}

func TestBuildBanCommandMessageUsesUsername(t *testing.T) {
	t.Parallel()

	got := buildBanCommandMessage("alice", "rule violation", false)
	if !containsAll(got, []string{"alice", "rule violation"}) {
		t.Fatalf("unexpected message: %q", got)
	}
}

func TestBuildMassBanCommandMessageOnlyCount(t *testing.T) {
	t.Parallel()

	got := buildMassBanCommandMessage(4)
	if got != "Banned 4 user(s)." {
		t.Fatalf("unexpected message: %q", got)
	}
}

func TestSendModerationLogNoChannel(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	botID := "bot"

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}
	if cm.Config() == nil {
		t.Fatal("config is nil")
	}
	cm.Config().RuntimeConfig.ModerationLogMode = "alice_only"

	session := &discordgo.Session{State: discordgo.NewState()}
	session.State.User = &discordgo.User{ID: botID}

	ctx := &core.Context{
		Session: session,
		Config:  cm,
		GuildID: guildID,
		UserID:  "user",
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("sendModerationLog panicked: %v", r)
		}
	}()

	sendModerationLog(ctx, moderationLogPayload{
		Action:      "ban",
		TargetID:    "target",
		TargetLabel: "target",
		Reason:      "reason",
		RequestedBy: "user",
	})
}

func containsAll(value string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
