package control

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestServerDiscordSessionForGuildUsesGuildRegistrationResolver(t *testing.T) {
	t.Parallel()

	server := &Server{}
	wantSession := &discordgo.Session{}
	var gotGuildID string

	server.SetDiscordSessionResolver(func(guildID string) (*discordgo.Session, error) {
		gotGuildID = guildID
		return wantSession, nil
	})

	got, err := server.discordSessionForGuild("guild-1")
	if err != nil {
		t.Fatalf("discordSessionForGuild() error = %v", err)
	}
	if got != wantSession {
		t.Fatalf("discordSessionForGuild() session = %p, want %p", got, wantSession)
	}
	if gotGuildID != "guild-1" {
		t.Fatalf("resolver called with guild=%q, want guild-1", gotGuildID)
	}
}
