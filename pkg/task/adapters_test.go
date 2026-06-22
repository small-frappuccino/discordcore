//go:build !legacy
// +build !legacy

package task

import (
	"context"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/members"
)

type mockDB struct {
	execCount int
}

type mockMembersRepo struct {
	db *mockDB
}

func (m *mockMembersRepo) GetUserPreferences(ctx context.Context, userID string) (*members.UserPreferences, error) {
	return nil, nil
}
func (m *mockMembersRepo) UpdateUserPreferences(ctx context.Context, prefs *members.UserPreferences) error {
	return nil
}
func (m *mockMembersRepo) UpsertGuildMemberSnapshotsContext(ctx context.Context, guildID string, snapshots []members.Snapshot, updatedAt time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		m.db.execCount++
		return nil
	}
}
func (m *mockMembersRepo) UpsertMemberJoinContext(ctx context.Context, guildID, userID string, joinedAt time.Time) error {
	return nil
}
func (m *mockMembersRepo) UpsertMemberPresenceContext(ctx context.Context, input members.PresenceInput) error {
	return nil
}
func (m *mockMembersRepo) MemberJoin(ctx context.Context, guildID, userID string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (m *mockMembersRepo) GetAvatar(ctx context.Context, guildID, userID string) (hash string, updatedAt time.Time, ok bool, err error) {
	return "", time.Time{}, false, nil
}
func (m *mockMembersRepo) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[members.CurrentState, error] {
	return nil
}
func (m *mockMembersRepo) StreamAllGuildMemberRoles(ctx context.Context, guildID string) (iter.Seq2[string, []string], error) {
	return nil, nil
}
func (m *mockMembersRepo) MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error {
	return nil
}
func (m *mockMembersRepo) UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error {
	return nil
}

func TestAdapters_TransactionalFallback(t *testing.T) {
	t.Parallel()

	db := &mockDB{}
	store := &mockMembersRepo{db: db}

	adapters := &NotificationAdapters{
		Router:          NewRouter(Defaults()),
		AvatarProcessor: nil, // Intentionally suppressed to trigger storage fallback
		MembersRepo:     store,
	}
	defer adapters.Router.Close()

	adapters.RegisterHandlers()

	payload := AvatarChangePayload{
		GuildID:   "123456789012345678",
		UserID:    "876543210987654321",
		NewAvatar: "abcdef1234567890",
	}

	// First pass: Valid context (Commit)
	ctx := context.Background()
	err := adapters.handleProcessAvatarChange(ctx, payload)
	if err != nil {
		t.Fatalf("Expected success for UpsertGuildMemberSnapshotsContext, got: %v", err)
	}

	if db.execCount == 0 {
		t.Fatalf("Expected fallback database insertion to be executed")
	}

	// Second pass: Invalid context (Rollback via simulated error)
	canceledCtx, cancelRollback := context.WithCancel(context.Background())
	cancelRollback() // Cancel immediately

	errRollback := adapters.handleProcessAvatarChange(canceledCtx, payload)
	if errRollback == nil {
		t.Fatalf("Expected context cancellation to force a database transaction rollback, got success")
	}
	if !strings.Contains(errRollback.Error(), "context canceled") {
		t.Fatalf("Expected context cancellation error, got: %v", errRollback)
	}
}
