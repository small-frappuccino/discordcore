package cache

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// funcWarmupSession replaced with warmupSession inline closures

func TestWarmupGuildErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := warmupSession{
		Guild: func(id string, options ...discordgo.RequestOption) (*discordgo.Guild, error) { return nil, boom },
	}
	cache := newTestCache(t)

	err := warmupGuild(session, cache, "g1")
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestWarmupGuildRolesErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := warmupSession{
		GuildRoles: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Role, error) { return nil, boom },
	}
	cache := newTestCache(t)

	_, err := warmupGuildRoles(session, cache, nil, "g1")
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestWarmupGuildChannelsErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := warmupSession{
		GuildChannels: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) { return nil, boom },
	}
	cache := newTestCache(t)

	_, err := warmupGuildChannels(session, cache, "g1")
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestWarmupGuildMembersErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := warmupSession{
		GuildMembers: func(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
			return nil, boom
		},
	}
	cache := newTestCache(t)

	_, err := warmupGuildMembers(session, cache, nil, "g1", 1)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
