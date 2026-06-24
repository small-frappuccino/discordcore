package logging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logging"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"github.com/small-frappuccino/discordcore/pkg/messages"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// Logger implements the various EventSinks to handle logging natively via Arikawa,
// decoupling embed creation from domain packages and reducing GC heap allocations.
type Logger struct {
	client  *api.Client
	config  *files.ConfigManager
	state   *state.State
	intents gateway.Intents
	logger  *slog.Logger
}

// NewLogger creates a new event logger instance.
func NewLogger(client *api.Client, config *files.ConfigManager, st *state.State, intents gateway.Intents, logger *slog.Logger) *Logger {
	return &Logger{
		client:  client,
		config:  config,
		state:   st,
		intents: intents,
		logger:  logger,
	}
}

// checkPolicy evaluates whether the event should be logged.
func (l *Logger) checkPolicy(eventType logging.LogEventType, guildID string) (logging.EmitDecision, bool) {
	decision := logging.CheckFeatureEnabled(l.config, eventType, guildID)
	if !decision.Enabled {
		l.logger.Debug("Log event suppressed by configuration policy", slog.String("event_type", string(eventType)), slog.String("guild_id", guildID), slog.String("reason", string(decision.Reason)))
		return decision, false
	}

	reason, mask, ok := logging.ValidateLogCapability(l.state, l.intents, decision, guildID, l.config)
	if !ok {
		if reason == logging.EmitReasonMissingIntent || reason == logging.EmitReasonChannelInvalid {
			l.logger.Warn("Dropped logging event due to capability restrictions",
				slog.String("event_type", string(eventType)),
				slog.String("guild_id", guildID),
				slog.String("reason", string(reason)),
				slog.Int("missing_mask", int(mask)),
			)
		} else {
			l.logger.Debug("Log event suppressed by capability policy", slog.String("event_type", string(eventType)), slog.String("guild_id", guildID), slog.String("reason", string(reason)))
		}
		return decision, false
	}
	return decision, true
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
func (l *Logger) OnMemberJoin(ctx context.Context, intent members.MemberJoinIntent, accountAge time.Duration) {
	decision, ok := l.checkPolicy(logging.LogEventMemberJoin, intent.GuildID)
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		l.logger.Error("Failed to parse Snowflake ID for MemberJoin log channel", "guild_id", intent.GuildID, "channel_id", decision.ChannelID, "error", err)
		return
	}

	joinAgeText := logging.FormatDurationSmart(accountAge)
	if joinAgeText == "" {
		joinAgeText = "- ago"
	} else {
		joinAgeText = joinAgeText + " ago"
	}

	ce := files.CustomEmbedConfig{
		Title:        "Member Joined",
		Description:  logging.FormatUserLabel(intent.Username, intent.UserID),
		Color:        theme.MemberJoin(),
		ThumbnailURL: logging.FormatAvatarURL(intent.UserID, intent.AvatarHash),
		Fields: []files.CustomEmbedFieldConfig{
			{
				Name:   "Account Created",
				Value:  joinAgeText,
				Inline: true,
			},
		},
	}
	embed := embeds.Render(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logging.LogEventMemberJoin)
}

// OnMemberLeave handles member leave events.
func (l *Logger) OnMemberLeave(ctx context.Context, intent members.MemberLeaveIntent, serverTime time.Duration, botTime time.Duration) {
	decision, ok := l.checkPolicy(logging.LogEventMemberLeave, intent.GuildID)
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	ce := files.CustomEmbedConfig{
		Title:        "Member Left",
		Description:  logging.FormatUserLabel(intent.Username, intent.UserID),
		Color:        theme.MemberLeave(),
		ThumbnailURL: logging.FormatAvatarURL(intent.UserID, intent.AvatarHash),
		Fields: []files.CustomEmbedFieldConfig{
			{
				Name:   "Time on Server",
				Value:  "N/A", // This could be enriched by passing joinedAt from the domain event
				Inline: true,
			},
		},
	}
	embed := embeds.Render(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logging.LogEventMemberLeave)
}

