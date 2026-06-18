package logging

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
)

// MemberEventSink defines the contract for logging member-related events.
type MemberEventSink interface {
	OnMemberJoin(ctx context.Context, guildID string, member discord.Member)
	OnMemberLeave(ctx context.Context, guildID string, user discord.User)
	OnRoleUpdate(ctx context.Context, guildID string, user discord.User, addedRoles, removedRoles []discord.RoleID)
}

// ModerationEventSink defines the contract for logging moderation actions.
type ModerationEventSink interface {
	OnModerationAction(ctx context.Context, guildID string, actionType string, targetUser discord.User, reason string, moderator discord.User)
}

// MonitoringEventSink defines the contract for logging generalized monitoring events.
type MonitoringEventSink interface {
	OnAvatarUpdate(ctx context.Context, guildID string, user discord.User, oldAvatarHash, newAvatarHash string)
}
