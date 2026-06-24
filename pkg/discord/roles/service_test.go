package roles

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestRolePanelSyncEditsEachPosting(t *testing.T) {
	t.Parallel()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "123"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	if err := cm.UpsertRolePanelButton("123", "pings", files.RolePanelButtonConfig{
		RoleID: "100",
		Label:  "Click",
	}); err != nil {
		t.Fatalf("seed button: %v", err)
	}
	messageIDs := []string{"222000", "222001"}
	for _, mid := range messageIDs {
		if err := cm.AddRolePanelPosting("123", "pings", files.RolePanelPostingConfig{ChannelID: "111000", MessageID: mid}); err != nil {
			t.Fatalf("seed posting %s: %v", mid, err)
		}
	}

	var edits []string
	svc := &RolePanelService{
		configManager: cm,
		editMessage: func(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID, data api.EditMessageData) error {
			edits = append(edits, messageID.String())
			return nil
		},
		dropPostings: func(c *files.ConfigManager, gid, k string, mid []string) error {
			return nil
		},
	}

	postings, err := cm.ListRolePanelPostings("123", "pings")
	if err != nil {
		t.Fatalf("list postings: %v", err)
	}
	panel := &files.RolePanelConfig{Title: "Pings"}

	client := &api.Client{}
	result := svc.Sync(client, "123", "pings", postings, panel)
	if result.Edited != 2 || len(result.Dropped) != 0 || len(result.Failed) != 0 {
		t.Fatalf("unexpected sync result: %+v", result)
	}
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits, got %d", len(edits))
	}
}

func TestRolePanelSyncDropsMissingPostings(t *testing.T) {
	t.Parallel()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "123"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	if err := cm.UpsertRolePanelButton("123", "pings", files.RolePanelButtonConfig{
		RoleID: "100",
		Label:  "Click",
	}); err != nil {
		t.Fatalf("seed button: %v", err)
	}
	const (
		goneMsg     = "300001"
		goneChannel = "300002"
		keep        = "300003"
	)
	for _, mid := range []string{goneMsg, goneChannel, keep} {
		if err := cm.AddRolePanelPosting("123", "pings", files.RolePanelPostingConfig{ChannelID: "111000", MessageID: mid}); err != nil {
			t.Fatalf("seed posting %s: %v", mid, err)
		}
	}

	svc := &RolePanelService{
		configManager: cm,
		editMessage: func(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID, data api.EditMessageData) error {
			switch messageID.String() {
			case goneMsg:
				return &httputil.HTTPError{Code: discordErrUnknownMessage}
			case goneChannel:
				return &httputil.HTTPError{Code: discordErrUnknownChannel}
			default:
				return nil
			}
		},
		dropPostings: func(c *files.ConfigManager, gid, k string, mid []string) error {
			return c.RemoveRolePanelPostings(gid, k, mid)
		},
	}

	postings, _ := cm.ListRolePanelPostings("123", "pings")
	panel := &files.RolePanelConfig{Title: "Pings"}

	client := &api.Client{}
	result := svc.Sync(client, "123", "pings", postings, panel)
	if result.Edited != 1 {
		t.Fatalf("expected 1 edited, got %d", result.Edited)
	}
	if len(result.Dropped) != 2 {
		t.Fatalf("expected 2 dropped, got %d", len(result.Dropped))
	}

	remaining, err := cm.ListRolePanelPostings("123", "pings")
	if err != nil {
		t.Fatalf("list remaining: %v", err)
	}
	if len(remaining) != 1 || remaining[0].MessageID != keep {
		t.Fatalf("expected only %s to remain, got %+v", keep, remaining)
	}
}
