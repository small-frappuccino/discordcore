package logging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/small-frappuccino/discordcore/pkg/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logging"
	"github.com/small-frappuccino/discordcore/pkg/theme"
	"github.com/small-frappuccino/discordgo"
)

// Logger implements the various EventSinks to handle logging natively via Arikawa,
// decoupling embed creation from domain packages and reducing GC heap allocations.
type Logger struct {
	client  *api.Client
	config  *files.ConfigManager
	session *discordgo.Session
	logger  *slog.Logger
}

// NewLogger creates a new event logger instance.
func NewLogger(client *api.Client, config *files.ConfigManager, session *discordgo.Session, logger *slog.Logger) *Logger {
	return &Logger{
		client:  client,
		config:  config,
		session: session,
		logger:  logger,
	}
}

// checkPolicy evaluates whether the event should be logged.
func (l *Logger) checkPolicy(eventType logging.LogEventType, guildID string) (logging.EmitDecision, bool) {
	decision := logging.ShouldEmitLogEvent(l.session, l.config, eventType, guildID)
	return decision, decision.Enabled
}

// sendEmbed safely sends a logging embed using Arikawa API.
func (l *Logger) sendEmbed(ctx context.Context, channelID discord.ChannelID, embed discord.Embed, eventType logging.LogEventType) {
	_, err := l.client.WithContext(ctx).SendMessageComplex(channelID, api.SendMessageData{
		Embeds: []discord.Embed{embed},
	})
	if err != nil {
		l.logger.Error("Failed to send event log embed",
			slog.String("event_type", string(eventType)),
			slog.Int64("channel_id", int64(channelID)),
			slog.Any("error", err),
		)
	}
}

// OnMemberJoin handles member join events.
func (l *Logger) OnMemberJoin(ctx context.Context, guildID string, member discord.Member) {
	decision, ok := l.checkPolicy(logging.LogEventMemberJoin, guildID)
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	// Calculate account age (basic heuristic for illustration, full logic depends on snowflake parsing)
	createdAt := member.User.CreatedAt()
	accountAge := time.Since(createdAt)
	joinAgeText := logging.FormatDurationSmart(accountAge)
	if joinAgeText == "" {
		joinAgeText = "- ago"
	} else {
		joinAgeText = joinAgeText + " ago"
	}

	ce := files.CustomEmbedConfig{
		Title:        "Member Joined",
		Description:  logging.FormatUserLabel(member.User.Username, member.User.ID.String()),
		Color:        theme.MemberJoin(),
		ThumbnailURL: member.User.AvatarURL(),
		Fields: []files.CustomEmbedFieldConfig{
			{
				Name:   "Account Created",
				Value:  joinAgeText,
				Inline: true,
			},
		},
	}
	embed := embeds.RenderArikawa(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logging.LogEventMemberJoin)
}

// OnMemberLeave handles member leave events.
func (l *Logger) OnMemberLeave(ctx context.Context, guildID string, user discord.User) {
	decision, ok := l.checkPolicy(logging.LogEventMemberLeave, guildID)
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	ce := files.CustomEmbedConfig{
		Title:        "Member Left",
		Description:  logging.FormatUserLabel(user.Username, user.ID.String()),
		Color:        theme.MemberLeave(),
		ThumbnailURL: user.AvatarURL(),
		Fields: []files.CustomEmbedFieldConfig{
			{
				Name:   "Time on Server",
				Value:  "N/A", // This could be enriched by passing joinedAt from the domain event
				Inline: true,
			},
		},
	}
	embed := embeds.RenderArikawa(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logging.LogEventMemberLeave)
}

