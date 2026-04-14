package qotd

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type fakeForumSetupTransport struct {
	botUserID            string
	preferredChannel     *discordgo.Channel
	listedChannels       []*discordgo.Channel
	createdChannel       *discordgo.Channel
	syncedChannel        *discordgo.Channel
	questionListThreadID string

	createCalls []discordgo.GuildChannelCreateData
	syncCalls   []struct {
		channelID  string
		name       string
		topic      string
		overwrites []*discordgo.PermissionOverwrite
	}
	ensureThreadCalls []string

	resolveErr error
	listErr    error
	createErr  error
	syncErr    error
	threadErr  error
}

func (f *fakeForumSetupTransport) CurrentBotUserID(context.Context) (string, error) {
	if f.botUserID == "" {
		return "", errors.New("missing bot user id")
	}
	return f.botUserID, nil
}

func (f *fakeForumSetupTransport) ResolveForumChannel(context.Context, string, string) (*discordgo.Channel, error) {
	return f.preferredChannel, f.resolveErr
}

func (f *fakeForumSetupTransport) ListForumChannels(context.Context, string) ([]*discordgo.Channel, error) {
	return f.listedChannels, f.listErr
}

func (f *fakeForumSetupTransport) CreateForumChannel(_ context.Context, _ string, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
	f.createCalls = append(f.createCalls, discordgo.GuildChannelCreateData{
		Name:                 name,
		Topic:                topic,
		Type:                 discordgo.ChannelTypeGuildForum,
		PermissionOverwrites: overwrites,
	})
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createdChannel == nil {
		f.createdChannel = &discordgo.Channel{ID: "forum-created", Name: name, Type: discordgo.ChannelTypeGuildForum}
	}
	return f.createdChannel, nil
}

func (f *fakeForumSetupTransport) SyncForumChannel(_ context.Context, channelID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
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
		f.syncedChannel = &discordgo.Channel{ID: channelID, Name: name, Type: discordgo.ChannelTypeGuildForum}
	}
	return f.syncedChannel, nil
}

func (f *fakeForumSetupTransport) EnsureQuestionListThread(_ context.Context, forumChannelID, preferredThreadID string) (string, error) {
	f.ensureThreadCalls = append(f.ensureThreadCalls, forumChannelID+"|"+preferredThreadID)
	if f.threadErr != nil {
		return "", f.threadErr
	}
	if f.questionListThreadID == "" {
		f.questionListThreadID = "questions-list-thread"
	}
	return f.questionListThreadID, nil
}

func TestForumSetupCoordinatorCreatesCanonicalForumAndThread(t *testing.T) {
	t.Parallel()

	transport := &fakeForumSetupTransport{
		botUserID: "bot-user",
	}
	coordinator := forumSetupCoordinator{transport: transport}

	result, err := coordinator.Setup(context.Background(), SetupForumParams{
		GuildID: "guild-1",
	})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected setup result")
	}
	if result.ForumChannelID != "forum-created" || result.QuestionListThreadID != "questions-list-thread" {
		t.Fatalf("unexpected setup result: %+v", result)
	}
	if len(transport.createCalls) != 1 {
		t.Fatalf("expected one forum channel creation, got %+v", transport.createCalls)
	}
	if transport.createCalls[0].Name != canonicalQOTDForumChannelName {
		t.Fatalf("expected canonical forum name, got %+v", transport.createCalls[0])
	}
	if len(transport.createCalls[0].PermissionOverwrites) != 2 {
		t.Fatalf("expected everyone + bot overwrites, got %+v", transport.createCalls[0].PermissionOverwrites)
	}
	everyoneOverwrite := transport.createCalls[0].PermissionOverwrites[0]
	if everyoneOverwrite.Deny&discordgo.PermissionSendMessages == 0 ||
		everyoneOverwrite.Allow&discordgo.PermissionSendMessagesInThreads == 0 {
		t.Fatalf("expected setup to block new forum posts but keep thread replies available, got %+v", everyoneOverwrite)
	}
	if len(transport.ensureThreadCalls) != 1 || transport.ensureThreadCalls[0] != "forum-created|" {
		t.Fatalf("expected one question-list bootstrap call, got %+v", transport.ensureThreadCalls)
	}
}

func TestForumSetupCoordinatorPrefersExistingCanonicalForum(t *testing.T) {
	t.Parallel()

	transport := &fakeForumSetupTransport{
		botUserID: "bot-user",
		listedChannels: []*discordgo.Channel{
			{ID: "forum-existing", Name: canonicalQOTDForumChannelName, Type: discordgo.ChannelTypeGuildForum},
		},
	}
	coordinator := forumSetupCoordinator{transport: transport}

	result, err := coordinator.Setup(context.Background(), SetupForumParams{
		GuildID:                       "guild-1",
		PreferredForumChannelID:       "forum-old",
		PreferredQuestionListThreadID: "questions-list-existing",
	})
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	if result == nil || result.ForumChannelID != "forum-existing" {
		t.Fatalf("expected canonical forum reuse, got %+v", result)
	}
	if len(transport.createCalls) != 0 {
		t.Fatalf("expected no new forum creation, got %+v", transport.createCalls)
	}
	if len(transport.syncCalls) != 1 || transport.syncCalls[0].channelID != "forum-existing" {
		t.Fatalf("expected sync on canonical forum, got %+v", transport.syncCalls)
	}
	if len(transport.ensureThreadCalls) != 1 || transport.ensureThreadCalls[0] != "forum-existing|questions-list-existing" {
		t.Fatalf("unexpected question list bootstrap call: %+v", transport.ensureThreadCalls)
	}
}
