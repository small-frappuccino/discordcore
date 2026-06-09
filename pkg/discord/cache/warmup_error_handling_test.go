package cache_test

import (
	"errors"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// funcWarmupSession replaced with cache.WarmupSession inline closures

func TestWarmupGuildReturnsErrorOnFailure(t *testing.T) {
	session := cache.WarmupSession{
		Guild: func(id string, options ...discordgo.RequestOption) (*discordgo.Guild, error) {
			return nil, errors.New("network error")
		},
	}
	err := cache.WarmupGuild(session, newTestCache(t), "g1")
	if err == nil {
		t.Fatalf("expected error from warmupGuild on API failure")
	}
}

func TestWarmupGuildRolesReturnsErrorOnFailure(t *testing.T) {
	session := cache.WarmupSession{
		GuildRoles: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Role, error) {
			return nil, errors.New("network error")
		},
	}
	uc := newTestCache(t)

	_, err := cache.WarmupGuildRoles(session, uc, nil, "g1")
	if err == nil {
		t.Fatalf("expected error from warmupGuildRoles on API failure")
	}
}

func TestWarmupGuildChannelsErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := cache.WarmupSession{
		GuildChannels: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) { return nil, boom },
	}
	uc := newTestCache(t)

	_, err := cache.WarmupGuildChannels(session, uc, "g1")
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestWarmupGuildMembersErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := cache.WarmupSession{
		GuildMembers: func(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
			return nil, boom
		},
	}
	uc := newTestCache(t)

	_, err := cache.WarmupGuildMembers(session, uc, nil, "g1", 1)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
