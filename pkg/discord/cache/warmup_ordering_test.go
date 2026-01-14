package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

type recordingWarmupSession struct {
	mu          sync.Mutex
	calls       []string
	stateCalls  int
	guilds      map[string]*discordgo.Guild
	roles       map[string][]*discordgo.Role
	channels    map[string][]*discordgo.Channel
	members     map[string][]*discordgo.Member
	stateGuilds []*discordgo.Guild
}

func (s *recordingWarmupSession) record(call string) {
	s.mu.Lock()
	s.calls = append(s.calls, call)
	s.mu.Unlock()
}

func (s *recordingWarmupSession) StateGuilds() []*discordgo.Guild {
	s.mu.Lock()
	s.stateCalls++
	s.mu.Unlock()
	return s.stateGuilds
}

func (s *recordingWarmupSession) Guild(id string) (*discordgo.Guild, error) {
	s.record("guild:" + id)
	guild := s.guilds[id]
	if guild == nil {
		return nil, fmt.Errorf("missing guild %s", id)
	}
	return guild, nil
}

func (s *recordingWarmupSession) GuildRoles(id string) ([]*discordgo.Role, error) {
	s.record("roles:" + id)
	roles, ok := s.roles[id]
	if !ok {
		return nil, fmt.Errorf("missing roles %s", id)
	}
	return roles, nil
}

func (s *recordingWarmupSession) GuildChannels(id string) ([]*discordgo.Channel, error) {
	s.record("channels:" + id)
	channels, ok := s.channels[id]
	if !ok {
		return nil, fmt.Errorf("missing channels %s", id)
	}
	return channels, nil
}

func (s *recordingWarmupSession) GuildMembers(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	s.record("members:" + guildID)
	members, ok := s.members[guildID]
	if !ok {
		return nil, fmt.Errorf("missing members %s", guildID)
	}
	return members, nil
}

func TestIntelligentWarmupOrdering(t *testing.T) {
	g1 := &discordgo.Guild{ID: "g1", Name: "Guild One"}
	g2 := &discordgo.Guild{ID: "g2", Name: "Guild Two"}
	r1 := &discordgo.Role{ID: "r1", Name: "role"}
	c1 := &discordgo.Channel{ID: "c1", GuildID: "g1", Name: "chan"}
	m1 := &discordgo.Member{User: &discordgo.User{ID: "u1"}, JoinedAt: time.Now().UTC()}

	session := &recordingWarmupSession{
		guilds: map[string]*discordgo.Guild{
			"g1": g1,
			"g2": g2,
		},
		roles: map[string][]*discordgo.Role{
			"g1": {r1},
			"g2": {r1},
		},
		channels: map[string][]*discordgo.Channel{
			"g1": {c1},
			"g2": {c1},
		},
		members: map[string][]*discordgo.Member{
			"g1": {m1},
			"g2": {m1},
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
		GuildIDs:             []string{"g1", "g2"},
	}

	if err := IntelligentWarmup(&discordgo.Session{}, cache, nil, cfg); err != nil {
		t.Fatalf("IntelligentWarmup error: %v", err)
	}

	expected := []string{
		"guild:g1",
		"roles:g1",
		"channels:g1",
		"members:g1",
		"guild:g2",
		"roles:g2",
		"channels:g2",
		"members:g2",
	}

	session.mu.Lock()
	got := append([]string(nil), session.calls...)
	stateCalls := session.stateCalls
	session.mu.Unlock()

	if stateCalls != 0 {
		t.Fatalf("expected StateGuilds not to be called, got %d", stateCalls)
	}
	if len(got) != len(expected) {
		t.Fatalf("unexpected call count: got %d want %d (%v)", len(got), len(expected), got)
	}
	for i, want := range expected {
		if got[i] != want {
			t.Fatalf("call order mismatch at %d: got %q want %q", i, got[i], want)
		}
	}
}
