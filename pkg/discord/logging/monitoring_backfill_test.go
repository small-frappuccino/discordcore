package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestParseEntryExitBackfillMessage_MimuWelcome(t *testing.T) {
	m := &discordgo.Message{
		Content: "<@1234567890> Welcome to Alice Mains!",
	}
	gotEvt, gotUserID, ok := parseEntryExitBackfillMessage(m, "")
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
	gotEvt, gotUserID, ok := parseEntryExitBackfillMessage(m, "")
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
	gotEvt, gotUserID, ok := parseEntryExitBackfillMessage(m, "42")
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

func TestParseEntryExitBackfillMessage_IgnoresNonMatching(t *testing.T) {
	m := &discordgo.Message{Content: "hello world"}
	_, _, ok := parseEntryExitBackfillMessage(m, "")
	if ok {
		t.Fatalf("expected ok=false")
	}
}