// OnRoleUpdate handles role updates for a member.
func (l *Logger) OnRoleUpdate(ctx context.Context, intent members.RoleUpdateIntent) {
	if len(intent.AddedRoles) == 0 && len(intent.RemovedRoles) == 0 {
		return
	}

	decision, ok := l.checkPolicy(logging.LogEventRoleChange, intent.GuildID)
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	targetLabel := logging.FormatUserLabel(intent.Username, intent.UserID)
	ce := files.CustomEmbedConfig{
		Title:       "Role Updated",
		Description: targetLabel,
		Color:       theme.MemberRoleUpdate(),
	}

	var fields []files.CustomEmbedFieldConfig
	for _, r := range intent.AddedRoles {
		fields = append(fields, files.CustomEmbedFieldConfig{
			Name:   "Role",
			Value:  logging.FormatRoleLabel(r, ""),
			Inline: true,
		})
		fields = append(fields, files.CustomEmbedFieldConfig{
			Name:   "Action",
			Value:  "Added",
			Inline: true,
		})
	}
	for _, r := range intent.RemovedRoles {
		fields = append(fields, files.CustomEmbedFieldConfig{
			Name:   "Role",
			Value:  logging.FormatRoleLabel(r, ""),
			Inline: true,
		})
		fields = append(fields, files.CustomEmbedFieldConfig{
			Name:   "Action",
			Value:  "Removed",
			Inline: true,
		})
	}

	ce.Fields = fields
	embed := embeds.Render(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logging.LogEventRoleChange)
}

// OnMessageUpdate handles message update events to satisfy messages.MessageSink.
func (l *Logger) OnMessageUpdate(ctx context.Context, intent messages.MessageUpdateIntent, cachedMessage *messages.CachedMessageData) {
	if cachedMessage == nil {
		slog.Warn("Message update event dropped by event logger: no cached content available",
			slog.String("guild_id", intent.GuildID),
			slog.String("message_id", intent.MessageID),
		)
		return
	}

	decision, ok := l.checkPolicy(logging.LogEventMessageEdit, intent.GuildID)
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	jumpURL := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", intent.GuildID, intent.ChannelID, intent.MessageID)
	desc := "[Jump to message](" + jumpURL + ")"

	userField := logging.FormatUserLabel(cachedMessage.AuthorUsername, cachedMessage.AuthorID)
	channelField := logging.FormatChannelLabel(intent.ChannelID)
	messageTime := cachedMessage.Timestamp.Format("January 2, 2006 at 3:04 PM")

	ce := files.CustomEmbedConfig{
		Title:       "Message Edited",
		Description: desc,
		Color:       theme.MessageEdit(),
		AuthorName:  "Message Edited",
		Fields: []files.CustomEmbedFieldConfig{
			{Name: "User", Value: userField, Inline: true},
			{Name: "Channel", Value: channelField, Inline: true},
			{Name: "Message Timestamp", Value: messageTime, Inline: true},
			{Name: "Before", Value: logging.TruncateString(cachedMessage.Content, 1000), Inline: false},
			{Name: "After", Value: logging.TruncateString(intent.Content, 1000), Inline: false},
		},
		FooterText: fmt.Sprintf("Message ID: %s", intent.MessageID),
	}

	embed := embeds.Render(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logging.LogEventMessageEdit)
}

