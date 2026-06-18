package members

import (
	"context"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
)

// MemberSink is the abstraction for emitting pure member events.
type MemberSink interface {
	// OnMemberJoin is emitted when a member joins the guild.
	OnMemberJoin(ctx context.Context, e *gateway.GuildMemberAddEvent, accountAge time.Duration)

	// OnMemberLeave is emitted when a member leaves the guild.
	OnMemberLeave(ctx context.Context, e *gateway.GuildMemberRemoveEvent, serverTime time.Duration, botTime time.Duration)

	// OnRoleUpdate is emitted when a member's roles change.
	OnRoleUpdate(ctx context.Context, guildID string, user discord.User, addedRoles, removedRoles []discord.RoleID)

	// OnAvatarUpdate is emitted when a user's avatar changes.
	OnAvatarUpdate(ctx context.Context, guildID string, user discord.User, oldAvatarHash, newAvatarHash string)

	// OnModerationAction is emitted when a moderation action occurs.
	OnModerationAction(ctx context.Context, guildID string, actionType string, targetUser discord.User, reason string, moderator discord.User)
}

// NopMemberSink is a no-operation implementation of MemberSink.
type NopMemberSink struct{}

func (NopMemberSink) OnMemberJoin(ctx context.Context, e *gateway.GuildMemberAddEvent, accountAge time.Duration) {
}
func (NopMemberSink) OnMemberLeave(ctx context.Context, e *gateway.GuildMemberRemoveEvent, serverTime time.Duration, botTime time.Duration) {
}
func (NopMemberSink) OnRoleUpdate(ctx context.Context, guildID string, user discord.User, addedRoles, removedRoles []discord.RoleID) {
}
func (NopMemberSink) OnAvatarUpdate(ctx context.Context, guildID string, user discord.User, oldAvatarHash, newAvatarHash string) {
}
func (NopMemberSink) OnModerationAction(ctx context.Context, guildID string, actionType string, targetUser discord.User, reason string, moderator discord.User) {
}