// OnRoleUpdate handles role updates for a member.
func (l *Logger) OnRoleUpdate(ctx context.Context, guildID string, user discord.User, addedRoles, removedRoles []discord.RoleID) {
	if len(addedRoles) == 0 && len(removedRoles) == 0 {
		return
	}

	decision, ok := l.checkPolicy(logging.LogEventRoleChange, guildID)
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	targetLabel := logging.FormatUserLabel(user.Username, user.ID.String())
	ce := files.CustomEmbedConfig{
		Title:       "Role Updated",
		Description: targetLabel,
		Color:       theme.MemberRoleUpdate(),
	}

	var fields []files.CustomEmbedFieldConfig
	for _, r := range addedRoles {
		fields = append(fields, files.CustomEmbedFieldConfig{
			Name:   "Role",
			Value:  logging.FormatRoleLabel(r.String(), ""),
			Inline: true,
		})
		fields = append(fields, files.CustomEmbedFieldConfig{
			Name:   "Action",
			Value:  "Added",
			Inline: true,
		})
	}
	for _, r := range removedRoles {
		fields = append(fields, files.CustomEmbedFieldConfig{
			Name:   "Role",
			Value:  logging.FormatRoleLabel(r.String(), ""),
			Inline: true,
		})
		fields = append(fields, files.CustomEmbedFieldConfig{
			Name:   "Action",
			Value:  "Removed",
			Inline: true,
		})
	}

	ce.Fields = fields
	embed := embeds.RenderArikawa(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logging.LogEventRoleChange)
}

// OnMessageEdit handles message edit events.
func (l *Logger) OnMessageEdit(ctx context.Context, guildID string, channelID discord.ChannelID, oldMessage, newMessage discord.Message) {
	decision, ok := l.checkPolicy(logging.LogEventMessageEdit, guildID)
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	jumpURL := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID.String(), newMessage.ID.String())
	desc := "[Jump to message](" + jumpURL + ")"

	userField := logging.FormatUserLabel(newMessage.Author.Username, newMessage.Author.ID.String())
	channelField := logging.FormatChannelLabel(channelID.String())
	messageTime := newMessage.Timestamp.Time().Format("January 2, 2006 at 3:04 PM")

	ce := files.CustomEmbedConfig{
		Title:         "Message Edited",
		Description:   desc,
		Color:         theme.MessageEdit(),
		AuthorName:    "Message Edited",
		AuthorIconURL: newMessage.Author.AvatarURL(),
		Fields: []files.CustomEmbedFieldConfig{
			{Name: "User", Value: userField, Inline: true},
			{Name: "Channel", Value: channelField, Inline: true},
			{Name: "Message Timestamp", Value: messageTime, Inline: true},
			{Name: "Before", Value: logging.TruncateString(oldMessage.Content, 1000), Inline: false},
			{Name: "After", Value: logging.TruncateString(newMessage.Content, 1000), Inline: false},
		},
		FooterText: fmt.Sprintf("Message ID: %s", newMessage.ID.String()),
	}

	embed := embeds.RenderArikawa(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logging.LogEventMessageEdit)
}

// OnMessageUpdate handles message update events to satisfy messages.MessageSink.
func (l *Logger) OnMessageUpdate(ctx context.Context, m *gateway.MessageUpdateEvent, cachedMessage *discord.Message) {
	if cachedMessage == nil {
		slog.Warn("Message update event dropped by event logger: no cached content available",
			slog.String("guild_id", m.GuildID.String()),
			slog.String("message_id", m.ID.String()),
		)
		return
	}
	newMessage := *cachedMessage
	if m.Content != "" {
		newMessage.Content = m.Content
	}
	l.OnMessageEdit(ctx, m.GuildID.String(), m.ChannelID, *cachedMessage, newMessage)
}