// OnMessageDelete handles message delete events to satisfy messages.MessageSink.
func (l *Logger) OnMessageDelete(ctx context.Context, intent messages.MessageDeleteIntent, cachedMessage *messages.CachedMessageData) {
	if cachedMessage == nil {
		slog.Warn("Message delete event dropped by event logger: no cached content available",
			slog.String("guild_id", intent.GuildID),
			slog.String("message_id", intent.MessageID),
		)
		return
	}

	decision, ok := l.checkPolicy(logging.LogEventMessageDelete, intent.GuildID)
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	userField := logging.FormatUserLabel(cachedMessage.AuthorUsername, cachedMessage.AuthorID)
	channelField := logging.FormatChannelLabel(intent.ChannelID)
	messageTime := cachedMessage.Timestamp.Format("January 2, 2006 at 3:04 PM")

	ce := files.CustomEmbedConfig{
		Title:      "Message Deleted",
		Color:      theme.MessageDelete(),
		AuthorName: "Message Deleted",
		Fields: []files.CustomEmbedFieldConfig{
			{Name: "User", Value: userField, Inline: true},
			{Name: "Channel", Value: channelField, Inline: true},
			{Name: "Message Timestamp", Value: messageTime, Inline: true},
			{Name: "Message", Value: logging.TruncateString(cachedMessage.Content, 1000), Inline: false},
		},
		FooterText: fmt.Sprintf("Message ID: %s", intent.MessageID),
	}

	if intent.ExecutorID != "" {
		ce.Description += fmt.Sprintf("\n**Deleted By:** <@%s>", intent.ExecutorID)
	}

	embed := embeds.Render(ce)
	embed.Timestamp = discord.NowTimestamp()

	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logging.LogEventMessageDelete)
}

// OnMessageDeleteBulk handles bulk message deletions to satisfy messages.MessageSink.
func (l *Logger) OnMessageDeleteBulk(ctx context.Context, intent messages.MessageDeleteBulkIntent) {
	slog.Info("Bulk delete event received but not fully forwarded to eventlog",
		slog.String("guild_id", intent.GuildID),
		slog.Int("count", len(intent.MessageIDs)),
	)
}

// OnModerationAction handles moderation actions (from our bot or external).
func (l *Logger) OnModerationAction(ctx context.Context, intent members.ModerationActionIntent) {
	decision, ok := l.checkPolicy(logging.LogEventModerationCase, intent.GuildID)
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	reason := intent.Reason
	if reason == "" {
		reason = "No reason provided."
	}

	ce := files.CustomEmbedConfig{
		Title: fmt.Sprintf("Moderation Action: %s", intent.ActionType),
		Color: theme.Danger(),
		Description: fmt.Sprintf("**Target:** %s\n**Moderator:** %s\n**Reason:** %s",
			logging.FormatUserRef(intent.TargetUserID),
			logging.FormatUserRef(intent.ModeratorID),
			reason),
		FooterText: fmt.Sprintf("Target ID: %s", intent.TargetUserID),
	}
	embed := embeds.Render(ce)
	embed.Timestamp = discord.NowTimestamp()
	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logging.LogEventModerationCase)
}

// OnAvatarUpdate handles user avatar change events.
func (l *Logger) OnAvatarUpdate(ctx context.Context, intent members.AvatarUpdateIntent) {
	decision, ok := l.checkPolicy(logging.LogEventAvatarChange, intent.GuildID)
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
		ThumbnailURL: logging.FormatAvatarURL(intent.UserID, intent.NewAvatarHash),
		Fields: []files.CustomEmbedFieldConfig{
			{Name: "User", Value: logging.FormatUserLabel(intent.Username, intent.UserID), Inline: true},
		},
		FooterText: fmt.Sprintf("User ID: %s", intent.UserID),
	}

	if intent.OldAvatarHash != "" {
		ce.Fields = append(ce.Fields, files.CustomEmbedFieldConfig{
			Name:   "Previous Avatar",
			Value:  "[See previous avatar](" + logging.FormatAvatarURL(intent.UserID, intent.OldAvatarHash) + ")",
			Inline: true,
		})
	}

	embed := embeds.Render(ce)
	embed.Timestamp = discord.NowTimestamp()

	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logging.LogEventAvatarChange)
}
