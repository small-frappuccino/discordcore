package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func newTestCache(t *testing.T) *UnifiedCache {
	t.Helper()
	cache := NewUnifiedCache(CacheConfig{
		MemberTTL:       time.Hour,
		GuildTTL:        time.Hour,
		RolesTTL:        time.Hour,
		ChannelTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	t.Cleanup(cache.Stop)
	return cache
}

type staticWarmupSession struct {
	stateGuilds []*discordgo.Guild
	guilds      map[string]*discordgo.Guild
	roles       map[string][]*discordgo.Role
	channels    map[string][]*discordgo.Channel
	memberPages map[string][][]*discordgo.Member
	memberCalls map[string]int
}

func (s *staticWarmupSession) StateGuilds() []*discordgo.Guild {
	return s.stateGuilds
}

func (s *staticWarmupSession) Guild(id string) (*discordgo.Guild, error) {
	guild := s.guilds[id]
	if guild == nil {
		return nil, fmt.Errorf("missing guild %s", id)
	}
	return guild, nil
}

func (s *staticWarmupSession) GuildRoles(id string) ([]*discordgo.Role, error) {
	roles, ok := s.roles[id]
	if !ok {
		return nil, fmt.Errorf("missing roles %s", id)
	}
	return roles, nil
}

func (s *staticWarmupSession) GuildChannels(id string) ([]*discordgo.Channel, error) {
	channels, ok := s.channels[id]
	if !ok {
		return nil, fmt.Errorf("missing channels %s", id)
	}
	return channels, nil
}

func (s *staticWarmupSession) GuildMembers(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	if s.memberCalls == nil {
		s.memberCalls = make(map[string]int)
	}
	idx := s.memberCalls[guildID]
	s.memberCalls[guildID] = idx + 1
	pages := s.memberPages[guildID]
	if idx >= len(pages) {
		return nil, nil
	}
	return pages[idx], nil
}

func TestIntelligentWarmupPopulatesCache(t *testing.T) {
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
		t.Fatalf("IntelligentWarmup error: %v", err)
	}

	if got, ok := cache.GetGuild("g1"); !ok || got == nil || got.ID != "g1" {
		t.Fatalf("expected guild cached, got %v %v", got, ok)
	}
	if got, ok := cache.GetRoles("g1"); !ok || len(got) != 1 || got[0].ID != "r1" {
		t.Fatalf("expected roles cached, got %v %v", got, ok)
	}
	if got, ok := cache.GetChannel("c1"); !ok || got == nil || got.ID != "c1" {
		t.Fatalf("expected channel cached, got %v %v", got, ok)
	}
	if got, ok := cache.GetMember("g1", "u1"); !ok || got == nil || got.User.ID != "u1" {
		t.Fatalf("expected member cached, got %v %v", got, ok)
	}
	if got, ok := cache.GetMember("g1", "missing"); ok || got != nil {
		t.Fatalf("expected cache miss for unknown member, got %v %v", got, ok)
	}
}

type blockingWarmupSession struct {
	ready   chan struct{}
	release chan struct{}
	members []*discordgo.Member
}

func (s *blockingWarmupSession) StateGuilds() []*discordgo.Guild { return nil }
func (s *blockingWarmupSession) Guild(id string) (*discordgo.Guild, error) {
	return nil, fmt.Errorf("unexpected Guild(%s)", id)
}
func (s *blockingWarmupSession) GuildRoles(id string) ([]*discordgo.Role, error) {
	return nil, fmt.Errorf("unexpected GuildRoles(%s)", id)
}
func (s *blockingWarmupSession) GuildChannels(id string) ([]*discordgo.Channel, error) {
	return nil, fmt.Errorf("unexpected GuildChannels(%s)", id)
}
func (s *blockingWarmupSession) GuildMembers(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	s.ready <- struct{}{}
	<-s.release
	return s.members, nil
}

func TestWarmupGuildMembersConcurrentCalls(t *testing.T) {
	session := &blockingWarmupSession{
		ready:   make(chan struct{}, 2),
		release: make(chan struct{}),
		members: []*discordgo.Member{{User: &discordgo.User{ID: "u1"}}},
	}
	cache := newTestCache(t)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := warmupGuildMembers(session, cache, nil, "g1", 1)
			errs <- err
		}()
	}

	<-session.ready
	<-session.ready
	close(session.release)

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("warmupGuildMembers error: %v", err)
		}
	}
	if got, ok := cache.GetMember("g1", "u1"); !ok || got == nil || got.User.ID != "u1" {
		t.Fatalf("expected member cached after concurrent warmup, got %v %v", got, ok)
	}
}
