package qotd

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	canonicalQOTDForumChannelName  = "☆-qotd-☆"
	canonicalQOTDForumChannelTopic = "Daily QOTD prompts and answer threads."
)

type SetupForumParams struct {
	GuildID                       string
	PreferredForumChannelID       string
	PreferredQuestionListThreadID string
}

type SetupForumResult struct {
	ForumChannelID       string
	ForumChannelName     string
	ForumChannelURL      string
	QuestionListThreadID string
	QuestionListPostURL  string
}

type forumSetupTransport interface {
	CurrentBotUserID(ctx context.Context) (string, error)
	ResolveForumChannel(ctx context.Context, guildID, channelID string) (*discordgo.Channel, error)
	ListForumChannels(ctx context.Context, guildID string) ([]*discordgo.Channel, error)
	CreateForumChannel(ctx context.Context, guildID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error)
	SyncForumChannel(ctx context.Context, channelID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error)
	EnsureQuestionListThread(ctx context.Context, forumChannelID, preferredThreadID string) (string, error)
}

type forumSetupCoordinator struct {
	transport forumSetupTransport
}

func newForumSetupCoordinator(p *Publisher, session *discordgo.Session) forumSetupCoordinator {
	return forumSetupCoordinator{
		transport: discordForumSetupTransport{
			publisher: p,
			session:   session,
		},
	}
}

func (p *Publisher) SetupForum(ctx context.Context, session *discordgo.Session, params SetupForumParams) (*SetupForumResult, error) {
	if session == nil {
		return nil, fmt.Errorf("setup qotd forum: discord session is required")
	}
	return newForumSetupCoordinator(p, session).Setup(ctx, params)
}

func (c forumSetupCoordinator) Setup(ctx context.Context, params SetupForumParams) (*SetupForumResult, error) {
	normalized, err := normalizeSetupForumParams(params)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: %w", err)
	}
	if c.transport == nil {
		return nil, fmt.Errorf("setup qotd forum: transport is required")
	}

	forumChannel, err := c.ensureForumChannel(ctx, normalized)
	if err != nil {
		return nil, err
	}
	forumChannelID := strings.TrimSpace(forumChannel.ID)
	if forumChannelID == "" {
		return nil, fmt.Errorf("setup qotd forum: missing forum channel id")
	}

	questionListThreadID, err := c.transport.EnsureQuestionListThread(
		ctx,
		forumChannelID,
		normalized.PreferredQuestionListThreadID,
	)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: ensure questions list thread: %w", err)
	}

	return &SetupForumResult{
		ForumChannelID:       forumChannelID,
		ForumChannelName:     canonicalQOTDForumChannelName,
		ForumChannelURL:      BuildChannelJumpURL(normalized.GuildID, forumChannelID),
		QuestionListThreadID: strings.TrimSpace(questionListThreadID),
		QuestionListPostURL:  BuildChannelJumpURL(normalized.GuildID, questionListThreadID),
	}, nil
}

func (c forumSetupCoordinator) ensureForumChannel(ctx context.Context, params SetupForumParams) (*discordgo.Channel, error) {
	botUserID, err := c.transport.CurrentBotUserID(ctx)
	if err != nil {
		return nil, err
	}
	overwrites := qotdForumPermissionOverwrites(params.GuildID, botUserID)

	preferred, err := c.transport.ResolveForumChannel(ctx, params.GuildID, params.PreferredForumChannelID)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: resolve preferred forum channel: %w", err)
	}
	if preferred != nil && strings.TrimSpace(preferred.Name) == canonicalQOTDForumChannelName {
		return c.syncForumChannel(ctx, preferred.ID, overwrites)
	}

	channels, err := c.transport.ListForumChannels(ctx, params.GuildID)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: list forum channels: %w", err)
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if strings.TrimSpace(channel.Name) == canonicalQOTDForumChannelName {
			return c.syncForumChannel(ctx, channel.ID, overwrites)
		}
	}

	if preferred != nil {
		return c.syncForumChannel(ctx, preferred.ID, overwrites)
	}

	channel, err := c.transport.CreateForumChannel(
		ctx,
		params.GuildID,
		canonicalQOTDForumChannelName,
		canonicalQOTDForumChannelTopic,
		overwrites,
	)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: create forum channel: %w", err)
	}
	if channel == nil || strings.TrimSpace(channel.ID) == "" {
		return nil, fmt.Errorf("setup qotd forum: create forum channel: missing channel id")
	}
	return channel, nil
}

