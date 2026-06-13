package stats

import (
	"context"
	"iter"
)

type mockPublisher struct {
	channels map[string]Channel
	members  map[string][]GuildMember
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{
		channels: make(map[string]Channel),
		members:  make(map[string][]GuildMember),
	}
}

func (m *mockPublisher) ChannelEditName(ctx context.Context, channelID, newName string) error {
	ch := m.channels[channelID]
	ch.Name = newName
	m.channels[channelID] = ch
	return nil
}

func (m *mockPublisher) Channel(ctx context.Context, channelID string) (Channel, error) {
	return m.channels[channelID], nil
}

func (m *mockPublisher) StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[GuildMember, error] {
	return func(yield func(GuildMember, error) bool) {
		for _, mem := range m.members[guildID] {
			if !yield(mem, nil) {
				return
			}
		}
	}
}
