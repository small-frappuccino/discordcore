package roles

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type capturedEdit struct {
	Channel    string
	Message    string
	Components int
}

func newRolePanelSyncTestManager(t *testing.T) *files.ConfigManager {
	t.Helper()
	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "guild"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	if err := cm.UpsertRolePanelButton("guild", "pings", files.RolePanelButtonConfig{
		RoleID: "100",
		Label:  "Click",
	}); err != nil {
		t.Fatalf("seed button: %v", err)
	}
	return cm
}

func TestRolePanelSyncEditsEachPosting(t *testing.T) {
	cm := newRolePanelSyncTestManager(t)
	messageIDs := []string{"222000", "222001"}
	for _, mid := range messageIDs {
		if err := cm.AddRolePanelPosting("guild", "pings", files.RolePanelPostingConfig{ChannelID: "111000", MessageID: mid}); err != nil {
			t.Fatalf("seed posting %s: %v", mid, err)
		}
	}

	var mu sync.Mutex
	var edits []capturedEdit
	syncer := newRolePanelPostingSyncer(cm)
	syncer.editMessage = func(_ *discordgo.Session, edit *discordgo.MessageEdit) error {
		mu.Lock()
		defer mu.Unlock()
		var comps int
		if edit.Components != nil {
			comps = len(*edit.Components)
		}
		edits = append(edits, capturedEdit{Channel: edit.Channel, Message: edit.ID, Components: comps})
		return nil
	}

	postings, err := cm.ListRolePanelPostings("guild", "pings")
	if err != nil {
		t.Fatalf("list postings: %v", err)
	}
	embed := &discordgo.MessageEmbed{Title: "Pings"}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{}}

	result := syncer.Sync(rolePanelSyncRequest{GuildID: "guild", Key: "pings", Postings: postings, Embed: embed, Components: components})
	if result.Edited != 2 || len(result.Dropped) != 0 || len(result.Failed) != 0 {
		t.Fatalf("unexpected sync result: %+v", result)
	}
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits, got %d", len(edits))
	}
	got := make(map[string]capturedEdit, len(edits))
	for _, e := range edits {
		got[e.Message] = e
	}
	for _, mid := range messageIDs {
		e, ok := got[mid]
		if !ok {
			t.Fatalf("missing edit for message %s", mid)
		}
		if e.Channel != "111000" || e.Components != 1 {
			t.Fatalf("unexpected edit target for %s: %+v", mid, e)
		}
	}
}

func TestRolePanelSyncDropsMissingPostings(t *testing.T) {
	cm := newRolePanelSyncTestManager(t)
	const (
		goneMsg     = "300001"
		goneChannel = "300002"
		keep        = "300003"
	)
	for _, mid := range []string{goneMsg, goneChannel, keep} {
		if err := cm.AddRolePanelPosting("guild", "pings", files.RolePanelPostingConfig{ChannelID: "111000", MessageID: mid}); err != nil {
			t.Fatalf("seed posting %s: %v", mid, err)
		}
	}

	syncer := newRolePanelPostingSyncer(cm)
	syncer.editMessage = func(_ *discordgo.Session, edit *discordgo.MessageEdit) error {
		switch edit.ID {
		case goneMsg:
			return &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: discordErrUnknownMessage}}
		case goneChannel:
			return &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: discordErrUnknownChannel}}
		default:
			return nil
		}
	}

	postings, _ := cm.ListRolePanelPostings("guild", "pings")
	result := syncer.Sync(rolePanelSyncRequest{GuildID: "guild", Key: "pings", Postings: postings, Embed: &discordgo.MessageEmbed{}})
	if result.Edited != 1 {
		t.Fatalf("expected 1 edited, got %d", result.Edited)
	}
	if len(result.Dropped) != 2 {
		t.Fatalf("expected 2 dropped, got %d", len(result.Dropped))
	}
	if len(result.Failed) != 0 {
		t.Fatalf("expected 0 failed, got %d", len(result.Failed))
	}

	remaining, err := cm.ListRolePanelPostings("guild", "pings")
	if err != nil {
		t.Fatalf("list remaining: %v", err)
	}
	if len(remaining) != 1 || remaining[0].MessageID != keep {
		t.Fatalf("expected only %s to remain, got %+v", keep, remaining)
	}
}

