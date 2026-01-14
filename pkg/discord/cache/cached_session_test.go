package cache

import (
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestCachedSessionGuildMemberUsesStateAndCaches(t *testing.T) {
	session := &discordgo.Session{State: discordgo.NewState()}
	member := &discordgo.Member{User: &discordgo.User{ID: "user"}}
	guild := &discordgo.Guild{ID: "guild", Members: []*discordgo.Member{member}}
	if err := session.State.GuildAdd(guild); err != nil {
		t.Fatalf("guild add: %v", err)
	}

	cache := NewUnifiedCache(CacheConfig{MemberTTL: time.Minute, GuildTTL: time.Minute, RolesTTL: time.Minute, ChannelTTL: time.Minute, CleanupInterval: time.Hour})
	cs := NewCachedSession(session, cache)

	got, err := cs.GuildMember("guild", "user")
	if err != nil {
		t.Fatalf("GuildMember returned error: %v", err)
	}
	if got == nil || got.User.ID != "user" {
		t.Fatalf("unexpected member: %+v", got)
	}
	if cached, ok := cache.GetMember("guild", "user"); !ok || cached.User.ID != "user" {
		t.Fatalf("expected member cached after state hit")
	}
}

func TestCachedSessionChannelUsesStateFallbackOrder(t *testing.T) {
	session := &discordgo.Session{State: discordgo.NewState()}
	ch := &discordgo.Channel{ID: "chan", GuildID: "g"}
	_ = session.State.ChannelAdd(ch)

	cache := NewUnifiedCache(CacheConfig{MemberTTL: time.Minute, GuildTTL: time.Minute, RolesTTL: time.Minute, ChannelTTL: time.Minute, CleanupInterval: time.Hour})
	// Prime cache to avoid hitting REST.
	cache.SetChannel("chan", ch)
	cs := NewCachedSession(session, cache)

	got, err := cs.Channel("chan")
	if err != nil {
		t.Fatalf("Channel returned error: %v", err)
	}
	if got == nil || got.ID != "chan" {
		t.Fatalf("unexpected channel: %+v", got)
	}
	if cached, ok := cache.GetChannel("chan"); !ok || cached.ID != "chan" {
		t.Fatalf("expected channel cached after state hit")
	}
}
