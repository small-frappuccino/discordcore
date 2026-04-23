package qotd

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	canonicalQOTDChannelName  = "☆-qotd-☆"
	canonicalQOTDChannelTopic = "Daily QOTD prompts and answer threads."
)

type SetupForumParams struct {
	GuildID                       string
	PreferredChannelID            string
	PreferredQuestionListThreadID string
	VerifiedRoleID                string
}

type SetupForumResult struct {
	ChannelID            string
	ChannelName          string
	ChannelURL           string
	QuestionListThreadID string
	QuestionListPostURL  string
}

type forumSetupTransport interface {
	CurrentBotUserID(ctx context.Context) (string, error)
	ResolveChannel(ctx context.Context, guildID, channelID string) (*discordgo.Channel, error)
	ListTextChannels(ctx context.Context, guildID string) ([]*discordgo.Channel, error)
	CreateTextChannel(ctx context.Context, guildID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error)
	SyncChannel(ctx context.Context, channelID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error)
	EnsureQuestionListThread(ctx context.Context, channelID, preferredThreadID string) (string, error)
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

	channel, err := c.ensureChannel(ctx, normalized)
	if err != nil {
		return nil, err
	}
	channelID := strings.TrimSpace(channel.ID)
	if channelID == "" {
		return nil, fmt.Errorf("setup qotd forum: missing channel id")
	}

	return &SetupForumResult{
		ChannelID:   channelID,
		ChannelName: canonicalQOTDChannelName,
		ChannelURL:  BuildChannelJumpURL(normalized.GuildID, channelID),
	}, nil
}

func (c forumSetupCoordinator) ensureChannel(ctx context.Context, params SetupForumParams) (*discordgo.Channel, error) {
	botUserID, err := c.transport.CurrentBotUserID(ctx)
	if err != nil {
		return nil, err
	}
	overwrites := qotdChannelPermissionOverwrites(params.GuildID, params.VerifiedRoleID, botUserID)

	preferred, err := c.transport.ResolveChannel(ctx, params.GuildID, params.PreferredChannelID)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: resolve preferred channel: %w", err)
	}
	if preferred != nil && strings.TrimSpace(preferred.Name) == canonicalQOTDChannelName {
		return c.syncChannel(ctx, preferred.ID, overwrites)
	}

	channels, err := c.transport.ListTextChannels(ctx, params.GuildID)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: list text channels: %w", err)
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if strings.TrimSpace(channel.Name) == canonicalQOTDChannelName {
			return c.syncChannel(ctx, channel.ID, overwrites)
		}
	}

	if preferred != nil {
		return c.syncChannel(ctx, preferred.ID, overwrites)
	}

	channel, err := c.transport.CreateTextChannel(
		ctx,
		params.GuildID,
		canonicalQOTDChannelName,
		canonicalQOTDChannelTopic,
		overwrites,
	)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: create channel: %w", err)
	}
	if channel == nil || strings.TrimSpace(channel.ID) == "" {
		return nil, fmt.Errorf("setup qotd forum: create channel: missing channel id")
	}
	return channel, nil
}

func (c forumSetupCoordinator) syncChannel(ctx context.Context, channelID string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
	channel, err := c.transport.SyncChannel(
		ctx,
		strings.TrimSpace(channelID),
		canonicalQOTDChannelName,
		canonicalQOTDChannelTopic,
		overwrites,
	)
	if err != nil {
		return nil, fmt.Errorf("setup qotd forum: sync channel: %w", err)
	}
	if channel == nil || strings.TrimSpace(channel.ID) == "" {
		return nil, fmt.Errorf("setup qotd forum: sync channel: missing channel id")
	}
	return channel, nil
}

func normalizeSetupForumParams(params SetupForumParams) (SetupForumParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.PreferredChannelID = strings.TrimSpace(params.PreferredChannelID)
	params.PreferredQuestionListThreadID = strings.TrimSpace(params.PreferredQuestionListThreadID)
	params.VerifiedRoleID = strings.TrimSpace(params.VerifiedRoleID)
	if params.GuildID == "" {
		return SetupForumParams{}, fmt.Errorf("guild id is required")
	}
	return params, nil
}

func qotdChannelPermissionOverwrites(guildID, verifiedRoleID, botUserID string) []*discordgo.PermissionOverwrite {
	everyoneAllow := int64(0)
	everyoneDeny := int64(discordgo.PermissionSendMessages)
	if strings.TrimSpace(verifiedRoleID) == "" {
		everyoneAllow = int64(
			discordgo.PermissionViewChannel |
				discordgo.PermissionReadMessageHistory |
				discordgo.PermissionSendMessagesInThreads,
		)
	} else {
		everyoneDeny |= int64(discordgo.PermissionViewChannel)
	}

	overwrites := []*discordgo.PermissionOverwrite{{
		ID:    guildID,
		Type:  discordgo.PermissionOverwriteTypeRole,
		Allow: everyoneAllow,
		Deny:  everyoneDeny,
	}}
	if verifiedRoleID = strings.TrimSpace(verifiedRoleID); verifiedRoleID != "" {
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			ID:   verifiedRoleID,
			Type: discordgo.PermissionOverwriteTypeRole,
			Allow: discordgo.PermissionViewChannel |
				discordgo.PermissionReadMessageHistory |
				discordgo.PermissionSendMessagesInThreads,
			Deny: discordgo.PermissionSendMessages,
		})
	}
	if botUserID != "" {
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			ID:    botUserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: qotdBotChannelPermissions(),
		})
	}
	return overwrites
}

func qotdBotChannelPermissions() int64 {
	return discordgo.PermissionViewChannel |
		discordgo.PermissionReadMessageHistory |
		discordgo.PermissionSendMessages |
		discordgo.PermissionCreatePublicThreads |
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

func (t discordForumSetupTransport) ResolveChannel(ctx context.Context, guildID, channelID string) (*discordgo.Channel, error) {
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
	if channel == nil || strings.TrimSpace(channel.GuildID) != guildID || channel.Type != discordgo.ChannelTypeGuildText {
		return nil, nil
	}
	return channel, nil
}

func (t discordForumSetupTransport) ListTextChannels(ctx context.Context, guildID string) ([]*discordgo.Channel, error) {
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
	textChannels := make([]*discordgo.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Type != discordgo.ChannelTypeGuildText {
			continue
		}
		textChannels = append(textChannels, channel)
	}
	return textChannels, nil
}

func (t discordForumSetupTransport) CreateTextChannel(ctx context.Context, guildID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
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
		Type:                 discordgo.ChannelTypeGuildText,
		Topic:                topic,
		PermissionOverwrites: overwrites,
	})
}

func (t discordForumSetupTransport) SyncChannel(ctx context.Context, channelID, name, topic string, overwrites []*discordgo.PermissionOverwrite) (*discordgo.Channel, error) {
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

func (t discordForumSetupTransport) EnsureQuestionListThread(ctx context.Context, channelID, preferredThreadID string) (string, error) {
	if t.publisher == nil {
		return "", fmt.Errorf("publisher is required")
	}
	return newQuestionListArtifactPublisher(t.publisher, t.session).EnsureSealedThread(ctx, channelID, preferredThreadID)
}