func (c forumSetupCoordinator) syncForumChannel(ctx context.Context, channelID string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
	channel, err := c.transport.SyncForumChannel(
		ctx,
		strings.TrimSpace(channelID),
		canonicalQOTDForumChannelName,
		canonicalQOTDForumChannelTopic,
		overwrites,
	)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: sync forum channel: %w", err)
	}
	if channel == nil || strings.TrimSpace(channel.ID) == "" {
		return nil, fmt.Errorf("setup qotd forum: sync forum channel: missing channel id")
	}
	return channel, nil
}

func normalizeSetupForumParams(params SetupForumParams) (SetupForumParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.PreferredForumChannelID = strings.TrimSpace(params.PreferredForumChannelID)
	params.PreferredQuestionListThreadID = strings.TrimSpace(params.PreferredQuestionListThreadID)
	if params.GuildID == "" {
		return SetupForumParams{}, fmt.Errorf("guild id is required")
	}
	return params, nil
}

func qotdForumPermissionOverwrites(guildID, botUserID string) []*discordgo.PermissionOverwrite {
	overwrites := []*discordgo.PermissionOverwrite{
		{
			ID:   guildID,
			Type: discordgo.PermissionOverwriteTypeRole,
			Allow: discordgo.PermissionViewChannel |
				discordgo.PermissionReadMessageHistory |
				discordgo.PermissionSendMessagesInThreads,
			Deny: discordgo.PermissionSendMessages,
		},
	}
	if botUserID != "" {
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			ID:    botUserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: qotdBotForumPermissions(),
		})
	}
	return overwrites
}

func qotdBotForumPermissions() int64 {
	return discordgo.PermissionViewChannel |
		discordgo.PermissionReadMessageHistory |
		discordgo.PermissionSendMessages |
		discordgo.PermissionSendMessagesInThreads |
		discordgo.PermissionManageThreads
}

type discordForumSetupTransport struct {
	publisher *Publisher
	session   *discordgo.Session
}

func (t discordForumSetupTransport) CurrentBotUserID(ctx context.Context) (string, error) {
	if t.session == nil {
		return "", fmt.Errorf("discord session is required")
	}
	if t.session.State != nil && t.session.State.User != nil {
		if botUserID := strings.TrimSpace(t.session.State.User.ID); botUserID != "" {
			return botUserID, nil
		}
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	user, err := t.session.User("@me")
	if err != nil {
		return "", fmt.Errorf("resolve bot user: %w", err)
	}
	if user == nil {
		return "", fmt.Errorf("resolve bot user: discord returned no user")
	}
	return strings.TrimSpace(user.ID), nil
}

func (t discordForumSetupTransport) ResolveForumChannel(ctx context.Context, guildID, channelID string) (*discordgo.Channel, error) {
	guildID = strings.TrimSpace(guildID)
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, nil
	}
	if guildID == "" {
		return nil, fmt.Errorf("guild id is required")
	}
	if t.session == nil {
		return nil, fmt.Errorf("discord session is required")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	channel, err := t.session.Channel(channelID)
	if err != nil {
		if isNotFoundRESTError(err) {
			return nil, nil
		}
		return nil, err
	}
	if channel == nil || strings.TrimSpace(channel.GuildID) != guildID || channel.Type != discordgo.ChannelTypeGuildForum {
		return nil, nil
	}
	return channel, nil
}

func (t discordForumSetupTransport) ListForumChannels(ctx context.Context, guildID string) ([]*discordgo.Channel, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, fmt.Errorf("guild id is required")
	}
	if t.session == nil {
		return nil, fmt.Errorf("discord session is required")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	channels, err := t.session.GuildChannels(guildID)
	if err != nil {
		return nil, err
	}
	forums := make([]*discordgo.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Type != discordgo.ChannelTypeGuildForum {
			continue
		}
		forums = append(forums, channel)
	}
	return forums, nil
}

func (t discordForumSetupTransport) CreateForumChannel(ctx context.Context, guildID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, fmt.Errorf("guild id is required")
	}
	if t.session == nil {
		return nil, fmt.Errorf("discord session is required")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return t.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:                 name,
		Type:                 discordgo.ChannelTypeGuildForum,
		Topic:                topic,
		PermissionOverwrites: overwrites,
	})
}

func (t discordForumSetupTransport) SyncForumChannel(ctx context.Context, channelID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	if t.session == nil {
		return nil, fmt.Errorf("discord session is required")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return t.session.ChannelEditComplex(channelID, &discordgo.ChannelEdit{
		Name:                 name,
		Topic:                topic,
		PermissionOverwrites: overwrites,
	})
}

func (t discordForumSetupTransport) EnsureQuestionListThread(ctx context.Context, forumChannelID, preferredThreadID string) (string, error) {
	if t.publisher == nil {
		return "", fmt.Errorf("publisher is required")
	}
	return newQuestionListArtifactPublisher(t.publisher, t.session).EnsureSealedThread(ctx, forumChannelID, preferredThreadID)
}