func TestRolePanelSyncRecordsNonTerminalFailures(t *testing.T) {
	cm := newRolePanelSyncTestManager(t)
	const forbidden = "400000"
	if err := cm.AddRolePanelPosting("guild", "pings", files.RolePanelPostingConfig{ChannelID: "111000", MessageID: forbidden}); err != nil {
		t.Fatalf("seed posting: %v", err)
	}

	syncer := newRolePanelPostingSyncer(cm)
	syncer.editMessage = func(_ *discordgo.Session, _ *discordgo.MessageEdit) error {
		return &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: 50013, Message: "Missing Permissions"}}
	}
	syncer.dropPostings = func(_ *files.ConfigManager, _, _ string, _ []string) error {
		t.Fatal("dropPostings should not run for non-terminal failures")
		return nil
	}

	postings, _ := cm.ListRolePanelPostings("guild", "pings")
	result := syncer.Sync(rolePanelSyncRequest{GuildID: "guild", Key: "pings", Postings: postings, Embed: &discordgo.MessageEmbed{}})
	if result.Edited != 0 || len(result.Dropped) != 0 {
		t.Fatalf("unexpected counts in result: %+v", result)
	}
	if len(result.Failed) != 1 || result.Failed[0].Posting.MessageID != forbidden {
		t.Fatalf("expected one failure for %s, got %+v", forbidden, result.Failed)
	}

	// Posting must remain on file so the operator can retry.
	remaining, err := cm.ListRolePanelPostings("guild", "pings")
	if err != nil {
		t.Fatalf("list remaining: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected posting to stay on file after non-terminal failure, got %d", len(remaining))
	}
}

func TestFormatRolePanelSyncSummaryEdgeCases(t *testing.T) {
	t.Parallel()
	empty := rolePanelSyncResult{}
	if formatRolePanelSyncSummary(empty, "Refreshed") != "" {
		t.Fatalf("empty result should yield empty summary")
	}

	good := rolePanelSyncResult{Edited: 3}
	if got := formatRolePanelSyncSummary(good, "Refreshed"); !strings.Contains(got, "Refreshed 3 posting(s)") {
		t.Fatalf("unexpected good summary: %q", got)
	}

	mixed := rolePanelSyncResult{
		Edited:  1,
		Dropped: []files.RolePanelPostingConfig{{MessageID: "G1"}},
		Failed:  []rolePanelSyncFailure{{Posting: files.RolePanelPostingConfig{MessageID: "F1"}, Err: errors.New("boom")}},
	}
	got := formatRolePanelSyncSummary(mixed, "Stripped buttons from")
	for _, want := range []string{"Stripped buttons from 1 posting(s)", "Dropped 1 orphaned", "G1", "F1", "boom"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q: %s", want, got)
		}
	}
}

func TestIsRolePanelPostingMissingError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "plain error", err: errors.New("nope"), want: false},
		{name: "rest without message", err: &discordgo.RESTError{}, want: false},
		{name: "rest unknown channel", err: &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: discordErrUnknownChannel}}, want: true},
		{name: "rest unknown message", err: &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: discordErrUnknownMessage}}, want: true},
		{name: "rest other", err: &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: 50013}}, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isRolePanelPostingMissingError(tc.err); got != tc.want {
				t.Fatalf("isRolePanelPostingMissingError(%v) = %v want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestRolePanelPostingSyncer_BatchDrops(t *testing.T) {
	cm := newRolePanelSyncTestManager(t)
	const (
		goneMsg1 = "300001"
		goneMsg2 = "300002"
		keep     = "300003"
	)
	for _, mid := range []string{goneMsg1, goneMsg2, keep} {
		if err := cm.AddRolePanelPosting("guild", "pings", files.RolePanelPostingConfig{ChannelID: "111000", MessageID: mid}); err != nil {
			t.Fatalf("seed posting %s: %v", mid, err)
		}
	}

	syncer := newRolePanelPostingSyncer(cm)
	syncer.editMessage = func(_ *discordgo.Session, edit *discordgo.MessageEdit) error {
		switch edit.ID {
		case goneMsg1, goneMsg2:
			return &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: discordErrUnknownMessage}}
		default:
			return nil
		}
	}

	var dropCallCount int
	var droppedIDs []string
	syncer.dropPostings = func(_ *files.ConfigManager, guildID, key string, messageIDs []string) error {
		dropCallCount++
		droppedIDs = messageIDs
		return cm.RemoveRolePanelPostings(guildID, key, messageIDs)
	}

	postings, _ := cm.ListRolePanelPostings("guild", "pings")
	result := syncer.Sync(rolePanelSyncRequest{GuildID: "guild", Key: "pings", Postings: postings, Embed: &discordgo.MessageEmbed{}})

	if dropCallCount != 1 {
		t.Fatalf("expected exactly 1 call to dropPostings, got %d", dropCallCount)
	}
	if len(droppedIDs) != 2 {
		t.Fatalf("expected 2 dropped IDs in batch call, got %d", len(droppedIDs))
	}
	if result.Edited != 1 {
		t.Fatalf("expected 1 edited, got %d", result.Edited)
	}
	if len(result.Dropped) != 2 {
		t.Fatalf("expected 2 dropped, got %d", len(result.Dropped))
	}

	remaining, err := cm.ListRolePanelPostings("guild", "pings")
	if err != nil {
		t.Fatalf("list remaining: %v", err)
	}
	if len(remaining) != 1 || remaining[0].MessageID != keep {
		t.Fatalf("expected only %s to remain, got %+v", keep, remaining)
	}
}
