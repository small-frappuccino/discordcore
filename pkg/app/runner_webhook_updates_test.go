package app

import (
	"encoding/json"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			WebhookEmbedUpdates: []files.WebhookEmbedUpdateConfig{
				{
					MessageID:  "global-1",
					WebhookURL: "https://discord.com/api/webhooks/1/token",
					Embed:      json.RawMessage(`{"title":"g1"}`),
				},
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID: "guild-a",
				RuntimeConfig: files.RuntimeConfig{
					WebhookEmbedUpdates: []files.WebhookEmbedUpdateConfig{
						{
							MessageID:  "guild-a-1",
							WebhookURL: "https://discord.com/api/webhooks/2/token",
							Embed:      json.RawMessage(`{"title":"a1"}`),
						},
						{
							MessageID:  "guild-a-2",
							WebhookURL: "https://discord.com/api/webhooks/3/token",
							Embed:      json.RawMessage(`{"title":"a2"}`),
						},
					},
				},
			},
			{
				GuildID: "guild-b",
				RuntimeConfig: files.RuntimeConfig{
					// Legacy single-item field should be included via NormalizedWebhookEmbedUpdates().
					WebhookEmbedUpdate: files.WebhookEmbedUpdateConfig{
						MessageID:  "guild-b-legacy",
						WebhookURL: "https://discord.com/api/webhooks/4/token",
						Embed:      json.RawMessage(`{"title":"b-legacy"}`),
					},
				},
			},
		},
	}

	got := collectStartupWebhookEmbedUpdates(cfg)
	if len(got) != 4 {
		t.Fatalf("expected 4 startup updates, got %d", len(got))
	}

	if got[0].scope != "global" || got[0].index != 0 || got[0].update.MessageID != "global-1" {
		t.Fatalf("unexpected first item: %+v", got[0])
	}
	if got[1].scope != "guild:guild-a" || got[1].index != 0 || got[1].update.MessageID != "guild-a-1" {
		t.Fatalf("unexpected second item: %+v", got[1])
	}
	if got[2].scope != "guild:guild-a" || got[2].index != 1 || got[2].update.MessageID != "guild-a-2" {
		t.Fatalf("unexpected third item: %+v", got[2])
	}
	if got[3].scope != "guild:guild-b" || got[3].index != 0 || got[3].update.MessageID != "guild-b-legacy" {
		t.Fatalf("unexpected fourth item: %+v", got[3])
	}
}

func TestCollectStartupWebhookEmbedUpdatesNilConfig(t *testing.T) {
	t.Parallel()

	if got := collectStartupWebhookEmbedUpdates(nil); len(got) != 0 {
		t.Fatalf("expected nil/empty list for nil config, got %+v", got)
	}
}
