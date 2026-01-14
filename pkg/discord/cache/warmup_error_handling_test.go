package cache

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type funcWarmupSession struct {
	guildFunc    func(string) (*discordgo.Guild, error)
	rolesFunc    func(string) ([]*discordgo.Role, error)
	channelsFunc func(string) ([]*discordgo.Channel, error)
	membersFunc  func(string, string, int, ...discordgo.RequestOption) ([]*discordgo.Member, error)
}

func (s *funcWarmupSession) StateGuilds() []*discordgo.Guild { return nil }
func (s *funcWarmupSession) Guild(id string) (*discordgo.Guild, error) {
	if s.guildFunc == nil {
		return nil, errors.New("unexpected Guild")
	}
	return s.guildFunc(id)
}
func (s *funcWarmupSession) GuildRoles(id string) ([]*discordgo.Role, error) {
	if s.rolesFunc == nil {
		return nil, errors.New("unexpected GuildRoles")
	}
	return s.rolesFunc(id)
}
func (s *funcWarmupSession) GuildChannels(id string) ([]*discordgo.Channel, error) {
	if s.channelsFunc == nil {
		return nil, errors.New("unexpected GuildChannels")
	}
	return s.channelsFunc(id)
}
func (s *funcWarmupSession) GuildMembers(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	if s.membersFunc == nil {
		return nil, errors.New("unexpected GuildMembers")
	}
	return s.membersFunc(guildID, after, limit, options...)
}

func TestWarmupGuildErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := &funcWarmupSession{
		guildFunc: func(string) (*discordgo.Guild, error) { return nil, boom },
	}
	cache := newTestCache(t)

	err := warmupGuild(session, cache, "g1")
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestWarmupGuildRolesErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := &funcWarmupSession{
		rolesFunc: func(string) ([]*discordgo.Role, error) { return nil, boom },
	}
	cache := newTestCache(t)

	_, err := warmupGuildRoles(session, cache, nil, "g1")
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestWarmupGuildChannelsErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := &funcWarmupSession{
		channelsFunc: func(string) ([]*discordgo.Channel, error) { return nil, boom },
	}
	cache := newTestCache(t)

	_, err := warmupGuildChannels(session, cache, "g1")
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestWarmupGuildMembersErrorPropagation(t *testing.T) {
	boom := errors.New("boom")
	session := &funcWarmupSession{
		membersFunc: func(string, string, int, ...discordgo.RequestOption) ([]*discordgo.Member, error) {
			return nil, boom
		},
	}
	cache := newTestCache(t)

	_, err := warmupGuildMembers(session, cache, nil, "g1", 1)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
