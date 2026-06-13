package embeds

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestCustomEmbedPostingSyncer(t *testing.T) {
	t.Parallel()

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	guildID := "guild-sync"
	key := "embed-key"

	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	ce := files.CustomEmbedConfig{
		Key:         key,
		Title:       "Title",
		Description: "Desc",
		Postings: []files.CustomEmbedPostingConfig{
			{ChannelID: "ch1", MessageID: "msg1"},
			{ChannelID: "ch2", MessageID: "msg2"},
		},
	}
	if err := cm.SetCustomEmbedProperties(guildID, key, ce); err != nil {
		t.Fatalf("set custom embed: %v", err)
	}
	for _, p := range ce.Postings {
		if err := cm.AddCustomEmbedPosting(guildID, key, p); err != nil {
			t.Fatalf("add posting: %v", err)
		}
	}

	var editedPaths []string
	var publisher mockPublisher
	publisher.updateFn = func(channelID, messageID string, embed files.CustomEmbedConfig) error {
		if messageID == "msg2" {
			// simulate deleted message on second posting
			return ErrPostingMissing
		}
		editedPaths = append(editedPaths, messageID)
		return nil
	}

	syncer := &customEmbedPostingSyncer{
		configManager: cm,
		publisher:     &publisher,
		dropPostings: func(c *files.ConfigManager, gid, k string, mid []string) error {
			return c.RemoveCustomEmbedPostings(gid, k, mid)
		},
	}

	result := syncer.Sync(guildID, key, ce.Postings, ce)

	if result.Edited != 1 {
		t.Fatalf("expected 1 edit, got %d", result.Edited)
	}
	if len(result.Dropped) != 1 || result.Dropped[0].MessageID != "msg2" {
		t.Fatalf("expected msg2 to be dropped")
	}

	// Verify that msg2 was dropped from config Manager
	updated, err := cm.CustomEmbed(guildID, key)
	if err != nil {
		t.Fatalf("load custom embed: %v", err)
	}
	if len(updated.Postings) != 1 || updated.Postings[0].MessageID != "msg1" {
		t.Fatalf("expected only msg1 to remain in custom embed postings, got %+v", updated.Postings)
	}
}

type mockPublisher struct {
	updateFn func(channelID, messageID string, embed files.CustomEmbedConfig) error
}

func (m *mockPublisher) UpdatePosting(channelID, messageID string, embed files.CustomEmbedConfig) error {
	return m.updateFn(channelID, messageID, embed)
}
