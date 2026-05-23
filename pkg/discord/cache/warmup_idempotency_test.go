package cache

import (
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestIntelligentWarmupIdempotent(t *testing.T) {
	guild := &discordgo.Guild{ID: "g1", Name: "Guild One"}
	role := &discordgo.Role{ID: "r1", Name: "role"}
	channel := &discordgo.Channel{ID: "c1", GuildID: "g1", Name: "general"}
	member := &discordgo.Member{User: &discordgo.User{ID: "u1"}, JoinedAt: time.Now().UTC()}

	session := &staticWarmupSession{
		stateGuilds: []*discordgo.Guild{guild},
		guilds:      map[string]*discordgo.Guild{"g1": guild},
		roles:       map[string][]*discordgo.Role{"g1": {role}},
		channels:    map[string][]*discordgo.Channel{"g1": {channel}},
		memberPages: map[string][][]*discordgo.Member{"g1": {{member}}},
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

	session.memberCalls = make(map[string]int)
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
