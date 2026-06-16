package members

import (
	"context"
	"time"

	"github.com/diamondburned/arikawa/v3/gateway"
)

// MemberSink is the abstraction for emitting pure member events.
type MemberSink interface {
	// OnMemberJoin is emitted when a member joins the guild.
	OnMemberJoin(ctx context.Context, e *gateway.GuildMemberAddEvent, accountAge time.Duration)

	// OnMemberLeave is emitted when a member leaves the guild.
	OnMemberLeave(ctx context.Context, e *gateway.GuildMemberRemoveEvent, serverTime time.Duration, botTime time.Duration)
}

// NopMemberSink is a no-operation implementation of MemberSink.
type NopMemberSink struct{}

func (NopMemberSink) OnMemberJoin(ctx context.Context, e *gateway.GuildMemberAddEvent, accountAge time.Duration) {
}
func (NopMemberSink) OnMemberLeave(ctx context.Context, e *gateway.GuildMemberRemoveEvent, serverTime time.Duration, botTime time.Duration) {
}
