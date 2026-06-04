package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestIntelligentWarmupIdempotent(t *testing.T) {
	guild := &discordgo.Guild{ID: "g1", Name: "Guild One"}
	role := &discordgo.Role{ID: "r1", Name: "role"}
	channel := &discordgo.Channel{ID: "c1", GuildID: "g1", Name: "general"}
	member := &discordgo.Member{User: &discordgo.User{ID: "u1"}, JoinedAt: time.Now().UTC()}

	var memberCalls int
	session := warmupSession{
		StateGuilds: func() []*discordgo.Guild { return []*discordgo.Guild{guild} },
		Guild: func(id string, options ...discordgo.RequestOption) (*discordgo.Guild, error) {
			if id == "g1" {
				return guild, nil
			}
			return nil, fmt.Errorf("missing guild %s", id)
		},
		GuildRoles: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Role, error) {
			if id == "g1" {
				return []*discordgo.Role{role}, nil
			}
			return nil, fmt.Errorf("missing roles %s", id)
		},
		GuildChannels: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) {
			if id == "g1" {
				return []*discordgo.Channel{channel}, nil
			}
			return nil, fmt.Errorf("missing channels %s", id)
		},
		GuildMembers: func(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
			idx := memberCalls
			memberCalls++
			if idx == 0 {
				return []*discordgo.Member{member}, nil
			}
			return nil, nil
		},
	}

	old := newWarmupSession
	newWarmupSession = func(_ *discordgo.Session) warmupSession { return session }
	t.Cleanup(func() { newWarmupSession = old })

	cache := newTestCache(t)
	cfg := WarmupConfig{
		FetchMissingMembers:  true,
		FetchMissingRoles:    true,
		FetchMissingGuilds:   true,
		FetchMissingChannels: true,
		MaxMembersPerGuild:   1,
	}

	if err := IntelligentWarmup(&discordgo.Session{}, cache, nil, cfg); err != nil {
		t.Fatalf("first warmup error: %v", err)
	}
	firstMembers := cache.MemberCount()
	firstGuilds := cache.GuildCount()
	firstRoles := cache.RolesCount()
	firstChannels := cache.ChannelCount()

	memberCalls = 0
	if err := IntelligentWarmup(&discordgo.Session{}, cache, nil, cfg); err != nil {
		t.Fatalf("second warmup error: %v", err)
	}
	secondMembers := cache.MemberCount()
	secondGuilds := cache.GuildCount()
	secondRoles := cache.RolesCount()
	secondChannels := cache.ChannelCount()

	if firstMembers != secondMembers || firstGuilds != secondGuilds || firstRoles != secondRoles || firstChannels != secondChannels {
		t.Fatalf("expected idempotent metrics, got first=%v/%v/%v/%v second=%v/%v/%v/%v",
			firstMembers, firstGuilds, firstRoles, firstChannels,
			secondMembers, secondGuilds, secondRoles, secondChannels)
	}
}
