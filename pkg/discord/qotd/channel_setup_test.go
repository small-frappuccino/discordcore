package qotd

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type fakeChannelSetupTransport struct {
	botUserID      string
	preferred      *discordgo.Channel
	listedChannels []*discordgo.Channel
	createdChannel *discordgo.Channel
	syncedChannel  *discordgo.Channel

	createCalls []discordgo.GuildChannelCreateData
	syncCalls   []struct {
		channelID  string
		name       string
		topic      string
		overwrites []*discordgo.PermissionOverwrite
	}

	resolveErr error
	listErr    error
	createErr  error
	syncErr    error
}

func (f *fakeChannelSetupTransport) CurrentBotUserID(context.Context) (string, error) {
	if f.botUserID == "" {
		return "", errors.New("missing bot user id")
	}
	return f.botUserID, nil
}

func (f *fakeChannelSetupTransport) ResolveChannel(context.Context, string, string) (*discordgo.Channel, error) {
	return f.preferred, f.resolveErr
}

func (f *fakeChannelSetupTransport) ListTextChannels(context.Context, string) ([]*discordgo.Channel, error) {
	return f.listedChannels, f.listErr
}

func (f *fakeChannelSetupTransport) CreateTextChannel(_ context.Context, _ string, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
	f.createCalls = append(f.createCalls, discordgo.GuildChannelCreateData{
		Name:                 name,
		Topic:                topic,
		Type:                 discordgo.ChannelTypeGuildText,
		PermissionOverwrites: overwrites,
	})
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createdChannel == nil {
		f.createdChannel = &discordgo.Channel{ID: "channel-created", Name: name, Type: discordgo.ChannelTypeGuildText}
	}
	return f.createdChannel, nil
}

func (f *fakeChannelSetupTransport) SyncChannel(_ context.Context, channelID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
	f.syncCalls = append(f.syncCalls, struct {
		channelID  string
		name       string
		topic      string
		overwrites []*discordgo.PermissionOverwrite
	}{
		channelID:  channelID,
		name:       name,
		topic:      topic,
		overwrites: overwrites,
	})
	if f.syncErr != nil {
		return nil, f.syncErr
	}
	if f.syncedChannel == nil {
		f.syncedChannel = &discordgo.Channel{ID: channelID, Name: name, Type: discordgo.ChannelTypeGuildText}
	}
	return f.syncedChannel, nil
}

func TestChannelSetupCoordinatorCreatesCanonicalChannel(t *testing.T) {
	t.Parallel()

	transport := &fakeChannelSetupTransport{botUserID: "bot-user"}
	coordinator := channelSetupCoordinator{transport: transport}

	result, err := coordinator.Setup(context.Background(), SetupChannelParams{GuildID: "guild-1"})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	if result == nil || result.ChannelID != "channel-created" {
		t.Fatalf("unexpected setup result: %+v", result)
	}
	if len(transport.createCalls) != 1 {
		t.Fatalf("expected one text channel creation, got %+v", transport.createCalls)
	}
	if transport.createCalls[0].Name != canonicalQOTDChannelName {
		t.Fatalf("expected canonical channel name, got %+v", transport.createCalls[0])
	}
	if len(transport.createCalls[0].PermissionOverwrites) != 2 {
		t.Fatalf("expected everyone + bot overwrites, got %+v", transport.createCalls[0].PermissionOverwrites)
	}
	everyoneOverwrite := transport.createCalls[0].PermissionOverwrites[0]
	if everyoneOverwrite.Deny&discordgo.PermissionSendMessages == 0 {
		t.Fatalf("expected setup to block direct channel messages, got %+v", everyoneOverwrite)
	}
	if everyoneOverwrite.Allow&discordgo.PermissionSendMessagesInThreads == 0 {
		t.Fatalf("expected setup to keep thread replies available, got %+v", everyoneOverwrite)
	}
}

func TestChannelSetupCoordinatorReusesCanonicalChannel(t *testing.T) {
	t.Parallel()

	transport := &fakeChannelSetupTransport{
		botUserID: "bot-user",
		listedChannels: []*discordgo.Channel{
			{ID: "channel-existing", Name: canonicalQOTDChannelName, Type: discordgo.ChannelTypeGuildText},
		},
	}
	coordinator := channelSetupCoordinator{transport: transport}

	result, err := coordinator.Setup(context.Background(), SetupChannelParams{
		GuildID:            "guild-1",
		PreferredChannelID: "channel-old",
	})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	if result == nil || result.ChannelID != "channel-existing" {
		t.Fatalf("expected canonical channel reuse, got %+v", result)
	}
	if len(transport.createCalls) != 0 {
		t.Fatalf("expected no new channel creation, got %+v", transport.createCalls)
	}
	if len(transport.syncCalls) != 1 || transport.syncCalls[0].channelID != "channel-existing" {
		t.Fatalf("expected sync on canonical channel, got %+v", transport.syncCalls)
	}
}

func TestChannelSetupCoordinatorLocksChannelToVerifiedRole(t *testing.T) {
	t.Parallel()

	transport := &fakeChannelSetupTransport{botUserID: "bot-user"}
	coordinator := channelSetupCoordinator{transport: transport}

	_, err := coordinator.Setup(context.Background(), SetupChannelParams{
		GuildID:        "guild-1",
		VerifiedRoleID: "verified-role",
	})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	if len(transport.createCalls) != 1 {
		t.Fatalf("expected one channel creation, got %+v", transport.createCalls)
	}
	if len(transport.createCalls[0].PermissionOverwrites) != 3 {
		t.Fatalf("expected everyone + verified role + bot overwrites, got %+v", transport.createCalls[0].PermissionOverwrites)
	}
	everyoneOverwrite := transport.createCalls[0].PermissionOverwrites[0]
	verifiedOverwrite := transport.createCalls[0].PermissionOverwrites[1]
	if everyoneOverwrite.Deny&discordgo.PermissionViewChannel == 0 {
		t.Fatalf("expected everyone to lose channel visibility, got %+v", everyoneOverwrite)
	}
	if verifiedOverwrite.ID != "verified-role" || verifiedOverwrite.Allow&discordgo.PermissionViewChannel == 0 {
		t.Fatalf("expected verified role to receive channel visibility, got %+v", verifiedOverwrite)
	}
}
