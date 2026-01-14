package cache

import (
	"fmt"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type memberCall struct {
	after string
	limit int
}

type pagingWarmupSession struct {
	mu     sync.Mutex
	calls  []memberCall
	pages  [][]*discordgo.Member
	labels []string
}

func (s *pagingWarmupSession) StateGuilds() []*discordgo.Guild { return nil }
func (s *pagingWarmupSession) Guild(id string) (*discordgo.Guild, error) {
	return nil, fmt.Errorf("unexpected Guild(%s)", id)
}
func (s *pagingWarmupSession) GuildRoles(id string) ([]*discordgo.Role, error) {
	return nil, fmt.Errorf("unexpected GuildRoles(%s)", id)
}
func (s *pagingWarmupSession) GuildChannels(id string) ([]*discordgo.Channel, error) {
	return nil, fmt.Errorf("unexpected GuildChannels(%s)", id)
}

func (s *pagingWarmupSession) GuildMembers(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	s.mu.Lock()
	idx := len(s.calls)
	s.calls = append(s.calls, memberCall{after: after, limit: limit})
	s.mu.Unlock()

	if idx >= len(s.pages) {
		return nil, nil
	}
	return s.pages[idx], nil
}

func TestWarmupGuildMembersPagingUsesAfterID(t *testing.T) {
	firstPage := make([]*discordgo.Member, 1000)
	for i := 0; i < len(firstPage); i++ {
		firstPage[i] = &discordgo.Member{User: &discordgo.User{ID: fmt.Sprintf("u%04d", i+1)}}
	}
	secondPage := []*discordgo.Member{
		{User: &discordgo.User{ID: "u1001"}},
	}

	session := &pagingWarmupSession{
		pages: [][]*discordgo.Member{
			firstPage,
			secondPage,
		},
	}
	cache := newTestCache(t)

	gotCount, err := warmupGuildMembers(session, cache, nil, "g1", 0)
	if err != nil {
		t.Fatalf("warmupGuildMembers error: %v", err)
	}
	if gotCount != 1001 {
		t.Fatalf("expected 1001 cached members, got %d", gotCount)
	}
	if _, ok := cache.GetMember("g1", "u0001"); !ok {
		t.Fatalf("expected u0001 cached")
	}
	if _, ok := cache.GetMember("g1", "u1000"); !ok {
		t.Fatalf("expected u1000 cached")
	}
	if _, ok := cache.GetMember("g1", "u1001"); !ok {
		t.Fatalf("expected u1001 cached")
	}

	session.mu.Lock()
	calls := append([]memberCall(nil), session.calls...)
	session.mu.Unlock()

	if len(calls) != 2 {
		t.Fatalf("expected 2 paging calls, got %d", len(calls))
	}
	if calls[0].after != "" || calls[0].limit != 1000 {
		t.Fatalf("unexpected first call: %+v", calls[0])
	}
	if calls[1].after != "u1000" || calls[1].limit != 1000 {
		t.Fatalf("unexpected second call: %+v", calls[1])
	}
}