// OnMessageDelete handles message delete events to satisfy messages.MessageSink.
func (l *Logger) OnMessageDelete(ctx context.Context, m *gateway.MessageDeleteEvent, cachedMessage *discord.Message, executor *discord.User) {
	if cachedMessage == nil {
		slog.Warn("Message delete event dropped by event logger: no cached content available",
			slog.String("guild_id", m.GuildID.String()),
			slog.String("message_id", m.ID.String()),
		)
		return
	}

	decision, ok := l.checkPolicy(logging.LogEventMessageDelete, m.GuildID.String())
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	userField := logging.FormatUserLabel(cachedMessage.Author.Username, cachedMessage.Author.ID.String())
	channelField := logging.FormatChannelLabel(m.ChannelID.String())
	messageTime := cachedMessage.Timestamp.Time().Format("January 2, 2006 at 3:04 PM")

	ce := files.CustomEmbedConfig{
		Title:         "Message Deleted",
		Color:         theme.MessageDelete(),
		AuthorName:    "Message Deleted",
		AuthorIconURL: cachedMessage.Author.AvatarURL(),
		Fields: []files.CustomEmbedFieldConfig{
			{Name: "User", Value: userField, Inline: true},
			{Name: "Channel", Value: channelField, Inline: true},
			{Name: "Message Timestamp", Value: messageTime, Inline: true},
			{Name: "Message", Value: logging.TruncateString(cachedMessage.Content, 1000), Inline: false},
		},
		FooterText: fmt.Sprintf("Message ID: %s", cachedMessage.ID.String()),
	}

	if executor != nil {
		ce.Description += fmt.Sprintf("\n**Deleted By:** <@%s>", executor.ID.String())
	}

	embed := embeds.RenderArikawa(ce)
	embed.Timestamp = discord.NowTimestamp()

	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logging.LogEventMessageDelete)
}

// OnMessageDeleteBulk handles bulk message deletions to satisfy messages.MessageSink.
func (l *Logger) OnMessageDeleteBulk(ctx context.Context, guildID discord.GuildID, channelID discord.ChannelID, messageIDs []string) {
	slog.Info("Bulk delete event received but not fully forwarded to eventlog",
		slog.String("guild_id", guildID.String()),
		slog.Int("count", len(messageIDs)),
	)
}

// OnModerationAction handles moderation actions (from our bot or external).
func (l *Logger) OnModerationAction(ctx context.Context, guildID string, actionType string, targetUser discord.User, reason string, moderator discord.User) {
	decision, ok := l.checkPolicy(logging.LogEventModerationCase, guildID)
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	if reason == "" {
		reason = "No reason provided."
	}

	ce := files.CustomEmbedConfig{
		Title: fmt.Sprintf("Moderation Action: %s", actionType),
		Color: theme.Danger(),
		Description: fmt.Sprintf("**Target:** %s\n**Moderator:** %s\n**Reason:** %s",
			logging.FormatUserRef(targetUser.ID.String()),
			logging.FormatUserRef(moderator.ID.String()),
			reason),
		FooterText: fmt.Sprintf("Target ID: %s", targetUser.ID.String()),
	}
	embed := embeds.RenderArikawa(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logging.LogEventModerationCase)
}

// OnAvatarUpdate handles user avatar change events.
func (l *Logger) OnAvatarUpdate(ctx context.Context, guildID string, user discord.User, oldAvatarHash, newAvatarHash string) {
	decision, ok := l.checkPolicy(logging.LogEventAvatarChange, guildID)
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	ce := files.CustomEmbedConfig{
		Title:        "Avatar Updated",
		Color:        theme.AvatarChange(),
		ThumbnailURL: user.AvatarURL(),
		Fields: []files.CustomEmbedFieldConfig{
			{Name: "User", Value: logging.FormatUserLabel(user.Username, user.ID.String()), Inline: true},
		},
		FooterText: fmt.Sprintf("User ID: %s", user.ID.String()),
	}

	if oldAvatarHash != "" {
		oldUrl := fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", user.ID.String(), oldAvatarHash)
		if strings.HasPrefix(oldAvatarHash, "a_") {
			oldUrl = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.gif", user.ID.String(), oldAvatarHash)
		}
		ce.Fields = append(ce.Fields, files.CustomEmbedFieldConfig{
			Name:   "Previous Avatar",
			Value:  "[See previous avatar](" + oldUrl + ")",
			Inline: true,
		})
	}

	embed := embeds.RenderArikawa(ce)
	embed.Timestamp = discord.NowTimestamp()

	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logging.LogEventAvatarChange)
}
