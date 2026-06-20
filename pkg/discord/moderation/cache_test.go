package moderation

import (
	"context"
	"errors"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
)

type mockCacheResolver struct {
	apiHits int
}

func (m *mockCacheResolver) Member(guildID discord.GuildID, userID discord.UserID) (*discord.Member, error) {
	// Simulate cache miss
	return nil, errors.New("not found in cache")
}

func (m *mockCacheResolver) MemberFromAPI(guildID discord.GuildID, userID discord.UserID) (*discord.Member, error) {
	m.apiHits++
	return &discord.Member{User: discord.User{ID: userID}}, nil
}

// TestFallbackCache_ResolveMember validates that cache misses trigger immediate
// secondary REST calls.
func TestFallbackCache_ResolveMember(t *testing.T) {
	mockState := &mockCacheResolver{}
	cache := NewFallbackCache(mockState)

	ctx := context.Background()
	guildID := discord.GuildID(123)
	userID := discord.UserID(456)

	member, err := cache.ResolveMember(ctx, guildID, userID)
	if err != nil {
		t.Fatalf("expected successful fallback, got error: %v", err)
	}

	if member == nil || member.User.ID != userID {
		t.Fatalf("resolved member does not match requested ID")
	}

	if mockState.apiHits != 1 {
		t.Errorf("expected 1 API hit due to cache miss, got %d", mockState.apiHits)
	}
}
