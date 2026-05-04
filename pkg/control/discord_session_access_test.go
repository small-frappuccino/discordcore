package control

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestServerDiscordSessionForGuildDomainUsesDomainResolver(t *testing.T) {
	t.Parallel()

	server := &Server{}
	wantSession := &discordgo.Session{}
	var gotGuildID string
	var gotDomain string

	server.SetDiscordSessionResolverForDomain(func(guildID, domain string) (*discordgo.Session, error) {
		gotGuildID = guildID
		gotDomain = domain
		return wantSession, nil
	})

	got, err := server.discordSessionForGuildDomain("guild-1", files.BotDomainQOTD)
	if err != nil {
		t.Fatalf("discordSessionForGuildDomain() error = %v", err)
	}
	if got != wantSession {
		t.Fatalf("discordSessionForGuildDomain() session = %p, want %p", got, wantSession)
	}
	if gotGuildID != "guild-1" || gotDomain != files.BotDomainQOTD {
		t.Fatalf("resolver called with guild=%q domain=%q, want guild-1/qotd", gotGuildID, gotDomain)
	}

	got, err = server.discordSessionForGuild("guild-1")
	if err != nil {
		t.Fatalf("discordSessionForGuild() error = %v", err)
	}
	if got != wantSession {
		t.Fatalf("discordSessionForGuild() session = %p, want %p", got, wantSession)
	}
	if gotGuildID != "guild-1" || gotDomain != "" {
		t.Fatalf("legacy wrapper called with guild=%q domain=%q, want guild-1/empty", gotGuildID, gotDomain)
	}
}

func TestServerDiscordSessionResolverLegacySetterWrapsDomainResolver(t *testing.T) {
	t.Parallel()

	server := &Server{}
	wantSession := &discordgo.Session{}
	var gotGuildID string

	server.SetDiscordSessionResolver(func(guildID string) (*discordgo.Session, error) {
		gotGuildID = guildID
		return wantSession, nil
	})

	got, err := server.discordSessionForGuildDomain("guild-2", files.BotDomainQOTD)
	if err != nil {
		t.Fatalf("discordSessionForGuildDomain() error = %v", err)
	}
	if got != wantSession {
		t.Fatalf("discordSessionForGuildDomain() session = %p, want %p", got, wantSession)
	}
	if gotGuildID != "guild-2" {
		t.Fatalf("legacy resolver called with guild=%q, want guild-2", gotGuildID)
	}
}