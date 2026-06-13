package control

import (
	"testing"
)

func TestServerDiscordSessionForGuildUsesGuildRegistrationResolver(t *testing.T) {
	t.Parallel()

	server := &Server{}
	wantSession := newFakeDiscordService()
	var gotGuildID string

	server.SetDiscordServiceResolver(func(guildID string) (DiscordService, error) {
		gotGuildID = guildID
		return wantSession, nil
	})

	got, err := server.discordServiceForGuild("guild-1")
	if err != nil {
		t.Fatalf("discordServiceForGuild() error = %v", err)
	}
	if got != wantSession {
		t.Fatalf("discordServiceForGuild() session = %p, want %p", got, wantSession)
	}
	if gotGuildID != "guild-1" {
		t.Fatalf("resolver called with guild=%q, want guild-1", gotGuildID)
	}
}
