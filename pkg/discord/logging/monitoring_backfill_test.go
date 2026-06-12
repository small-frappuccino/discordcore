package logging

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestParseEntryExitBackfillMessage_MimuWelcome(t *testing.T) {
	m := &discordgo.Message{
		Content: "<@1234567890> Welcome to Alice Mains!",
	}
	gotEvt, gotUserID, ok := parseEntryExitBackfillMessage(m, "", files.RuntimeConfig{})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if gotEvt != "join" {
		t.Fatalf("expected evt=join, got %q", gotEvt)
	}
	if gotUserID != "1234567890" {
		t.Fatalf("expected userID=1234567890, got %q", gotUserID)
	}
}

func TestParseEntryExitBackfillMessage_MimuGoodbye(t *testing.T) {
	m := &discordgo.Message{
		Content: "<@!987654321> goodbye!",
	}
	gotEvt, gotUserID, ok := parseEntryExitBackfillMessage(m, "", files.RuntimeConfig{})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if gotEvt != "leave" {
		t.Fatalf("expected evt=leave, got %q", gotEvt)
	}
	if gotUserID != "987654321" {
		t.Fatalf("expected userID=987654321, got %q", gotUserID)
	}
}

func TestParseEntryExitBackfillMessage_EmbedJoin_ByBot(t *testing.T) {
	m := &discordgo.Message{
		Author: &discordgo.User{ID: "42"},
		Embeds: []*discordgo.MessageEmbed{
			{Title: "Member Joined", Description: "**u** (<@123>, `123`)"},
		},
	}
	gotEvt, gotUserID, ok := parseEntryExitBackfillMessage(m, "42", files.RuntimeConfig{})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if gotEvt != "join" {
		t.Fatalf("expected evt=join, got %q", gotEvt)
	}
	if gotUserID != "123" {
		t.Fatalf("expected userID=123, got %q", gotUserID)
	}
}

func TestParseEntryExitBackfillMessage_NewFormats(t *testing.T) {
	t.Run("welcome to alice mains! @user", func(t *testing.T) {
		m := &discordgo.Message{
			Content: "welcome to alice mains! <@1234567890>",
		}
		gotEvt, gotUserID, ok := parseEntryExitBackfillMessage(m, "", files.RuntimeConfig{})
		if !ok {
			t.Fatalf("expected ok=true")
		}
		if gotEvt != "join" {
			t.Fatalf("expected evt=join, got %q", gotEvt)
		}
		if gotUserID != "1234567890" {
			t.Fatalf("expected userID=1234567890, got %q", gotUserID)
		}
	})

	t.Run("@user has left the server... :(", func(t *testing.T) {
		m := &discordgo.Message{
			Content: "<@987654321> has left the server... :(",
		}
		gotEvt, gotUserID, ok := parseEntryExitBackfillMessage(m, "", files.RuntimeConfig{})
		if !ok {
			t.Fatalf("expected ok=true")
		}
		if gotEvt != "leave" {
			t.Fatalf("expected evt=leave, got %q", gotEvt)
		}
		if gotUserID != "987654321" {
			t.Fatalf("expected userID=987654321, got %q", gotUserID)
		}
	})
}

func TestParseEntryExitBackfillMessage_IgnoresNonMatching(t *testing.T) {
	m := &discordgo.Message{Content: "hello world"}
	_, _, ok := parseEntryExitBackfillMessage(m, "", files.RuntimeConfig{})
	if ok {
		t.Fatalf("expected ok=false")
	}
}

// TestApplyBackfillPage_WindowsAndStopsAtStart verifies that a page ordered
// newest-to-oldest counts messages at/after the window end, processes messages inside
// [start, end), and halts mid-page (reachedStart) the moment it sees a message older
// than start, leaving trailing messages untouched.
func TestApplyBackfillPage_WindowsAndStopsAtStart(t *testing.T) {
	ms := &MonitoringService{}
	start := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC)

	msgs := []*discordgo.Message{
		{ID: "1", Timestamp: time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC)},  // at/after end: counted, not persisted
		{ID: "2", Timestamp: time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)}, // in window: counted
		{ID: "3", Timestamp: time.Date(2024, 1, 10, 1, 0, 0, 0, time.UTC)},  // in window: counted
		{ID: "4", Timestamp: time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC)},   // before start: stop here
		{ID: "5", Timestamp: time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)},   // never reached
	}

	res, err := ms.applyBackfillPage(context.Background(), backfillScope{GuildID: "g", ChannelID: "c", BotID: "bot", Mode: "range"}, msgs, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.reachedStart {
		t.Fatalf("expected reachedStart=true once a message older than start is seen")
	}
	if res.processed != 3 {
		t.Fatalf("expected processed=3 (messages 1-3, message 5 never reached), got %d", res.processed)
	}
	if res.eventsFound != 0 {
		t.Fatalf("expected eventsFound=0 with nil store, got %d", res.eventsFound)
	}
}

// TestApplyBackfillPage_NoStartReachedProcessesAll verifies that when no message is
// older than start, the whole page is counted and reachedStart stays false so the
// caller keeps paging.
func TestApplyBackfillPage_NoStartReachedProcessesAll(t *testing.T) {
	ms := &MonitoringService{}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	msgs := []*discordgo.Message{
		{ID: "1", Timestamp: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "2", Timestamp: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)},
	}

	res, err := ms.applyBackfillPage(context.Background(), backfillScope{GuildID: "g", ChannelID: "c", BotID: "bot", Mode: "day"}, msgs, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.reachedStart {
		t.Fatalf("expected reachedStart=false when no message predates start")
	}
	if res.processed != 2 {
		t.Fatalf("expected processed=2, got %d", res.processed)
	}
}

// TestApplyBackfillPage_CanceledContextReturnsError verifies the per-message context
// check aborts the page and surfaces a wrapped error.
func TestApplyBackfillPage_CanceledContextReturnsError(t *testing.T) {
	ms := &MonitoringService{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msgs := []*discordgo.Message{
		{ID: "1", Timestamp: time.Now().UTC()},
	}

	if _, err := ms.applyBackfillPage(ctx, backfillScope{GuildID: "g", ChannelID: "c", BotID: "bot", Mode: "day"}, msgs, time.Time{}, time.Now().Add(time.Hour)); err == nil {
		t.Fatalf("expected error from canceled context")
	}
}
