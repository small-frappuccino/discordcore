package eventlog

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
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
func (l *Logger) checkPolicy(eventType logpolicy.LogEventType, guildID string) (logpolicy.EmitDecision, bool) {
	decision := logpolicy.ShouldEmitLogEvent(l.session, l.config, eventType, guildID)
	return decision, decision.Enabled
}

// sendEmbed safely sends a logging embed using Arikawa API.
func (l *Logger) sendEmbed(ctx context.Context, channelID discord.ChannelID, embed discord.Embed, eventType logpolicy.LogEventType) {
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
	decision, ok := l.checkPolicy(logpolicy.LogEventMemberJoin, guildID)
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	embed := discord.Embed{
		Title:       "Member Joined",
		Description: fmt.Sprintf("<@%d> joined the server.", member.User.ID),
		Color:       0x43b581, // Green
		Thumbnail: &discord.EmbedThumbnail{
			URL: member.User.AvatarURL(),
		},
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("ID: %d", member.User.ID),
		},
	}
	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logpolicy.LogEventMemberJoin)
}

// OnMemberLeave handles member leave events.
func (l *Logger) OnMemberLeave(ctx context.Context, guildID string, user discord.User) {
	decision, ok := l.checkPolicy(logpolicy.LogEventMemberLeave, guildID)
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	embed := discord.Embed{
		Title:       "Member Left",
		Description: fmt.Sprintf("<@%d> left the server.", user.ID),
		Color:       0xf04747, // Red
		Thumbnail: &discord.EmbedThumbnail{
			URL: user.AvatarURL(),
		},
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("ID: %d", user.ID),
		},
	}
	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logpolicy.LogEventMemberLeave)
}

// OnRoleUpdate handles role updates for a member.
func (l *Logger) OnRoleUpdate(ctx context.Context, guildID string, user discord.User, addedRoles, removedRoles []discord.RoleID) {
	if len(addedRoles) == 0 && len(removedRoles) == 0 {
		return
	}

	decision, ok := l.checkPolicy(logpolicy.LogEventRoleChange, guildID)
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	desc := fmt.Sprintf("<@%d> roles were updated.\n", user.ID)
	if len(addedRoles) > 0 {
		desc += "**Added:** "
		for _, r := range addedRoles {
			desc += fmt.Sprintf("<@&%d> ", r)
		}
		desc += "\n"
	}
	if len(removedRoles) > 0 {
		desc += "**Removed:** "
		for _, r := range removedRoles {
			desc += fmt.Sprintf("<@&%d> ", r)
		}
	}

	embed := discord.Embed{
		Title:       "Member Roles Updated",
		Description: desc,
		Color:       0xfaa61a, // Yellow
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("ID: %d", user.ID),
		},
	}
	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logpolicy.LogEventRoleChange)
}

// OnMessageEdit handles message edit events.
func (l *Logger) OnMessageEdit(ctx context.Context, guildID string, channelID discord.ChannelID, oldMessage, newMessage discord.Message) {
	decision, ok := l.checkPolicy(logpolicy.LogEventMessageEdit, guildID)
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	embed := discord.Embed{
		Title:       "Message Edited",
		Description: fmt.Sprintf("**Channel:** <#%d>\n**Author:** <@%d>", channelID, newMessage.Author.ID),
		Color:       0xfaa61a, // Yellow
		Fields: []discord.EmbedField{
			{Name: "Old Content", Value: oldMessage.Content},
			{Name: "New Content", Value: newMessage.Content},
		},
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("Message ID: %d | Author ID: %d", newMessage.ID, newMessage.Author.ID),
		},
	}
	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logpolicy.LogEventMessageEdit)
}

// OnMessageDelete handles message delete events.
func (l *Logger) OnMessageDelete(ctx context.Context, guildID string, channelID discord.ChannelID, message discord.Message) {
	decision, ok := l.checkPolicy(logpolicy.LogEventMessageDelete, guildID)
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	embed := discord.Embed{
		Title:       "Message Deleted",
		Description: fmt.Sprintf("**Channel:** <#%d>\n**Author:** <@%d>", channelID, message.Author.ID),
		Color:       0xf04747, // Red
		Fields: []discord.EmbedField{
			{Name: "Content", Value: message.Content},
		},
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("Message ID: %d | Author ID: %d", message.ID, message.Author.ID),
		},
	}
	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logpolicy.LogEventMessageDelete)
}

// OnModerationAction handles moderation actions (from our bot or external).
func (l *Logger) OnModerationAction(ctx context.Context, guildID string, actionType string, targetUser discord.User, reason string, moderator discord.User) {
	decision, ok := l.checkPolicy(logpolicy.LogEventModerationCase, guildID)
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

	embed := discord.Embed{
		Title:       fmt.Sprintf("Moderation Action: %s", actionType),
		Color:       0x992d22, // Dark Red
		Description: fmt.Sprintf("**Target:** <@%d>\n**Moderator:** <@%d>\n**Reason:** %s", targetUser.ID, moderator.ID, reason),
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("Target ID: %d", targetUser.ID),
		},
	}
	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logpolicy.LogEventModerationCase)
}

// OnAvatarUpdate handles user avatar change events.
func (l *Logger) OnAvatarUpdate(ctx context.Context, guildID string, user discord.User, oldAvatarHash, newAvatarHash string) {
	decision, ok := l.checkPolicy(logpolicy.LogEventAvatarChange, guildID)
	if !ok {
		return
	}

	logChannelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	embed := discord.Embed{
		Title:       "Avatar Updated",
		Description: fmt.Sprintf("<@%d> changed their avatar.", user.ID),
		Color:       0x3498db, // Blue
		Thumbnail: &discord.EmbedThumbnail{
			URL: user.AvatarURL(),
		},
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("User ID: %d", user.ID),
		},
	}
	l.sendEmbed(ctx, discord.ChannelID(logChannelID), embed, logpolicy.LogEventAvatarChange)
}
