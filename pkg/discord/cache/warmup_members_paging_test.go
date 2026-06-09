package cache_test

import (
	"fmt"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type memberCall struct {
	after string
	limit int
}

// pagingWarmupSession replaced with inline closures

func TestWarmupGuildMembersPagingUsesAfterID(t *testing.T) {
	firstPage := make([]*discordgo.Member, 1000)
	for i := 0; i < len(firstPage); i++ {
		firstPage[i] = &discordgo.Member{User: &discordgo.User{ID: fmt.Sprintf("u%04d", i+1)}}
	}
	secondPage := []*discordgo.Member{
		{User: &discordgo.User{ID: "u1001"}},
	}

	var mu sync.Mutex
	var calls []memberCall
	pages := [][]*discordgo.Member{
		firstPage,
		secondPage,
	}

	session := cache.WarmupSession{
		GuildMembers: func(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
			mu.Lock()
			idx := len(calls)
			calls = append(calls, memberCall{after: after, limit: limit})
			mu.Unlock()

			if idx >= len(pages) {
				return nil, nil
			}
			return pages[idx], nil
		},
	}
	uc := newTestCache(t)

	gotCount, err := cache.WarmupGuildMembers(session, uc, nil, "g1", 0)
	if err != nil {
		t.Fatalf("cache.WarmupGuildMembers error: %v", err)
	}
	if gotCount != 1001 {
		t.Fatalf("expected 1001 cached members, got %d", gotCount)
	}
	if _, ok := uc.GetMember("g1", "u0001"); !ok {
		t.Fatalf("expected u0001 cached")
	}
	if _, ok := uc.GetMember("g1", "u1000"); !ok {
		t.Fatalf("expected u1000 cached")
	}
	if _, ok := uc.GetMember("g1", "u1001"); !ok {
		t.Fatalf("expected u1001 cached")
	}

	mu.Lock()
	gotCalls := append([]memberCall(nil), calls...)
	mu.Unlock()

	if len(gotCalls) != 2 {
		t.Fatalf("expected 2 paging calls, got %d", len(gotCalls))
	}
	if gotCalls[0].after != "" || gotCalls[0].limit != 1000 {
		t.Fatalf("unexpected first call: %+v", gotCalls[0])
	}
	if gotCalls[1].after != "u1000" || gotCalls[1].limit != 1000 {
		t.Fatalf("unexpected second call: %+v", gotCalls[1])
	}
}
