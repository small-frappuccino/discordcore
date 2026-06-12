package cache_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"

	"github.com/small-frappuccino/discordgo"
)

func TestIntelligentWarmupIdempotent(t *testing.T) {
	guild := &discordgo.Guild{ID: "g1", Name: "Guild One"}
	role := &discordgo.Role{ID: "r1", Name: "role"}
	channel := &discordgo.Channel{ID: "c1", GuildID: "g1", Name: "general"}
	member := &discordgo.Member{User: &discordgo.User{ID: "u1"}, JoinedAt: time.Now().UTC()}

	var memberCalls int
	session := cache.WarmupSession{
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

	old := cache.NewWarmupSession
	cache.NewWarmupSession = func(_ *discordgo.Session) cache.WarmupSession { return session }
	t.Cleanup(func() { cache.NewWarmupSession = old })

	uc := newTestCache(t)
	cfg := cache.WarmupConfig{
		FetchMissingMembers:  true,
		FetchMissingRoles:    true,
		FetchMissingGuilds:   true,
		FetchMissingChannels: true,
		MaxMembersPerGuild:   1,
	}

	if err := cache.IntelligentWarmup(&discordgo.Session{}, uc, nil, cfg); err != nil {
		t.Fatalf("first warmup error: %v", err)
	}
	firstMembers := uc.MemberCount()
	firstGuilds := uc.GuildCount()
	firstRoles := uc.RolesCount()
	firstChannels := uc.ChannelCount()

	memberCalls = 0
	if err := cache.IntelligentWarmup(&discordgo.Session{}, uc, nil, cfg); err != nil {
		t.Fatalf("second warmup error: %v", err)
	}
	secondMembers := uc.MemberCount()
	secondGuilds := uc.GuildCount()
	secondRoles := uc.RolesCount()
	secondChannels := uc.ChannelCount()

	if firstMembers != secondMembers || firstGuilds != secondGuilds || firstRoles != secondRoles || firstChannels != secondChannels {
		t.Fatalf("expected idempotent metrics, got first=%v/%v/%v/%v second=%v/%v/%v/%v",
			firstMembers, firstGuilds, firstRoles, firstChannels,
			secondMembers, secondGuilds, secondRoles, secondChannels)
	}
}
