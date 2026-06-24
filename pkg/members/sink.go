package members

import (
	"context"
	"time"
)

// MemberSink is the abstraction for emitting pure member events.
type MemberSink interface {
	// OnMemberJoin is emitted when a member joins the guild.
	OnMemberJoin(ctx context.Context, intent MemberJoinIntent, accountAge time.Duration)

	// OnMemberLeave is emitted when a member leaves the guild.
	OnMemberLeave(ctx context.Context, intent MemberLeaveIntent, serverTime time.Duration, botTime time.Duration)

	// OnRoleUpdate is emitted when a member's roles change.
	OnRoleUpdate(ctx context.Context, intent RoleUpdateIntent)

	// OnAvatarUpdate is emitted when a user's avatar changes.
	OnAvatarUpdate(ctx context.Context, intent AvatarUpdateIntent)

	// OnModerationAction is emitted when a moderation action occurs.
	OnModerationAction(ctx context.Context, intent ModerationActionIntent)
}

// NopMemberSink is a no-operation implementation of MemberSink.
type NopMemberSink struct{}

func (NopMemberSink) OnMemberJoin(ctx context.Context, intent MemberJoinIntent, accountAge time.Duration) {
}
func (NopMemberSink) OnMemberLeave(ctx context.Context, intent MemberLeaveIntent, serverTime time.Duration, botTime time.Duration) {
}
func (NopMemberSink) OnRoleUpdate(ctx context.Context, intent RoleUpdateIntent)             {}
func (NopMemberSink) OnAvatarUpdate(ctx context.Context, intent AvatarUpdateIntent)         {}
func (NopMemberSink) OnModerationAction(ctx context.Context, intent ModerationActionIntent) {}
