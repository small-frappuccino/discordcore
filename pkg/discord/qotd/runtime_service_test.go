package qotd

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type fakeGuildLifecycleService struct {
	publishCalls   []string
	reconcileCalls []string
}

func (f *fakeGuildLifecycleService) PublishScheduledIfDue(_ context.Context, guildID string, _ *discordgo.Session) (bool, error) {
	f.publishCalls = append(f.publishCalls, guildID)
	return false, nil
}

func (f *fakeGuildLifecycleService) ReconcileGuild(_ context.Context, guildID string, _ *discordgo.Session) error {
	f.reconcileCalls = append(f.reconcileCalls, guildID)
	return nil
}

func TestRuntimeServiceCyclesUseScopedGuilds(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	for _, guild := range []files.GuildConfig{
		{
			GuildID:       "g-enabled",
			BotInstanceID: "alice",
			QOTD: files.QOTDConfig{
				Enabled:        true,
				ForumChannelID: "forum-enabled",
			},
		},
		{
			GuildID:       "g-configured-disabled",
			BotInstanceID: "alice",
			QOTD: files.QOTDConfig{
				ForumChannelID: "forum-disabled",
			},
		},
		{
			GuildID:       "g-other-runtime",
			BotInstanceID: "other",
			QOTD: files.QOTDConfig{
				Enabled:        true,
				ForumChannelID: "forum-other",
			},
		},
		{
			GuildID:       "g-empty",
			BotInstanceID: "alice",
		},
	} {
		if err := configManager.AddGuildConfig(guild); err != nil {
			t.Fatalf("AddGuildConfig(%s) failed: %v", guild.GuildID, err)
		}
	}

	fake := &fakeGuildLifecycleService{}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "alice", "alice")
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.runPublishCycle(service.clock())
	service.runReconcileCycle(service.clock())

	if len(fake.publishCalls) != 1 || fake.publishCalls[0] != "g-enabled" {
		t.Fatalf("expected publish cycle to include only enabled guilds on the runtime, got %v", fake.publishCalls)
	}
	if len(fake.reconcileCalls) != 2 {
		t.Fatalf("expected reconcile cycle to include configured guilds on the runtime, got %v", fake.reconcileCalls)
	}
	if fake.reconcileCalls[0] != "g-enabled" || fake.reconcileCalls[1] != "g-configured-disabled" {
		t.Fatalf("unexpected reconcile target order: %v", fake.reconcileCalls)
	}
}
