package members

import (
	"context"
	"iter"
	"time"
)

// Repository encapsulates the persistent storage logic for the members domain.
type Repository interface {
	GetUserPreferences(ctx context.Context, userID string) (*UserPreferences, error)
	UpdateUserPreferences(ctx context.Context, prefs *UserPreferences) error
	UpsertGuildMemberSnapshotsContext(ctx context.Context, guildID string, snapshots []Snapshot, updatedAt time.Time) error
	UpsertMemberJoinContext(ctx context.Context, guildID, userID string, joinedAt time.Time) error
	UpsertMemberPresenceContext(ctx context.Context, input PresenceInput) error
	MemberJoin(ctx context.Context, guildID, userID string) (time.Time, bool, error)
	GetAvatar(ctx context.Context, guildID, userID string) (hash string, updatedAt time.Time, ok bool, err error)
	GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[CurrentState, error]
	StreamAllGuildMemberRoles(ctx context.Context, guildID string) (iter.Seq2[string, []string], error)
	MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error
	UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error
}
