package control

import (
	"errors"
	"strings"
)

type fakeDiscordService struct {
	guilds  map[string]*Guild
	intents int
}

func newFakeDiscordService() *fakeDiscordService {
	return &fakeDiscordService{
		guilds:  make(map[string]*Guild),
		intents: -1, // -1 means all intents by default
	}
}

func (f *fakeDiscordService) addGuild(g *Guild) {
	f.guilds[g.ID] = g
}

func (f *fakeDiscordService) Guild(guildID string) (*Guild, error) {
	g, ok := f.guilds[guildID]
	if !ok {
		return nil, errors.New("guild not found")
	}
	return g, nil
}

func (f *fakeDiscordService) GuildMember(guildID, userID string) (*Member, error) {
	g, ok := f.guilds[guildID]
	if !ok {
		return nil, errors.New("guild not found")
	}
	for _, m := range g.Members {
		if m.User != nil && m.User.ID == userID {
			return m, nil
		}
	}
	return nil, errors.New("member not found")
}

func (f *fakeDiscordService) GuildMembers(guildID, after string, limit int) ([]*Member, error) {
	// Not needed for most tests right now, but easy to mock
	return nil, errors.New("not implemented")
}

func (f *fakeDiscordService) GuildMembersSearch(guildID, query string, limit int) ([]*Member, error) {
	g, ok := f.guilds[guildID]
	if !ok {
		return nil, errors.New("guild not found")
	}
	var res []*Member
	q := strings.ToLower(query)
	for _, m := range g.Members {
		if m.User != nil && (strings.Contains(strings.ToLower(m.User.Username), q) || strings.Contains(strings.ToLower(m.User.GlobalName), q)) || strings.Contains(strings.ToLower(m.Nick), q) {
			res = append(res, m)
			if len(res) == limit {
				break
			}
		}
	}
	return res, nil
}

func (f *fakeDiscordService) HasIntent(intentMask int) bool {
	if f.intents == -1 {
		return true
	}
	return f.intents&intentMask == intentMask
}

func (f *fakeDiscordService) BotUser() (*User, error) {
	return &User{Username: "TestBot"}, nil
}
