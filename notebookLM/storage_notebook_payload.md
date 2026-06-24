# Domain Architecture: storage

## Layout Topology
```text
storage/
└── postgres
    ├── storagetest
    │   ├── failing.go
    │   └── failing_test.go
    ├── members.go
    ├── members_test.go
    ├── messages.go
    ├── messages_test.go
    ├── moderation.go
    ├── moderation_test.go
    ├── qotd.go
    ├── qotd_test.go
    ├── query_placeholders_test.go
    ├── schema.go
    ├── schema_test.go
    ├── schema_unit_test.go
    ├── store.go
    ├── store_test.go
    ├── system.go
    └── system_test.go
```

## Source Stream Aggregation

// === FILE: pkg/storage/postgres/members.go ===
```go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/members"
)

// GetUserPreferences retrieves preferences or returns defaults if none exist.
//
// Defaults to "system" theme and "UTC" timezone upon pgx.ErrNoRows interception.
func (s *Store) GetUserPreferences(ctx context.Context, userID string) (*members.UserPreferences, error) {
	var prefs members.UserPreferences
	err := s.db.QueryRow(ctx, `SELECT user_id, theme, timezone FROM user_preferences WHERE user_id = $1`, userID).Scan(&prefs.UserID, &prefs.Theme, &prefs.Timezone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &members.UserPreferences{UserID: userID, Theme: "system", Timezone: "UTC"}, nil
		}
		return nil, fmt.Errorf("GetUserPreferences scan: %w", err)
	}
	return &prefs, nil
}

// UpdateUserPreferences upserts user preferences.
func (s *Store) UpdateUserPreferences(ctx context.Context, prefs *members.UserPreferences) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_preferences (user_id, theme, timezone, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			theme = EXCLUDED.theme,
			timezone = EXCLUDED.timezone,
			updated_at = NOW()
	`, prefs.UserID, prefs.Theme, prefs.Timezone)
	if err != nil {
		return fmt.Errorf("UpdateUserPreferences exec: %w", err)
	}
	return nil
}

// UpsertGuildMemberSnapshotsContext persists a batch of member snapshots transactionally.
func (s *Store) UpsertGuildMemberSnapshotsContext(ctx context.Context, guildID string, snapshots []members.Snapshot, updatedAt time.Time) (err error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || len(snapshots) == 0 {
		return nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	} else {
		updatedAt = updatedAt.UTC()
	}

	normalized := normalizeGuildMemberSnapshots(snapshots)
	if len(normalized) == 0 {
		return nil
	}

	s.log().Debug("upserting guild member snapshots batch", slog.String("guild_id", guildID), slog.Int("count", len(normalized)))

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Store.UpsertGuildMemberSnapshotsContext: %w", err)
	}
	defer func() {
		// Rollback logic intercepts pgx.ErrTxClosed for idempotent execution safety
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			s.log().Error("failed to rollback transaction", slog.String("guild_id", guildID), slog.String("error", rerr.Error()))
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	if err := upsertGuildMemberSnapshotBatch(ctx, tx, guildID, normalized, updatedAt); err != nil {
		return fmt.Errorf("Store.UpsertGuildMemberSnapshotsContext: %w", err)
	}
	return tx.Commit(ctx)
}

func upsertGuildMemberSnapshotBatch(ctx context.Context, tx pgx.Tx, guildID string, snapshots []members.Snapshot, updatedAt time.Time) error {
	avatarRows := make([]members.Snapshot, 0, len(snapshots))
	roleRows := make([]members.Snapshot, 0, len(snapshots))
	joinRows := make([]members.Snapshot, 0, len(snapshots))
	avatarUserIDs := make([]string, 0, len(snapshots))
	roleUserIDs := make([]string, 0, len(snapshots))

	for _, snapshot := range snapshots {
		if snapshot.HasAvatar {
			avatarRows = append(avatarRows, snapshot)
			avatarUserIDs = append(avatarUserIDs, snapshot.UserID)
		}
		if snapshot.HasRoles {
			roleRows = append(roleRows, snapshot)
			roleUserIDs = append(roleUserIDs, snapshot.UserID)
		}
		if !snapshot.JoinedAt.IsZero() || snapshot.HasBot {
			joinRows = append(joinRows, snapshot)
		}
	}

	if len(avatarRows) > 0 {
		currentHashes, err := queryCurrentAvatarHashesByUserID(ctx, tx, guildID, avatarUserIDs)
		if err != nil {
			return fmt.Errorf("query current avatar hashes: %w", err)
		}
		if err := insertAvatarHistoryBatch(ctx, tx, avatarHistoryBatch{GuildID: guildID, Snapshots: avatarRows, CurrentHashes: currentHashes, UpdatedAt: updatedAt}); err != nil {
			return fmt.Errorf("insert avatar history batch: %w", err)
		}
		if err := upsertAvatarCurrentBatch(ctx, tx, guildID, avatarRows, updatedAt); err != nil {
			return fmt.Errorf("upsert avatar current batch: %w", err)
		}
	}

	if len(roleRows) > 0 {
		if err := deleteRolesForUsersBatch(ctx, tx, guildID, roleUserIDs, updatedAt); err != nil {
			return fmt.Errorf("delete roles batch: %w", err)
		}
		if err := insertMemberRolesBatch(ctx, tx, guildID, roleRows, updatedAt); err != nil {
			return fmt.Errorf("insert roles batch: %w", err)
		}
	}

	if len(joinRows) > 0 {
		if err := upsertMemberJoinsBatch(ctx, tx, guildID, joinRows, updatedAt); err != nil {
			return fmt.Errorf("upsert member joins batch: %w", err)
		}
	}

	return nil
}

func normalizeGuildMemberSnapshots(snapshots []members.Snapshot) []members.Snapshot {
	if len(snapshots) == 0 {
		return nil
	}

	order := make([]string, 0, len(snapshots))
	byUser := make(map[string]*members.Snapshot, len(snapshots))
	for _, snapshot := range snapshots {
		userID := strings.TrimSpace(snapshot.UserID)
		if userID == "" {
			continue
		}
		existing, ok := byUser[userID]
		if !ok {
			existing = &members.Snapshot{UserID: userID}
			byUser[userID] = existing
			order = append(order, userID)
		}
		if snapshot.HasAvatar {
			existing.HasAvatar = true
			existing.AvatarHash = strings.TrimSpace(snapshot.AvatarHash)
			if existing.AvatarHash == "" {
				existing.AvatarHash = "default"
			}
		}
		if snapshot.HasRoles {
			existing.HasRoles = true
			existing.Roles = dedupeNonEmptyStrings(snapshot.Roles)
		}
		if snapshot.HasBot {
			existing.HasBot = true
			existing.IsBot = snapshot.IsBot
		}
		if !snapshot.JoinedAt.IsZero() {
			joinedAt := snapshot.JoinedAt.UTC()
			if existing.JoinedAt.IsZero() || joinedAt.Before(existing.JoinedAt) {
				existing.JoinedAt = joinedAt
			}
		}
	}

	normalized := make([]members.Snapshot, 0, len(order))
	for _, userID := range order {
		normalized = append(normalized, *byUser[userID])
	}
	return normalized
}

func dedupeNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func queryCurrentAvatarHashesByUserID(ctx context.Context, tx pgx.Tx, guildID string, userIDs []string) (map[string]string, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	rows, err := txQueryContext(ctx, tx, `SELECT user_id, avatar_hash FROM avatars_current WHERE guild_id=$1 AND user_id = ANY($2)`, guildID, userIDs)
	if err != nil {
		return nil, fmt.Errorf("queryCurrentAvatarHashesByUserID: %w", err)
	}
	defer rows.Close()

	hashes := make(map[string]string, len(userIDs))
	for rows.Next() {
		var userID, avatarHash string
		if err := rows.Scan(&userID, &avatarHash); err != nil {
			return nil, fmt.Errorf("queryCurrentAvatarHashesByUserID: %w", err)
		}
		hashes[userID] = avatarHash
	}
	return hashes, rows.Err()
}

type avatarHistoryBatch struct {
	GuildID       string
	Snapshots     []members.Snapshot
	CurrentHashes map[string]string
	UpdatedAt     time.Time
}

func insertAvatarHistoryBatch(ctx context.Context, tx pgx.Tx, batch avatarHistoryBatch) error {
	type avatarHistoryRow struct {
		UserID    string
		OldHash   string
		NewHash   string
		ChangedAt time.Time
	}
	rows := make([]avatarHistoryRow, 0, len(batch.Snapshots))
	for _, snapshot := range batch.Snapshots {
		oldHash, ok := batch.CurrentHashes[snapshot.UserID]
		if !ok || oldHash == snapshot.AvatarHash {
			continue
		}
		rows = append(rows, avatarHistoryRow{
			UserID:    snapshot.UserID,
			OldHash:   oldHash,
			NewHash:   snapshot.AvatarHash,
			ChangedAt: batch.UpdatedAt,
		})
	}
	if len(rows) == 0 {
		return nil
	}

	guildIDs := make([]string, len(rows))
	userIDs := make([]string, len(rows))
	oldHashes := make([]string, len(rows))
	newHashes := make([]string, len(rows))
	changedAts := make([]time.Time, len(rows))
	ids := make([]int64, len(rows))

	for i, row := range rows {
		guildIDs[i] = batch.GuildID
		userIDs[i] = row.UserID
		oldHashes[i] = row.OldHash
		newHashes[i] = row.NewHash
		changedAts[i] = row.ChangedAt
		ids[i] = idgen.GenerateID()
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO avatars_history (id, guild_id, user_id, old_hash, new_hash, changed_at)
         SELECT * FROM UNNEST($1::bigint[], $2::text[], $3::text[], $4::text[], $5::text[], $6::timestamptz[])`,
		ids, guildIDs, userIDs, oldHashes, newHashes, changedAts,
	)
	return err
}

func upsertAvatarCurrentBatch(ctx context.Context, tx pgx.Tx, guildID string, snapshots []members.Snapshot, updatedAt time.Time) error {
	if len(snapshots) == 0 {
		return nil
	}
	userIDs := make([]string, len(snapshots))
	avatarHashes := make([]string, len(snapshots))
	updatedAts := make([]time.Time, len(snapshots))

	for i, row := range snapshots {
		userIDs[i] = row.UserID
		avatarHashes[i] = row.AvatarHash
		updatedAts[i] = updatedAt
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO avatars_current (guild_id, user_id, avatar_hash, updated_at)
         SELECT $1::text, * FROM UNNEST($2::text[], $3::text[], $4::timestamptz[])
         ON CONFLICT(guild_id, user_id) DO UPDATE SET avatar_hash=excluded.avatar_hash, updated_at=excluded.updated_at
         WHERE avatars_current.updated_at < excluded.updated_at`,
		guildID, userIDs, avatarHashes, updatedAts,
	)
	return err
}

func deleteRolesForUsersBatch(ctx context.Context, tx pgx.Tx, guildID string, userIDs []string, updatedAt time.Time) error {
	if len(userIDs) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx,
		`UPDATE roles_current SET deleted_at = $3, updated_at = $3 WHERE guild_id=$1 AND user_id = ANY($2::text[]) AND deleted_at IS NULL`,
		guildID, userIDs, updatedAt,
	)
	return err
}

func insertMemberRolesBatch(ctx context.Context, tx pgx.Tx, guildID string, snapshots []members.Snapshot, updatedAt time.Time) error {
	var totalRoles int
	for _, snap := range snapshots {
		totalRoles += len(snap.Roles)
	}
	if totalRoles == 0 {
		return nil
	}

	userIDs := make([]string, 0, totalRoles)
	roleIDs := make([]string, 0, totalRoles)
	updatedAts := make([]time.Time, 0, totalRoles)

	for _, snap := range snapshots {
		for _, roleID := range snap.Roles {
			userIDs = append(userIDs, snap.UserID)
			roleIDs = append(roleIDs, roleID)
			updatedAts = append(updatedAts, updatedAt)
		}
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO roles_current (guild_id, user_id, role_id, updated_at)
         SELECT $1::text, * FROM UNNEST($2::text[], $3::text[], $4::timestamptz[])
         ON CONFLICT(guild_id, user_id, role_id) DO UPDATE SET updated_at=excluded.updated_at, deleted_at=NULL
         WHERE roles_current.deleted_at IS NOT NULL OR roles_current.updated_at < excluded.updated_at - interval '5 minutes'`,
		guildID, userIDs, roleIDs, updatedAts,
	)
	return err
}

func upsertMemberJoinsBatch(ctx context.Context, tx pgx.Tx, guildID string, snapshots []members.Snapshot, seenAt time.Time) error {
	if len(snapshots) == 0 {
		return nil
	}
	userIDs := make([]string, len(snapshots))
	joinedAts := make([]time.Time, len(snapshots))
	seenAts := make([]time.Time, len(snapshots))
	isBots := make([]*bool, len(snapshots))

	for i, row := range snapshots {
		userIDs[i] = row.UserID
		joinedAt := seenAt
		if !row.JoinedAt.IsZero() {
			joinedAt = row.JoinedAt.UTC()
		}
		joinedAts[i] = joinedAt
		seenAts[i] = seenAt

		var isBot *bool
		if row.HasBot {
			b := row.IsBot
			isBot = &b
		}
		isBots[i] = isBot
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO member_joins (guild_id, user_id, joined_at, last_seen_at, is_bot)
         SELECT $1::text, * FROM UNNEST($2::text[], $3::timestamptz[], $4::timestamptz[], $5::boolean[])
         ON CONFLICT(guild_id, user_id) DO UPDATE SET
           joined_at = CASE
             WHEN excluded.joined_at < member_joins.joined_at THEN excluded.joined_at
             ELSE member_joins.joined_at
           END,
           last_seen_at = CASE
             WHEN member_joins.last_seen_at IS NULL OR excluded.last_seen_at > member_joins.last_seen_at THEN excluded.last_seen_at
             ELSE member_joins.last_seen_at
           END,
           is_bot = COALESCE(excluded.is_bot, member_joins.is_bot),
           left_at = NULL
         WHERE excluded.joined_at < member_joins.joined_at
            OR member_joins.last_seen_at IS NULL
            OR excluded.last_seen_at > member_joins.last_seen_at + interval '5 minutes'
            OR member_joins.is_bot IS DISTINCT FROM COALESCE(excluded.is_bot, member_joins.is_bot)
            OR member_joins.left_at IS NOT NULL`,
		guildID, userIDs, joinedAts, seenAts, isBots,
	)
	return err
}

// UpsertMemberJoinContext records the earliest known join time for a member.
func (s *Store) UpsertMemberJoinContext(ctx context.Context, guildID, userID string, joinedAt time.Time) error {
	if guildID == "" || userID == "" || joinedAt.IsZero() {
		return nil
	}
	joinedAt = joinedAt.UTC()
	seenAt := time.Now().UTC()
	_, err := s.db.Exec(
		ctx,
		`INSERT INTO member_joins (guild_id, user_id, joined_at, last_seen_at, left_at)
         VALUES ($1, $2, $3, $4, NULL)
         ON CONFLICT(guild_id, user_id) DO UPDATE SET
           joined_at = CASE
             WHEN excluded.joined_at < member_joins.joined_at THEN excluded.joined_at
             ELSE member_joins.joined_at
           END,
           last_seen_at = CASE
             WHEN member_joins.last_seen_at IS NULL OR excluded.last_seen_at > member_joins.last_seen_at THEN excluded.last_seen_at
             ELSE member_joins.last_seen_at
           END,
           left_at = NULL
         WHERE excluded.joined_at < member_joins.joined_at
            OR member_joins.last_seen_at IS NULL
            OR excluded.last_seen_at > member_joins.last_seen_at + interval '5 minutes'
            OR member_joins.left_at IS NOT NULL`,
		guildID, userID, joinedAt, seenAt,
	)
	return err
}

// UpsertMemberPresenceContext records that a member is currently present in a guild.
func (s *Store) UpsertMemberPresenceContext(ctx context.Context, input members.PresenceInput) error {
	input.GuildID = strings.TrimSpace(input.GuildID)
	input.UserID = strings.TrimSpace(input.UserID)
	if input.GuildID == "" || input.UserID == "" {
		return nil
	}
	if input.SeenAt.IsZero() {
		input.SeenAt = time.Now().UTC()
	} else {
		input.SeenAt = input.SeenAt.UTC()
	}
	if input.JoinedAt.IsZero() {
		input.JoinedAt = input.SeenAt
	} else {
		input.JoinedAt = input.JoinedAt.UTC()
	}

	_, err := s.db.Exec(
		ctx,
		`INSERT INTO member_joins (guild_id, user_id, joined_at, last_seen_at, is_bot, left_at)
         VALUES ($1, $2, $3, $4, $5, NULL)
         ON CONFLICT(guild_id, user_id) DO UPDATE SET
           joined_at = CASE
             WHEN excluded.joined_at < member_joins.joined_at THEN excluded.joined_at
             ELSE member_joins.joined_at
           END,
           last_seen_at = CASE
             WHEN member_joins.last_seen_at IS NULL OR excluded.last_seen_at > member_joins.last_seen_at THEN excluded.last_seen_at
             ELSE member_joins.last_seen_at
           END,
           is_bot = excluded.is_bot,
           left_at = NULL
         WHERE excluded.joined_at < member_joins.joined_at
            OR member_joins.last_seen_at IS NULL
            OR excluded.last_seen_at > member_joins.last_seen_at + interval '5 minutes'
            OR member_joins.is_bot IS DISTINCT FROM excluded.is_bot
            OR member_joins.left_at IS NOT NULL`,
		input.GuildID, input.UserID, input.JoinedAt, input.SeenAt, input.IsBot,
	)
	return err
}

// MemberJoin returns the stored join time for a member, enforcing pgx.ErrNoRows behavior.
func (s *Store) MemberJoin(ctx context.Context, guildID, userID string) (time.Time, bool, error) {
	var jt time.Time
	err := s.db.QueryRow(ctx, `SELECT joined_at FROM member_joins WHERE guild_id=$1 AND user_id=$2`, guildID, userID).Scan(&jt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("Store.MemberJoin: %w", err)
	}
	return jt, true, nil
}

// GetAvatar returns the current avatar hash for a user.
func (s *Store) GetAvatar(ctx context.Context, guildID, userID string) (hash string, updatedAt time.Time, ok bool, err error) {
	err = s.db.QueryRow(ctx, `SELECT avatar_hash, updated_at FROM avatars_current WHERE guild_id=$1 AND user_id=$2`, guildID, userID).Scan(&hash, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, false, nil
		}
		return "", time.Time{}, false, err
	}
	return hash, updatedAt, true, nil
}

// GetActiveGuildMemberStatesContext streams current member states utilizing iter.Seq2, avoiding slice heap allocations.
func (s *Store) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[members.CurrentState, error] {
	return func(yield func(members.CurrentState, error) bool) {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return
		}

		rows, err := s.db.Query(ctx, `
			SELECT mj.user_id, mj.joined_at, mj.last_seen_at, mj.is_bot, rc.role_id
			  FROM member_joins mj
			  LEFT JOIN roles_current rc
			    ON rc.guild_id = mj.guild_id
			   AND rc.user_id = mj.user_id
			   AND rc.deleted_at IS NULL
			 WHERE mj.guild_id = $1
			   AND mj.left_at IS NULL
			 ORDER BY CAST(mj.user_id AS BIGINT), rc.role_id
		`, guildID)
		if err != nil {
			yield(members.CurrentState{}, fmt.Errorf("Store.GetActiveGuildMemberStatesContext: %w", err))
			return
		}
		defer rows.Close()

		var currentState *members.CurrentState

		var (
			userID     string
			joinedAt   time.Time
			lastSeenAt *time.Time
			isBot      *bool
			roleID     *string
		)

		for rows.Next() {
			userID = ""
			joinedAt = time.Time{}
			lastSeenAt = nil
			isBot = nil
			roleID = nil

			if err := rows.Scan(&userID, &joinedAt, &lastSeenAt, &isBot, &roleID); err != nil {
				yield(members.CurrentState{}, fmt.Errorf("Store.GetActiveGuildMemberStatesContext: %w", err))
				return
			}

			if currentState != nil && currentState.UserID != userID {
				if !yield(*currentState, nil) {
					return // Early exit propagates rows.Close() natively via defer
				}
				currentState = nil
			}

			if currentState == nil {
				currentState = &members.CurrentState{
					UserID:   userID,
					JoinedAt: joinedAt.UTC(),
					Active:   true,
				}
				if lastSeenAt != nil {
					currentState.LastSeenAt = lastSeenAt.UTC()
				}
				if isBot != nil {
					currentState.HasBot = true
					currentState.IsBot = *isBot
				}
			}

			if roleID != nil && strings.TrimSpace(*roleID) != "" {
				currentState.Roles = append(currentState.Roles, *roleID)
			}
		}

		if err := rows.Err(); err != nil {
			yield(members.CurrentState{}, fmt.Errorf("Store.GetActiveGuildMemberStatesContext: %w", err))
			return
		}

		if currentState != nil {
			yield(*currentState, nil)
		}
	}
}

// StreamAllGuildMemberRoles streams role sets utilizing iter.Seq2 for memory retention.
func (s *Store) StreamAllGuildMemberRoles(ctx context.Context, guildID string) (iter.Seq2[string, []string], error) {
	rows, err := s.db.Query(ctx, `SELECT user_id, role_id FROM roles_current WHERE guild_id=$1 AND deleted_at IS NULL ORDER BY CAST(user_id AS BIGINT)`, guildID)
	if err != nil {
		return nil, fmt.Errorf("Store.StreamAllGuildMemberRoles: %w", err)
	}

	return func(yield func(string, []string) bool) {
		defer rows.Close()

		var currentUser string
		var currentRoles []string
		var userID, roleID string

		for rows.Next() {
			if err := rows.Scan(&userID, &roleID); err != nil {
				return
			}
			if currentUser != "" && currentUser != userID {
				if !yield(currentUser, currentRoles) {
					return
				}
				currentRoles = currentRoles[:0] // Reuse slice backing array to prevent allocations
			}
			currentUser = userID
			currentRoles = append(currentRoles, roleID)
		}
		if currentUser != "" {
			yield(currentUser, currentRoles)
		}
	}, nil
}

// MarkMemberLeftContext marks a member as having left the guild.
func (s *Store) MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error {
	_, err := s.db.Exec(ctx, `UPDATE member_joins SET left_at = $1 WHERE guild_id = $2 AND user_id = $3 AND left_at IS NULL`, at, guildID, userID)
	return err
}

// UpsertMemberRoles updates a member's roles.
func (s *Store) UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error {
	_, err := s.db.Exec(context.Background(), `
		UPDATE member_current
		SET roles = $1,
			updated_at = $2
		WHERE guild_id = $3 AND user_id = $4
	`, roles, at, guildID, userID)
	return err
}

```

// === FILE: pkg/storage/postgres/members_test.go ===
```go
package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/members"
)

func init() {
	idgen.Init(1)
}

func TestStore_Iterators_EarlyExitCursorClosure(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	rows := pgxmock.NewRows([]string{"user_id", "role_id"}).
		AddRow("1", "role1").
		AddRow("2", "role2").
		AddRow("3", "role3").
		AddRow("4", "role4")

	mock.ExpectQuery("SELECT user_id, role_id FROM roles_current").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	iterSeq, err := store.StreamAllGuildMemberRoles(context.Background(), "guild1")
	if err != nil {
		t.Fatalf("failed to stream: %v", err)
	}

	count := 0
	for _, _ = range iterSeq {
		count++
		if count == 3 {
			break
		}
	}

	if count != 3 {
		t.Errorf("expected 3 records processed, got %d", count)
	}
}

func BenchmarkStore_Iterators_CompleteDrain(b *testing.B) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		b.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		rows := pgxmock.NewRows([]string{"user_id", "role_id"})
		for j := 0; j < 100; j++ {
			rows.AddRow("1", "role1")
		}
		mock.ExpectQuery("SELECT user_id, role_id FROM roles_current").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
		iterSeq, _ := store.StreamAllGuildMemberRoles(context.Background(), "guild1")
		b.StartTimer()

		for _, _ = range iterSeq {
		}
	}
}

func TestStore_Context_ExecutionBoundaryTimeout(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	mock.ExpectExec("INSERT INTO member_joins").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1)).WillDelayFor(10 * time.Millisecond)

	err = store.UpsertMemberPresenceContext(ctx, members.PresenceInput{
		GuildID: "123",
		UserID:  "456",
	})

	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestStore_Context_StructuralMisalignment(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id, avatar_hash").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"user_id", "avatar_hash"}).AddRow("1", "hash"))
	mock.ExpectExec("INSERT INTO avatars_history").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(&pgconn.PgError{
		Code:    "2202E",
		Message: "arrays must have same bounds",
	})
	mock.ExpectRollback()

	snapshots := []members.Snapshot{
		{UserID: "1", HasAvatar: true, AvatarHash: "hash1"},
		{UserID: "2", HasAvatar: true, AvatarHash: "hash2"},
	}

	err = store.UpsertGuildMemberSnapshotsContext(context.Background(), "guild1", snapshots, time.Now())
	if err == nil || !strings.Contains(err.Error(), "arrays must have same bounds") {
		t.Errorf("expected structural misalignment error, got %v", err)
	}
}

func TestStore_Context_UnaryMissingState(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT joined_at FROM member_joins").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	_, ok, err := store.MemberJoin(context.Background(), "invalid", "invalid")
	if err != nil {
		t.Errorf("expected nil error on ErrNoRows, got %v", err)
	}
	if ok {
		t.Errorf("expected ok to be false")
	}

	mock.ExpectQuery("SELECT avatar_hash, updated_at FROM avatars_current").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)
	_, _, ok, err = store.GetAvatar(context.Background(), "invalid", "invalid")
	if err != nil {
		t.Errorf("expected nil error on ErrNoRows, got %v", err)
	}
	if ok {
		t.Errorf("expected ok to be false")
	}
}

var _ DB = (*pgxpool.Pool)(nil)

func TestStore_Members_Idempotency_And_Temporal_Precedence(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	snapshots := []members.Snapshot{
		{
			UserID:   "user1",
			JoinedAt: time.Now().Add(-10 * time.Hour),
			HasBot:   true,
			IsBot:    false,
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO member_joins").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	mock.ExpectRollback()

	err = store.UpsertGuildMemberSnapshotsContext(context.Background(), "guild1", snapshots, time.Now())
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestStore_Members_UserPreferences(t *testing.T) {
	t.Parallel()
	t.Run("success GetUserPreferences", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		rows := pgxmock.NewRows([]string{"user_id", "theme", "timezone"}).
			AddRow("u1", "dark", "EST")

		mock.ExpectQuery(`SELECT user_id, theme, timezone FROM user_preferences`).
			WithArgs("u1").
			WillReturnRows(rows)

		prefs, err := store.GetUserPreferences(context.Background(), "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prefs.Theme != "dark" || prefs.Timezone != "EST" {
			t.Errorf("unexpected preferences: %+v", prefs)
		}
	})

	t.Run("defaults when GetUserPreferences ErrNoRows", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT user_id, theme, timezone FROM user_preferences`).
			WithArgs("u1").
			WillReturnError(pgx.ErrNoRows)

		prefs, err := store.GetUserPreferences(context.Background(), "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prefs.Theme != "system" || prefs.Timezone != "UTC" {
			t.Errorf("unexpected preferences: %+v", prefs)
		}
	})

	t.Run("error GetUserPreferences", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT user_id, theme, timezone FROM user_preferences`).
			WithArgs("u1").
			WillReturnError(errors.New("db error"))

		_, err := store.GetUserPreferences(context.Background(), "u1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("success UpdateUserPreferences", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		prefs := &members.UserPreferences{
			UserID:   "u1",
			Theme:    "light",
			Timezone: "PST",
		}

		mock.ExpectExec(`INSERT INTO user_preferences`).
			WithArgs("u1", "light", "PST").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.UpdateUserPreferences(context.Background(), prefs)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestStore_Members_UpsertMemberJoinContext(t *testing.T) {
	t.Parallel()
	t.Run("empty validation", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.UpsertMemberJoinContext(context.Background(), "", "", time.Now())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		mock.ExpectExec(`INSERT INTO member_joins`).
			WithArgs("g1", "u1", now.UTC(), pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.UpsertMemberJoinContext(context.Background(), "g1", "u1", now)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestStore_Members_GetActiveGuildMemberStatesContext(t *testing.T) {
	t.Parallel()
	t.Run("empty validation", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		seq := store.GetActiveGuildMemberStatesContext(context.Background(), "")
		for range seq {
			t.Error("expected no output")
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		lastSeen := now.Add(time.Minute)
		isBot := false
		role1 := "role1"
		role2 := "role2"

		rows := pgxmock.NewRows([]string{"user_id", "joined_at", "last_seen_at", "is_bot", "role_id"}).
			AddRow("1", now, &lastSeen, &isBot, &role1).
			AddRow("1", now, &lastSeen, &isBot, &role2).
			AddRow("2", now, nil, nil, nil)

		mock.ExpectQuery(`SELECT mj\.user_id`).
			WithArgs("g1").
			WillReturnRows(rows)

		seq := store.GetActiveGuildMemberStatesContext(context.Background(), "g1")
		var results []members.CurrentState
		for state, err := range seq {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			results = append(results, state)
		}

		if len(results) != 2 {
			t.Fatalf("expected 2 states, got %d", len(results))
		}
		if results[0].UserID != "1" || len(results[0].Roles) != 2 || results[0].Roles[0] != "role1" || results[0].Roles[1] != "role2" {
			t.Errorf("unexpected state 1: %+v", results[0])
		}
		if results[1].UserID != "2" || len(results[1].Roles) != 0 || results[1].HasBot {
			t.Errorf("unexpected state 2: %+v", results[1])
		}
	})

	t.Run("query error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT mj\.user_id`).
			WillReturnError(errors.New("db error"))

		seq := store.GetActiveGuildMemberStatesContext(context.Background(), "g1")
		for _, err := range seq {
			if err == nil {
				t.Error("expected error from iterator")
			}
		}
	})
}

func TestStore_Members_MarkMemberLeftContext(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	now := time.Now()
	mock.ExpectExec(`UPDATE member_joins SET left_at =`).
		WithArgs(now, "g1", "u1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.MarkMemberLeftContext(context.Background(), "g1", "u1", now)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStore_Members_UpsertMemberRoles(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	now := time.Now()
	roles := []string{"role1", "role2"}

	mock.ExpectExec(`UPDATE member_current`).
		WithArgs(roles, now, "g1", "u1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.UpsertMemberRoles("g1", "u1", roles, now)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

```

// === FILE: pkg/storage/postgres/messages.go ===
```go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/small-frappuccino/discordcore/pkg/messages"
)

// UpsertMessage inserts or updates a message record transactionally.
func (s *Store) UpsertMessage(m messages.Record) error {
	var expires any
	if m.HasExpiry {
		expires = m.ExpiresAt.UTC()
	}

	_, err := s.db.Exec(context.Background(),
		`INSERT INTO messages (guild_id, message_id, channel_id, author_id, author_username, author_avatar, content, cached_at, expires_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
         ON CONFLICT(guild_id, message_id) DO UPDATE SET
           channel_id=excluded.channel_id,
           author_id=excluded.author_id,
           author_username=excluded.author_username,
           author_avatar=excluded.author_avatar,
           content=excluded.content,
           cached_at=excluded.cached_at,
           expires_at=excluded.expires_at`,
		m.GuildID, m.MessageID, m.ChannelID, m.AuthorID, m.AuthorUsername, m.AuthorAvatar, m.Content, m.CachedAt.UTC(), expires,
	)
	return err
}

// UpsertMessagesContext upserts a batch of cached messages.
func (s *Store) UpsertMessagesContext(ctx context.Context, records []messages.Record) error {
	normalized := normalizeMessageRecords(records)
	if len(normalized) == 0 {
		return nil
	}

	guildIDs := make([]string, len(normalized))
	messageIDs := make([]string, len(normalized))
	channelIDs := make([]string, len(normalized))
	authorIDs := make([]string, len(normalized))
	authorUsernames := make([]string, len(normalized))
	authorAvatars := make([]string, len(normalized))
	contents := make([]string, len(normalized))
	cachedAts := make([]time.Time, len(normalized))
	expiresAts := make([]*time.Time, len(normalized))

	for i, record := range normalized {
		guildIDs[i] = record.GuildID
		messageIDs[i] = record.MessageID
		channelIDs[i] = record.ChannelID
		authorIDs[i] = record.AuthorID
		authorUsernames[i] = record.AuthorUsername
		authorAvatars[i] = record.AuthorAvatar
		contents[i] = record.Content
		cachedAts[i] = record.CachedAt.UTC()
		if record.HasExpiry {
			t := record.ExpiresAt.UTC()
			expiresAts[i] = &t
		}
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO messages (guild_id, message_id, channel_id, author_id, author_username, author_avatar, content, cached_at, expires_at)
         SELECT * FROM UNNEST($1::text[], $2::text[], $3::text[], $4::text[], $5::text[], $6::text[], $7::text[], $8::timestamptz[], $9::timestamptz[])
         ON CONFLICT(guild_id, message_id) DO UPDATE SET
           channel_id=excluded.channel_id,
           author_id=excluded.author_id,
           author_username=excluded.author_username,
           author_avatar=excluded.author_avatar,
           content=excluded.content,
           cached_at=excluded.cached_at,
           expires_at=excluded.expires_at`,
		guildIDs, messageIDs, channelIDs, authorIDs, authorUsernames, authorAvatars, contents, cachedAts, expiresAts,
	)
	return err
}

func normalizeMessageRecords(records []messages.Record) []messages.Record {
	if len(records) == 0 {
		return nil
	}
	order := make([]string, 0, len(records))
	byKey := make(map[string]messages.Record, len(records))
	for _, record := range records {
		guildID := strings.TrimSpace(record.GuildID)
		messageID := strings.TrimSpace(record.MessageID)
		if guildID == "" || messageID == "" {
			continue
		}
		record.GuildID = guildID
		record.MessageID = messageID
		key := guildID + ":" + messageID
		if _, ok := byKey[key]; !ok {
			order = append(order, key)
		}
		byKey[key] = record
	}

	normalized := make([]messages.Record, 0, len(order))
	for _, key := range order {
		normalized = append(normalized, byKey[key])
	}
	return normalized
}

// GetMessage returns a non-expired message if present.
func (s *Store) GetMessage(ctx context.Context, guildID, messageID string) (*messages.Record, error) {
	row := s.db.QueryRow(ctx,
		`SELECT guild_id, message_id, channel_id, author_id, author_username, author_avatar, content, cached_at, expires_at
         FROM messages
         WHERE guild_id=$1 AND message_id=$2 AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`,
		guildID, messageID,
	)

	var rec messages.Record
	var expires *time.Time
	if err := row.Scan(&rec.GuildID, &rec.MessageID, &rec.ChannelID, &rec.AuthorID, &rec.AuthorUsername, &rec.AuthorAvatar, &rec.Content, &rec.CachedAt, &expires); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if expires != nil {
		rec.HasExpiry = true
		rec.ExpiresAt = *expires
	}
	return &rec, nil
}

// DeleteMessagesContext removes a batch of message records via UNNEST.
func (s *Store) DeleteMessagesContext(ctx context.Context, keys []messages.DeleteKey) error {
	normalized := normalizeMessageDeleteKeys(keys)
	if len(normalized) == 0 {
		return nil
	}

	guildIDs := make([]string, len(normalized))
	messageIDs := make([]string, len(normalized))

	for i, key := range normalized {
		guildIDs[i] = key.GuildID
		messageIDs[i] = key.MessageID
	}

	_, err := s.db.Exec(ctx,
		`DELETE FROM messages 
         USING UNNEST($1::text[], $2::text[]) AS doomed(guild_id, message_id) 
         WHERE messages.guild_id = doomed.guild_id AND messages.message_id = doomed.message_id`,
		guildIDs, messageIDs,
	)
	return err
}

func normalizeMessageDeleteKeys(keys []messages.DeleteKey) []messages.DeleteKey {
	if len(keys) == 0 {
		return nil
	}
	order := make([]string, 0, len(keys))
	byKey := make(map[string]messages.DeleteKey, len(keys))
	for _, key := range keys {
		key.GuildID = strings.TrimSpace(key.GuildID)
		key.MessageID = strings.TrimSpace(key.MessageID)
		if key.GuildID == "" || key.MessageID == "" {
			continue
		}
		composite := key.GuildID + ":" + key.MessageID
		if _, ok := byKey[composite]; !ok {
			order = append(order, composite)
		}
		byKey[composite] = key
	}

	normalized := make([]messages.DeleteKey, 0, len(order))
	for _, composite := range order {
		normalized = append(normalized, byKey[composite])
	}
	return normalized
}

// InsertMessageVersionsMixedBatchContext inserts a batch of message history rows.
func (s *Store) InsertMessageVersionsMixedBatchContext(ctx context.Context, versions []messages.Version) (err error) {
	normalized := normalizeMessageVersions(versions)
	if len(normalized) == 0 {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin message versions tx: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	assigned, err := reserveMessageVersionRangesTx(ctx, tx, normalized)
	if err != nil {
		return fmt.Errorf("Store.InsertMessageVersionsMixedBatchContext: %w", err)
	}
	if err := insertMessageHistoryBatchTx(ctx, tx, assigned); err != nil {
		return fmt.Errorf("Store.InsertMessageVersionsMixedBatchContext: %w", err)
	}
	return tx.Commit(ctx)
}

func normalizeMessageVersions(versions []messages.Version) []messages.Version {
	if len(versions) == 0 {
		return nil
	}
	normalized := make([]messages.Version, 0, len(versions))
	for _, version := range versions {
		if strings.TrimSpace(version.GuildID) == "" || strings.TrimSpace(version.MessageID) == "" || strings.TrimSpace(version.EventType) == "" {
			continue
		}
		if version.CreatedAt.IsZero() {
			version.CreatedAt = time.Now().UTC()
		} else {
			version.CreatedAt = version.CreatedAt.UTC()
		}
		normalized = append(normalized, version)
	}
	return normalized
}

type messageVersionGroup struct {
	GuildID   string
	MessageID string
	Indexes   []int
}

func reserveMessageVersionRangesTx(ctx context.Context, tx pgx.Tx, versions []messages.Version) ([]messages.Version, error) {
	if len(versions) == 0 {
		return nil, nil
	}

	assigned := append([]messages.Version(nil), versions...)
	groups := groupMessageVersions(assigned)

	for _, group := range groups {
		lastVersion, err := lockMessageVersionCounterTx(ctx, tx, group.GuildID, group.MessageID)
		if err != nil {
			return nil, fmt.Errorf("reserveMessageVersionRangesTx: %w", err)
		}
		nextVersion := lastVersion
		for _, idx := range group.Indexes {
			if assigned[idx].Version > 0 {
				if assigned[idx].Version > nextVersion {
					nextVersion = assigned[idx].Version
				}
				continue
			}
			nextVersion++
			assigned[idx].Version = nextVersion
		}
		if nextVersion != lastVersion {
			if err := updateMessageVersionCounterTx(ctx, tx, group.GuildID, group.MessageID, nextVersion); err != nil {
				return nil, fmt.Errorf("reserveMessageVersionRangesTx: %w", err)
			}
		}
	}
	return assigned, nil
}

func groupMessageVersions(versions []messages.Version) []messageVersionGroup {
	if len(versions) == 0 {
		return nil
	}
	order := make([]string, 0, len(versions))
	groups := make(map[string]*messageVersionGroup, len(versions))
	for idx, version := range versions {
		key := strings.TrimSpace(version.GuildID) + ":" + strings.TrimSpace(version.MessageID)
		group, ok := groups[key]
		if !ok {
			group = &messageVersionGroup{GuildID: strings.TrimSpace(version.GuildID), MessageID: strings.TrimSpace(version.MessageID)}
			groups[key] = group
			order = append(order, key)
		}
		group.Indexes = append(group.Indexes, idx)
	}

	grouped := make([]messageVersionGroup, 0, len(order))
	for _, key := range order {
		grouped = append(grouped, *groups[key])
	}
	return grouped
}

func lockMessageVersionCounterTx(ctx context.Context, tx pgx.Tx, guildID, messageID string) (int, error) {
	if guildID == "" || messageID == "" {
		return 0, nil
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO message_version_counters (guild_id, message_id, last_version)
         VALUES ($1, $2, COALESCE((SELECT MAX(version) FROM messages_history WHERE guild_id=$3 AND message_id=$4), 0))
         ON CONFLICT (guild_id, message_id) DO NOTHING`,
		guildID, messageID, guildID, messageID); err != nil {
		return 0, fmt.Errorf("ensure message version counter: %w", err)
	}

	var lastVersion int64
	if err := tx.QueryRow(ctx, `SELECT last_version FROM message_version_counters WHERE guild_id=$1 AND message_id=$2 FOR UPDATE`, guildID, messageID).Scan(&lastVersion); err != nil {
		return 0, fmt.Errorf("lock message version counter: %w", err)
	}
	return int(lastVersion), nil
}

func updateMessageVersionCounterTx(ctx context.Context, tx pgx.Tx, guildID, messageID string, lastVersion int) error {
	if guildID == "" || messageID == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE message_version_counters SET last_version=$1 WHERE guild_id=$2 AND message_id=$3`, lastVersion, guildID, messageID); err != nil {
		return fmt.Errorf("update message version counter: %w", err)
	}
	return nil
}

func insertMessageHistoryBatchTx(ctx context.Context, tx pgx.Tx, versions []messages.Version) error {
	if len(versions) == 0 {
		return nil
	}
	guildIDs := make([]string, len(versions))
	messageIDs := make([]string, len(versions))
	channelIDs := make([]string, len(versions))
	authorIDs := make([]string, len(versions))
	versionNums := make([]int, len(versions))
	eventTypes := make([]string, len(versions))
	contents := make([]string, len(versions))
	attachments := make([]int, len(versions))
	embedsCounts := make([]int, len(versions))
	stickers := make([]int, len(versions))
	createdAts := make([]time.Time, len(versions))

	for i, v := range versions {
		guildIDs[i] = v.GuildID
		messageIDs[i] = v.MessageID
		channelIDs[i] = v.ChannelID
		authorIDs[i] = v.AuthorID
		versionNums[i] = v.Version
		eventTypes[i] = v.EventType
		contents[i] = v.Content
		attachments[i] = v.Attachments
		embedsCounts[i] = v.Embeds
		stickers[i] = v.Stickers
		createdAts[i] = v.CreatedAt.UTC()
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO messages_history
         (guild_id, message_id, channel_id, author_id, version, event_type, content, attachments, embeds_count, stickers, created_at)
         SELECT * FROM UNNEST($1::text[], $2::text[], $3::text[], $4::text[], $5::int[], $6::text[], $7::text[], $8::int[], $9::int[], $10::int[], $11::timestamptz[])
         ON CONFLICT(guild_id, message_id, version) DO NOTHING`,
		guildIDs, messageIDs, channelIDs, authorIDs, versionNums, eventTypes, contents, attachments, embedsCounts, stickers, createdAts,
	)
	return err
}

// CleanupExpiredMessages deletes all expired messages from the cache.
func (s *Store) CleanupExpiredMessages() error {
	_, err := s.db.Exec(context.Background(), `DELETE FROM messages WHERE expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP`)
	return err
}

// IncrementDailyMessageCountsContext increments the daily message counts for multiple guilds.
func (s *Store) IncrementDailyMessageCountsContext(ctx context.Context, deltas []messages.DailyCountDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	for _, delta := range deltas {
		// NOTE: daily_message_metrics actually doesn't have channel_id or user_id in the simple schema, but
		// we accept them here to satisfy the struct used by callers.
		_, err := s.db.Exec(ctx, `
			INSERT INTO daily_message_metrics (guild_id, date, count)
			VALUES ($1, $2, $3)
			ON CONFLICT(guild_id, date) DO UPDATE SET count = daily_message_metrics.count + $3
		`, delta.GuildID, delta.Day, delta.Count)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteMessage deletes a message from the store.
func (s *Store) DeleteMessage(ctx context.Context, guildID, messageID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM messages WHERE guild_id = $1 AND message_id = $2`, guildID, messageID)
	return err
}

// InsertMessageVersion records a new version of a message.
func (s *Store) InsertMessageVersion(ctx context.Context, v messages.Version) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO messages_history (guild_id, message_id, channel_id, author_id, version, event_type, content, attachments, embeds, stickers, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, v.GuildID, v.MessageID, v.ChannelID, v.AuthorID, v.Version, v.EventType, v.Content, v.Attachments, v.Embeds, v.Stickers, v.CreatedAt)
	return err
}

// IncrementDailyMessageCount increments the daily message count for a single guild.
func (s *Store) IncrementDailyMessageCount(ctx context.Context, guildID string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO daily_message_metrics (guild_id, date, count)
		VALUES ($1, CURRENT_DATE, 1)
		ON CONFLICT(guild_id, date) DO UPDATE SET count = daily_message_metrics.count + 1
	`, guildID)
	return err
}

```

// === FILE: pkg/storage/postgres/messages_test.go ===
```go
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/messages"
)

func TestStore_Messages_UpsertMessage(t *testing.T) {
	t.Parallel()
	t.Run("with expiry", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create mock: %v", err)
		}
		defer mock.Close()

		store, _ := NewStore(mock, nil)
		now := time.Now()
		expiry := now.Add(time.Hour)

		rec := messages.Record{
			GuildID:        "123",
			MessageID:      "456",
			ChannelID:      "789",
			AuthorID:       "999",
			AuthorUsername: "username",
			AuthorAvatar:   "avatar",
			Content:        "hello",
			CachedAt:       now,
			ExpiresAt:      expiry,
			HasExpiry:      true,
		}

		mock.ExpectExec(`INSERT INTO messages`).
			WithArgs("123", "456", "789", "999", "username", "avatar", "hello", now.UTC(), expiry.UTC()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = store.UpsertMessage(rec)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("without expiry", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create mock: %v", err)
		}
		defer mock.Close()

		store, _ := NewStore(mock, nil)
		now := time.Now()

		rec := messages.Record{
			GuildID:        "123",
			MessageID:      "456",
			ChannelID:      "789",
			AuthorID:       "999",
			AuthorUsername: "username",
			AuthorAvatar:   "avatar",
			Content:        "hello",
			CachedAt:       now,
			HasExpiry:      false,
		}

		mock.ExpectExec(`INSERT INTO messages`).
			WithArgs("123", "456", "789", "999", "username", "avatar", "hello", now.UTC(), nil).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = store.UpsertMessage(rec)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create mock: %v", err)
		}
		defer mock.Close()

		store, _ := NewStore(mock, nil)
		mock.ExpectExec(`INSERT INTO messages`).
			WillReturnError(errors.New("db error"))

		err = store.UpsertMessage(messages.Record{})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_UpsertMessagesContext(t *testing.T) {
	t.Parallel()
	t.Run("empty slice", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.UpsertMessagesContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error on nil: %v", err)
		}
	})

	t.Run("with records and validation", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		expiry := now.Add(time.Hour)

		records := []messages.Record{
			{GuildID: "123", MessageID: "456", ChannelID: "ch1", CachedAt: now, ExpiresAt: expiry, HasExpiry: true},
			// Duplicate, should be deduplicated
			{GuildID: "123", MessageID: "456", ChannelID: "ch1_dup", CachedAt: now, ExpiresAt: expiry, HasExpiry: true},
			// Invalid keys, should be filtered
			{GuildID: "", MessageID: "invalid"},
			{GuildID: "invalid", MessageID: ""},
			// Another valid one without expiry
			{GuildID: "789", MessageID: "101", ChannelID: "ch2", CachedAt: now},
		}

		expectedGuilds := []string{"123", "789"}
		expectedMessages := []string{"456", "101"}
		expectedChannels := []string{"ch1_dup", "ch2"}
		expectedCachedAts := []time.Time{now.UTC(), now.UTC()}
		expiryTime := expiry.UTC()
		expectedExpiresAts := []*time.Time{&expiryTime, nil}

		mock.ExpectExec(`INSERT INTO messages`).
			WithArgs(expectedGuilds, expectedMessages, expectedChannels, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), expectedCachedAts, expectedExpiresAts).
			WillReturnResult(pgxmock.NewResult("INSERT", 2))

		err := store.UpsertMessagesContext(context.Background(), records)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`INSERT INTO messages`).
			WillReturnError(errors.New("db exec error"))

		err := store.UpsertMessagesContext(context.Background(), []messages.Record{{GuildID: "123", MessageID: "456"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_GetMessage(t *testing.T) {
	t.Parallel()
	t.Run("found with expiry", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		expiry := now.Add(time.Hour)

		rows := pgxmock.NewRows([]string{"guild_id", "message_id", "channel_id", "author_id", "author_username", "author_avatar", "content", "cached_at", "expires_at"}).
			AddRow("123", "456", "789", "999", "user", "avatar", "hello", now, &expiry)

		mock.ExpectQuery(`SELECT guild_id, message_id`).
			WithArgs("123", "456").
			WillReturnRows(rows)

		rec, err := store.GetMessage(context.Background(), "123", "456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec == nil {
			t.Fatal("expected record, got nil")
		}
		if rec.GuildID != "123" || !rec.HasExpiry || !rec.ExpiresAt.Equal(expiry) {
			t.Errorf("unexpected record contents: %+v", rec)
		}
	})

	t.Run("found without expiry", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()

		rows := pgxmock.NewRows([]string{"guild_id", "message_id", "channel_id", "author_id", "author_username", "author_avatar", "content", "cached_at", "expires_at"}).
			AddRow("123", "456", "789", "999", "user", "avatar", "hello", now, nil)

		mock.ExpectQuery(`SELECT guild_id, message_id`).
			WithArgs("123", "456").
			WillReturnRows(rows)

		rec, err := store.GetMessage(context.Background(), "123", "456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec == nil {
			t.Fatal("expected record, got nil")
		}
		if rec.HasExpiry {
			t.Error("expected HasExpiry to be false")
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT guild_id, message_id`).
			WithArgs("123", "456").
			WillReturnError(pgx.ErrNoRows)

		rec, err := store.GetMessage(context.Background(), "123", "456")
		if err != nil {
			t.Errorf("unexpected error on ErrNoRows: %v", err)
		}
		if rec != nil {
			t.Errorf("expected nil record, got: %+v", rec)
		}
	})

	t.Run("scan error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT guild_id, message_id`).
			WithArgs("123", "456").
			WillReturnError(errors.New("scan error"))

		_, err := store.GetMessage(context.Background(), "123", "456")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_DeleteMessagesContext(t *testing.T) {
	t.Parallel()
	t.Run("empty keys", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.DeleteMessagesContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error on nil keys: %v", err)
		}
	})

	t.Run("valid keys and duplicates", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		keys := []messages.DeleteKey{
			{GuildID: "123", MessageID: "456"},
			{GuildID: "123", MessageID: "456"}, // Duplicate
			{GuildID: "", MessageID: "invalid"},
			{GuildID: "789", MessageID: "101"},
		}

		mock.ExpectExec(`DELETE FROM messages`).
			WithArgs([]string{"123", "789"}, []string{"456", "101"}).
			WillReturnResult(pgxmock.NewResult("DELETE", 2))

		err := store.DeleteMessagesContext(context.Background(), keys)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`DELETE FROM messages`).
			WillReturnError(errors.New("db error"))

		err := store.DeleteMessagesContext(context.Background(), []messages.DeleteKey{{GuildID: "123", MessageID: "456"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_InsertMessageVersionsMixedBatchContext(t *testing.T) {
	t.Parallel()
	t.Run("empty versions", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error on empty versions: %v", err)
		}
	})

	t.Run("begin tx error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin().WillReturnError(errors.New("begin error"))

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), []messages.Version{{GuildID: "g", MessageID: "m", EventType: "e"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("lock counter error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO message_version_counters`).
			WillReturnError(errors.New("insert counter error"))
		mock.ExpectRollback()

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), []messages.Version{{GuildID: "g", MessageID: "m", EventType: "e"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("successful reservation and insertion", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		versions := []messages.Version{
			{GuildID: "123", MessageID: "456", ChannelID: "ch1", AuthorID: "a1", EventType: "edit", Content: "hello v1"},
			{GuildID: "123", MessageID: "456", ChannelID: "ch1", AuthorID: "a1", EventType: "edit", Content: "hello v2"},
			// Invalid one
			{GuildID: "", MessageID: "invalid", EventType: "edit"},
		}

		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO message_version_counters`).
			WithArgs("123", "456", "123", "456").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		mock.ExpectQuery(`SELECT last_version FROM message_version_counters`).
			WithArgs("123", "456").
			WillReturnRows(pgxmock.NewRows([]string{"last_version"}).AddRow(int64(2)))

		mock.ExpectExec(`UPDATE message_version_counters`).
			WithArgs(4, "123", "456").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		mock.ExpectExec(`INSERT INTO messages_history`).
			WithArgs(
				[]string{"123", "123"},
				[]string{"456", "456"},
				[]string{"ch1", "ch1"},
				[]string{"a1", "a1"},
				[]int{3, 4},
				[]string{"edit", "edit"},
				[]string{"hello v1", "hello v2"},
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 2))

		mock.ExpectCommit()
		mock.ExpectRollback()

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), versions)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("failed rollback scenario", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO message_version_counters`).
			WillReturnError(errors.New("db error"))
		mock.ExpectRollback().WillReturnError(errors.New("rollback failed"))

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), []messages.Version{{GuildID: "123", MessageID: "456", EventType: "edit"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_CleanupExpiredMessages(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`DELETE FROM messages WHERE expires_at IS NOT NULL`).
			WillReturnResult(pgxmock.NewResult("DELETE", 5))

		err := store.CleanupExpiredMessages()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`DELETE FROM messages WHERE expires_at IS NOT NULL`).
			WillReturnError(errors.New("cleanup error"))

		err := store.CleanupExpiredMessages()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_IncrementDailyMessageCountsContext(t *testing.T) {
	t.Parallel()
	t.Run("empty deltas", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.IncrementDailyMessageCountsContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error on empty deltas: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		deltas := []messages.DailyCountDelta{
			{GuildID: "123", Day: now, Count: 5},
			{GuildID: "456", Day: now, Count: 10},
		}

		mock.ExpectExec(`INSERT INTO daily_message_metrics`).
			WithArgs("123", now, 5).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		mock.ExpectExec(`INSERT INTO daily_message_metrics`).
			WithArgs("456", now, 10).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.IncrementDailyMessageCountsContext(context.Background(), deltas)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		deltas := []messages.DailyCountDelta{
			{GuildID: "123", Day: time.Now(), Count: 5},
		}

		mock.ExpectExec(`INSERT INTO daily_message_metrics`).
			WillReturnError(errors.New("db error"))

		err := store.IncrementDailyMessageCountsContext(context.Background(), deltas)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_DeleteMessage(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectExec(`DELETE FROM messages WHERE guild_id = \$1 AND message_id = \$2`).
		WithArgs("123", "456").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := store.DeleteMessage(context.Background(), "123", "456")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStore_Messages_InsertMessageVersion(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	v := messages.Version{
		GuildID:     "123",
		MessageID:   "456",
		ChannelID:   "789",
		AuthorID:    "999",
		Version:     2,
		EventType:   "edit",
		Content:     "new text",
		Attachments: 0,
		Embeds:      1,
		Stickers:    0,
		CreatedAt:   time.Now(),
	}

	mock.ExpectExec(`INSERT INTO messages_history`).
		WithArgs(v.GuildID, v.MessageID, v.ChannelID, v.AuthorID, v.Version, v.EventType, v.Content, v.Attachments, v.Embeds, v.Stickers, v.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.InsertMessageVersion(context.Background(), v)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStore_Messages_IncrementDailyMessageCount(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectExec(`INSERT INTO daily_message_metrics`).
		WithArgs("123").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.IncrementDailyMessageCount(context.Background(), "123")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

```

// === FILE: pkg/storage/postgres/moderation.go ===
```go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/moderation"
)

// NextModerationCaseNumber atomically increments and returns the next case number.
func (s *Store) NextModerationCaseNumber(ctx context.Context, guildID string) (int64, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return 0, fmt.Errorf("guildID is empty")
	}

	var next int64
	err := s.db.QueryRow(ctx,
		`INSERT INTO moderation_cases (guild_id, last_case_number)
         VALUES ($1, 1)
         ON CONFLICT(guild_id) DO UPDATE
         SET last_case_number = moderation_cases.last_case_number + 1
         RETURNING last_case_number`,
		guildID,
	).Scan(&next)
	if err != nil {
		return 0, err
	}
	return next, nil
}

// CreateModerationWarning creates a moderation warning transactionally.
func (s *Store) CreateModerationWarning(ctx context.Context, guildID, userID, moderatorID, reason string, createdAt time.Time) (warning moderation.Warning, err error) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	moderatorID = strings.TrimSpace(moderatorID)
	reason = strings.TrimSpace(reason)
	if guildID == "" || userID == "" || moderatorID == "" || reason == "" {
		return moderation.Warning{}, fmt.Errorf("missing required fields for warning")
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	} else {
		createdAt = createdAt.UTC()
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return moderation.Warning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	var caseNumber int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO moderation_cases (guild_id, last_case_number)
         VALUES ($1, 1)
         ON CONFLICT(guild_id) DO UPDATE
         SET last_case_number = moderation_cases.last_case_number + 1
         RETURNING last_case_number`,
		guildID,
	).Scan(&caseNumber); err != nil {
		return moderation.Warning{}, err
	}

	warning = moderation.Warning{
		GuildID:     guildID,
		UserID:      userID,
		CaseNumber:  caseNumber,
		ModeratorID: moderatorID,
		Reason:      reason,
		CreatedAt:   createdAt,
	}

	if err := tx.QueryRow(ctx,
		`INSERT INTO moderation_warnings (id, guild_id, user_id, case_number, moderator_id, reason, created_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7)
         RETURNING id, created_at`,
		idgen.GenerateID(), warning.GuildID, warning.UserID, warning.CaseNumber, warning.ModeratorID, warning.Reason, warning.CreatedAt,
	).Scan(&warning.ID, &warning.CreatedAt); err != nil {
		return moderation.Warning{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return moderation.Warning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}
	return warning, nil
}

// ListModerationWarnings lists moderation warnings utilizing iter.Seq2.
func (s *Store) ListModerationWarnings(ctx context.Context, guildID, userID string, limit int) iter.Seq2[moderation.Warning, error] {
	return func(yield func(moderation.Warning, error) bool) {
		guildID = strings.TrimSpace(guildID)
		userID = strings.TrimSpace(userID)
		if guildID == "" || userID == "" {
			return
		}
		if limit <= 0 {
			limit = 5
		}
		if limit > 25 {
			limit = 25
		}

		rows, err := s.db.Query(ctx,
			`SELECT id, guild_id, user_id, case_number, moderator_id, reason, created_at
             FROM moderation_warnings
             WHERE guild_id=$1 AND user_id=$2
             ORDER BY case_number DESC
             LIMIT $3`,
			guildID, userID, limit,
		)
		if err != nil {
			yield(moderation.Warning{}, fmt.Errorf("Store.ListModerationWarnings: %w", err))
			return
		}
		defer rows.Close()

		var warning moderation.Warning
		for rows.Next() {
			warning = moderation.Warning{}
			if err := rows.Scan(&warning.ID, &warning.GuildID, &warning.UserID, &warning.CaseNumber, &warning.ModeratorID, &warning.Reason, &warning.CreatedAt); err != nil {
				yield(moderation.Warning{}, err)
				return
			}
			warning.CreatedAt = warning.CreatedAt.UTC()
			if !yield(warning, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(moderation.Warning{}, fmt.Errorf("Store.ListModerationWarnings: %w", err))
		}
	}
}

// SetGuildOwnerID sets or updates the cached owner ID for a guild.
func (s *Store) SetGuildOwnerID(ctx context.Context, guildID, ownerID string) error {
	if guildID == "" || ownerID == "" {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO guild_meta (guild_id, owner_id)
         VALUES ($1, $2)
         ON CONFLICT(guild_id) DO UPDATE SET owner_id=excluded.owner_id`,
		guildID, ownerID,
	)
	return err
}

// GetGuildOwnerID retrieves the cached owner ID for a guild.
func (s *Store) GetGuildOwnerID(ctx context.Context, guildID string) (string, bool, error) {
	var owner *string
	if err := s.db.QueryRow(ctx, `SELECT owner_id FROM guild_meta WHERE guild_id=$1`, guildID).Scan(&owner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("Store.GetGuildOwnerID: %w", err)
	}
	if owner == nil || strings.TrimSpace(*owner) == "" {
		return "", false, nil
	}
	return *owner, true, nil
}

```

// === FILE: pkg/storage/postgres/moderation_test.go ===
```go
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/moderation"
)

func TestStore_Moderation_NextModerationCaseNumber(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("INSERT INTO moderation_cases").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"last_case_number"}).AddRow(int64(2)))

	store.NextModerationCaseNumber(context.Background(), "guild1")
}

func TestStore_Moderation_GetGuildOwnerID_ErrNoRows(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT owner_id").WithArgs(pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	store.GetGuildOwnerID(context.Background(), "guild1")
}

func TestStore_Moderation_CreateWarning(t *testing.T) {
	t.Parallel()
	idgen.Init(1)
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO moderation_cases").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"last_case_number"}).AddRow(int64(1)))
	mock.ExpectQuery("INSERT INTO moderation_warnings").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(10), time.Now()))
	mock.ExpectCommit()
	mock.ExpectRollback()

	store.CreateModerationWarning(context.Background(), "guild1", "user1", "mod1", "spam", time.Now())
}

func TestStore_Moderation_NextModerationCaseNumber_Errors(t *testing.T) {
	t.Parallel()
	t.Run("empty guildID", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		_, err := store.NextModerationCaseNumber(context.Background(), "")
		if err == nil {
			t.Error("expected error on empty guildID")
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery("INSERT INTO moderation_cases").
			WillReturnError(errors.New("db error"))

		_, err := store.NextModerationCaseNumber(context.Background(), "guild1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Moderation_CreateWarning_Errors(t *testing.T) {
	t.Parallel()
	t.Run("missing fields", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		_, err := store.CreateModerationWarning(context.Background(), "", "", "", "", time.Now())
		if err == nil {
			t.Error("expected validation error")
		}
	})

	t.Run("begin tx error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin().WillReturnError(errors.New("begin error"))

		_, err := store.CreateModerationWarning(context.Background(), "g", "u", "m", "reason", time.Now())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("insert warning error", func(t *testing.T) {
		idgen.Init(1)
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectQuery("INSERT INTO moderation_cases").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"last_case_number"}).AddRow(int64(1)))
		mock.ExpectQuery("INSERT INTO moderation_warnings").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(errors.New("warning error"))
		mock.ExpectRollback()

		_, err := store.CreateModerationWarning(context.Background(), "g", "u", "m", "reason", time.Now())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Moderation_ListModerationWarnings(t *testing.T) {
	t.Parallel()
	t.Run("empty inputs", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		seq := store.ListModerationWarnings(context.Background(), "", "", 5)
		for range seq {
			t.Error("expected zero iteration")
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		rows := pgxmock.NewRows([]string{"id", "guild_id", "user_id", "case_number", "moderator_id", "reason", "created_at"}).
			AddRow(int64(1), "g1", "u1", int64(10), "mod1", "spam", now)

		mock.ExpectQuery(`SELECT id, guild_id, user_id, case_number, moderator_id, reason, created_at`).
			WithArgs("g1", "u1", 5).
			WillReturnRows(rows)

		seq := store.ListModerationWarnings(context.Background(), "g1", "u1", 5)
		var list []moderation.Warning
		for w, err := range seq {
			if err != nil {
				t.Fatalf("unexpected iterator error: %v", err)
			}
			list = append(list, w)
		}

		if len(list) != 1 || list[0].Reason != "spam" {
			t.Errorf("unexpected results: %+v", list)
		}
	})

	t.Run("query error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT id, guild_id, user_id, case_number, moderator_id, reason, created_at`).
			WillReturnError(errors.New("query error"))

		seq := store.ListModerationWarnings(context.Background(), "g1", "u1", 5)
		for _, err := range seq {
			if err == nil {
				t.Error("expected error from iterator")
			}
		}
	})
}

func TestStore_Moderation_GuildOwner(t *testing.T) {
	t.Parallel()
	t.Run("empty inputs", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.SetGuildOwnerID(context.Background(), "", "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("success Set and Get", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`INSERT INTO guild_meta`).
			WithArgs("g1", "owner1").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.SetGuildOwnerID(context.Background(), "g1", "owner1")
		if err != nil {
			t.Errorf("SetGuildOwnerID failed: %v", err)
		}

		ownerStr := "owner1"
		mock.ExpectQuery(`SELECT owner_id`).
			WithArgs("g1").
			WillReturnRows(pgxmock.NewRows([]string{"owner_id"}).AddRow(&ownerStr))

		owner, ok, err := store.GetGuildOwnerID(context.Background(), "g1")
		if err != nil || !ok || owner != "owner1" {
			t.Errorf("GetGuildOwnerID failed: owner=%s, ok=%t, err=%v", owner, ok, err)
		}
	})

	t.Run("owner is null", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT owner_id`).
			WithArgs("g1").
			WillReturnRows(pgxmock.NewRows([]string{"owner_id"}).AddRow(nil))

		owner, ok, err := store.GetGuildOwnerID(context.Background(), "g1")
		if err != nil || ok || owner != "" {
			t.Errorf("expected empty owner, ok=false; got: owner=%s, ok=%t, err=%v", owner, ok, err)
		}
	})
}

```

// === FILE: pkg/storage/postgres/qotd.go ===
```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
)

func normalizedSelector(s qotd.QuestionSelector) qotd.QuestionSelector {
	switch s {
	case qotd.QuestionSelectorRandom:
		return qotd.QuestionSelectorRandom
	default:
		return qotd.QuestionSelectorQueue
	}
}

// orderByClause returns the SQL ORDER BY fragment that implements the
// strategy. It is composed into the reserve queries and is intentionally
// bind-parameter-free so it can be inlined safely.
func orderByClause(s qotd.QuestionSelector) string {
	if normalizedSelector(s) == qotd.QuestionSelectorRandom {
		return "ORDER BY RANDOM()"
	}
	return "ORDER BY queue_position ASC, id ASC"
}

type qotdRowScanner interface {
	Scan(dest ...any) error
}

// CreateQOTDQuestion creates qotdquestion.
func (s *Store) CreateQOTDQuestion(ctx context.Context, rec qotd.QuestionRecord) (res *qotd.QuestionRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("create qotd question: %w", err)
		}
	}()
	normalized, err := normalizeQuestionRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDQuestion: %w", err)
	}

	position := normalized.QueuePosition
	if position < 1 {
		position = 0
	}

	row := s.db.QueryRow(ctx, `INSERT INTO qotd_questions (
			id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			display_id,
			created_by,
			scheduled_for_date_utc,
			used_at,
			published_once_at
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			CASE
				WHEN $6 > 0 THEN $7
				ELSE COALESCE((SELECT MAX(queue_position) + 1 FROM qotd_questions WHERE guild_id = $8 AND deck_id = $9), 1)
			END,
			COALESCE((SELECT MAX(display_id) + 1 FROM qotd_questions WHERE guild_id = $10 AND deck_id = $11), 1),
			$12,
			$13,
			$14,
			$15
		)
		RETURNING
			id,
			display_id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			published_once_at,
			created_at,
			updated_at`,
		idgen.GenerateID(),
		normalized.GuildID,
		normalized.DeckID,
		normalized.Body,
		normalized.Status,
		position,
		position,
		normalized.GuildID,
		normalized.DeckID,
		normalized.GuildID,
		normalized.DeckID,
		normalized.CreatedBy,
		nullableTime(normalized.ScheduledForDateUTC),
		nullableTime(normalized.UsedAt),
		nullableTime(normalized.PublishedOnceAt),
	)
	created, err := scanQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDQuestion: %w", err)
	}
	return created, nil
}

// UpdateQOTDQuestion updates qotdquestion.
func (s *Store) UpdateQOTDQuestion(ctx context.Context, rec qotd.QuestionRecord) (_ *qotd.QuestionRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update qotd question: %w", err)
		}
	}()
	if rec.ID <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	normalized, err := normalizeQuestionRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	var currentDeckID string
	var currentQueuePosition int64
	if err := tx.QueryRow(ctx,
		`SELECT deck_id, queue_position FROM qotd_questions WHERE id = $1 AND guild_id = $2 FOR UPDATE`,
		normalized.ID,
		normalized.GuildID,
	).Scan(&currentDeckID, &currentQueuePosition); err != nil {
		return nil, err
	}

	position := normalized.QueuePosition
	if position < 1 {
		position = 0
	}

	movingDeck := currentDeckID != normalized.DeckID
	row := tx.QueryRow(ctx,
		`UPDATE qotd_questions
		SET
			deck_id = $1,
			body = $2,
			status = $3,
			queue_position = CASE
				WHEN $4 > 0 THEN $5
				WHEN $6 THEN COALESCE((SELECT MAX(queue_position) + 1 FROM qotd_questions WHERE guild_id = $7 AND deck_id = $8), 1)
				ELSE queue_position
			END,
			display_id = CASE
				WHEN $9 THEN COALESCE((SELECT MAX(display_id) + 1 FROM qotd_questions WHERE guild_id = $10 AND deck_id = $11), 1)
				ELSE display_id
			END,
			created_by = $12,
			scheduled_for_date_utc = $13,
			used_at = $14,
			published_once_at = $15,
			updated_at = NOW()
		WHERE id = $16 AND guild_id = $17
		RETURNING
			id,
			display_id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			published_once_at,
			created_at,
			updated_at`,
		normalized.DeckID,
		normalized.Body,
		normalized.Status,
		position,
		position,
		movingDeck,
		normalized.GuildID,
		normalized.DeckID,
		movingDeck,
		normalized.GuildID,
		normalized.DeckID,
		normalized.CreatedBy,
		nullableTime(normalized.ScheduledForDateUTC),
		nullableTime(normalized.UsedAt),
		nullableTime(normalized.PublishedOnceAt),
		normalized.ID,
		normalized.GuildID,
	)
	updated, err := scanQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
	}

	needsReindex := movingDeck || (position > 0 && position != currentQueuePosition)
	if needsReindex {
		if movingDeck {
			if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, normalized.GuildID, currentDeckID); err != nil {
				return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
			}
		}
		if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, normalized.GuildID, normalized.DeckID); err != nil {
			return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
		}
		updated, err = getQOTDQuestionTx(ctx, tx, normalized.GuildID, normalized.ID)
		if err != nil {
			return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
	}
	return updated, nil
}

// DeleteQOTDQuestion deletes qotdquestion.
func (s *Store) DeleteQOTDQuestion(ctx context.Context, guildID string, questionID int64) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("delete qotd question: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || questionID <= 0 {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestion: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	var deckID string
	if err := tx.QueryRow(ctx,
		`SELECT deck_id FROM qotd_questions WHERE guild_id = $1 AND id = $2 FOR UPDATE`,
		guildID,
		questionID,
	).Scan(&deckID); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	if _, err := txExecContext(ctx, tx, `DELETE FROM qotd_questions WHERE guild_id = $1 AND id = $2`, guildID, questionID); err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestion: %w", err)
	}
	if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, guildID, deckID); err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestion: %w", err)
	}
	return nil
}

// DeleteQOTDQuestionsByDecks deletes qotdquestions by decks.
func (s *Store) DeleteQOTDQuestionsByDecks(ctx context.Context, guildID string, deckIDs []string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("delete qotd questions by decks: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil
	}
	normalizedDeckIDs := normalizeQOTDDeckIDs(deckIDs)
	if len(normalizedDeckIDs) == 0 {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestionsByDecks: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	for _, deckID := range normalizedDeckIDs {
		if _, err := txExecContext(ctx, tx,
			`DELETE FROM qotd_questions WHERE guild_id = $1 AND deck_id = $2`,
			guildID,
			deckID,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestionsByDecks: %w", err)
	}
	return nil
}

// ListQOTDQuestions lists qotdquestions.
func (s *Store) ListQOTDQuestions(ctx context.Context, guildID, deckID string) (_ iter.Seq2[qotd.QuestionRecord, error], err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list qotd questions: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, nil
	}

	return func(yield func(qotd.QuestionRecord, error) bool) {
		rows, err := s.db.Query(ctx, `SELECT
				id,
				display_id,
				guild_id,
				deck_id,
				body,
				status,
				queue_position,
				created_by,
				scheduled_for_date_utc,
				used_at,
				published_once_at,
				created_at,
				updated_at
			FROM qotd_questions
			WHERE guild_id = $1
			  AND ($2 = '' OR deck_id = $3)
			ORDER BY queue_position ASC, id ASC`,
			guildID,
			deckID,
			deckID,
		)
		if err != nil {
			yield(qotd.QuestionRecord{}, fmt.Errorf("Store.ListQOTDQuestions: %w", err))
			return
		}
		defer rows.Close()

		var record qotd.QuestionRecord
		for rows.Next() {
			record = qotd.QuestionRecord{}
			if err := scanQuestionRecordDest(rows, &record); err != nil {
				yield(qotd.QuestionRecord{}, fmt.Errorf("Store.ListQOTDQuestions: %w", err))
				return
			}
			if !yield(record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(qotd.QuestionRecord{}, fmt.Errorf("Store.ListQOTDQuestions: %w", err))
		}
	}, nil

}

// GetQOTDQuestion gets qotdquestion.
func (s *Store) GetQOTDQuestion(ctx context.Context, guildID string, questionID int64) (*qotd.QuestionRecord, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || questionID <= 0 {
		return nil, nil
	}

	row := s.db.QueryRow(ctx, `SELECT
			id,
			display_id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = $1 AND id = $2`,
		guildID,
		questionID,
	)
	record, err := scanQuestionRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDQuestion: %w", err)
	}
	return record, nil
}

// ReorderQOTDQuestions reorders qotdquestions.
func (s *Store) ReorderQOTDQuestions(ctx context.Context, guildID, deckID string, orderedIDs []int64) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("reorder qotd questions: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return fmt.Errorf("guild_id is required")
	}
	if deckID == "" {
		return fmt.Errorf("deck_id is required")
	}
	normalizedIDs, err := normalizeQOTDOrderedIDs(orderedIDs)
	if err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	rows, err := txQueryContext(ctx, tx,
		`SELECT id, queue_position
		FROM qotd_questions
		WHERE guild_id = $1
		  AND deck_id = $2
		ORDER BY queue_position ASC, id ASC
		FOR UPDATE`,
		guildID,
		deckID,
	)
	if err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	defer rows.Close()

	currentIDs := make([]int64, 0, len(normalizedIDs))
	var id int64
	var queuePosition int64
	var maxQueuePosition int64
	for rows.Next() {
		id = 0
		queuePosition = 0
		if err := rows.Scan(&id, &queuePosition); err != nil {
			return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
		}
		currentIDs = append(currentIDs, id)
		if queuePosition > maxQueuePosition {
			maxQueuePosition = queuePosition
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	if !sameQOTDIDSet(currentIDs, normalizedIDs) {
		return fmt.Errorf("ordered ids must match the full guild question set")
	}

	// Move rows out of the indexed range first so swaps do not trip the unique queue constraint.
	tempBase := maxQueuePosition + int64(len(normalizedIDs))
	for idx, id := range normalizedIDs {
		if _, err := txExecContext(ctx, tx,
			`UPDATE qotd_questions SET queue_position = $1, updated_at = NOW() WHERE guild_id = $2 AND deck_id = $3 AND id = $4`,
			tempBase+int64(idx)+1,
			guildID,
			deckID,
			id,
		); err != nil {
			return err
		}
	}
	for idx, id := range normalizedIDs {
		if _, err := txExecContext(ctx, tx,
			`UPDATE qotd_questions SET queue_position = $1, updated_at = NOW() WHERE guild_id = $2 AND deck_id = $3 AND id = $4`,
			idx+1,
			guildID,
			deckID,
			id,
		); err != nil {
			return err
		}
	}
	if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, guildID, deckID); err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	return nil
}

// ReserveNextQOTDQuestion reserves next qotdquestion.
func (s *Store) ReserveNextQOTDQuestion(ctx context.Context, guildID, deckID string, publishDateUTC time.Time, selector qotd.QuestionSelector) (_ *qotd.QuestionRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("reserve qotd question: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, fmt.Errorf("guild_id is required")
	}
	if deckID == "" {
		return nil, fmt.Errorf("deck_id is required")
	}
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if publishDateUTC.IsZero() {
		return nil, fmt.Errorf("publish_date_utc is required")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Store.ReserveNextQOTDQuestion: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	row := tx.QueryRow(ctx,
		`SELECT
			id,
			display_id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = $1
		  AND deck_id = $2
		  AND status = 'ready'
		  AND scheduled_for_date_utc IS NULL
		  AND published_once_at IS NULL
		`+orderByClause(selector)+`
		FOR UPDATE SKIP LOCKED
		LIMIT 1`,
		guildID,
		deckID,
	)
	record, err := scanQuestionRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.ReserveNextQOTDQuestion: %w", err)
	}

	row = tx.QueryRow(ctx,
		`UPDATE qotd_questions
		SET
			status = 'reserved',
			scheduled_for_date_utc = $1,
			updated_at = NOW()
		WHERE id = $2
		RETURNING
			id,
			display_id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			published_once_at,
			created_at,
			updated_at`,
		publishDateUTC,
		record.ID,
	)
	record, err = scanQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.ReserveNextQOTDQuestion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("Store.ReserveNextQOTDQuestion: %w", err)
	}
	return record, nil
}

// ReserveNextReadyQOTDQuestion reserves next ready qotdquestion.
func (s *Store) ReserveNextReadyQOTDQuestion(ctx context.Context, guildID, deckID string, selector qotd.QuestionSelector) (_ *qotd.QuestionRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("reserve ready qotd question: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, fmt.Errorf("guild_id is required")
	}
	if deckID == "" {
		return nil, fmt.Errorf("deck_id is required")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Store.ReserveNextReadyQOTDQuestion: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	row := tx.QueryRow(ctx,
		`SELECT
			id,
			display_id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = $1
		  AND deck_id = $2
		  AND status = 'ready'
		  AND scheduled_for_date_utc IS NULL
		  AND published_once_at IS NULL
		`+orderByClause(selector)+`
		FOR UPDATE SKIP LOCKED
		LIMIT 1`,
		guildID,
		deckID,
	)
	record, err := scanQuestionRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.ReserveNextReadyQOTDQuestion: %w", err)
	}

	row = tx.QueryRow(ctx,
		`UPDATE qotd_questions
		SET
			status = 'reserved',
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			display_id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			published_once_at,
			created_at,
			updated_at`,
		record.ID,
	)
	record, err = scanQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.ReserveNextReadyQOTDQuestion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("Store.ReserveNextReadyQOTDQuestion: %w", err)
	}
	return record, nil
}

// ReclaimOrphanReservedQOTDQuestions releases reservations whose
// CreateQOTDOfficialPostProvisioning never landed (process crashed between
// ReserveNextQOTDQuestion and the official-post insert) so they re-enter the
// publish queue. We restrict to scheduled_for_date_utc < todayUTC: today's
// reservations may belong to an in-flight publish that another goroutine is
// currently running. Returns the freed question IDs in queue order so callers
// can log or test the cleanup deterministically.
func (s *Store) ReclaimOrphanReservedQOTDQuestions(ctx context.Context, guildID string, todayUTC time.Time) iter.Seq2[int64, error] {
	return func(yield func(int64, error) bool) {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return
		}
		todayUTC = normalizeQOTDDateUTC(todayUTC)
		if todayUTC.IsZero() {
			yield(0, fmt.Errorf("reclaim orphan qotd reservations: today_utc is required"))
			return
		}

		rows, err := s.db.Query(ctx, `UPDATE qotd_questions q
			 SET
				status = 'ready',
				scheduled_for_date_utc = NULL,
				updated_at = NOW()
			 WHERE q.guild_id = $1
			   AND q.status = 'reserved'
			   AND q.scheduled_for_date_utc IS NOT NULL
			   AND q.scheduled_for_date_utc < $2
			   AND NOT EXISTS (
				 SELECT 1
				 FROM qotd_official_posts p
				 WHERE p.guild_id = q.guild_id
				   AND p.question_id = q.id
			   )
			 RETURNING q.id`,
			guildID,
			todayUTC,
		)
		if err != nil {
			yield(0, fmt.Errorf("reclaim orphan qotd reservations: Store.ReclaimOrphanReservedQOTDQuestions: %w", err))
			return
		}
		defer rows.Close()

		var id int64
		for rows.Next() {
			id = 0
			if err := rows.Scan(&id); err != nil {
				yield(0, fmt.Errorf("reclaim orphan qotd reservations: Store.ReclaimOrphanReservedQOTDQuestions: %w", err))
				return
			}
			if !yield(id, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(0, fmt.Errorf("reclaim orphan qotd reservations: Store.ReclaimOrphanReservedQOTDQuestions: %w", err))
		}
	}
}

func normalizeQuestionRecord(rec qotd.QuestionRecord) (qotd.QuestionRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.Body = strings.TrimSpace(rec.Body)
	rec.Status = strings.TrimSpace(rec.Status)
	rec.CreatedBy = strings.TrimSpace(rec.CreatedBy)
	rec.DisplayID = maxInt64(rec.DisplayID, 0)
	rec.QueuePosition = maxInt64(rec.QueuePosition, 0)
	rec.ScheduledForDateUTC = normalizeQOTDDatePtr(rec.ScheduledForDateUTC)
	rec.UsedAt = normalizeQOTDTimePtr(rec.UsedAt)
	rec.PublishedOnceAt = normalizeQOTDTimePtr(rec.PublishedOnceAt)

	if rec.GuildID == "" {
		return qotd.QuestionRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.DeckID == "" {
		return qotd.QuestionRecord{}, fmt.Errorf("deck_id is required")
	}
	if rec.Body == "" {
		return qotd.QuestionRecord{}, fmt.Errorf("body is required")
	}
	if rec.Status == "" {
		rec.Status = "draft"
	}
	return rec, nil
}

func normalizeQOTDOrderedIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("ordered ids are required")
	}
	seen := make(map[int64]struct{}, len(ids))
	normalized := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("ordered ids must be positive")
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("ordered ids must be unique")
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func normalizeQOTDDeckIDs(deckIDs []string) []string {
	if len(deckIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(deckIDs))
	normalized := make([]string, 0, len(deckIDs))
	for _, deckID := range deckIDs {
		deckID = strings.TrimSpace(deckID)
		if deckID == "" {
			continue
		}
		if _, ok := seen[deckID]; ok {
			continue
		}
		seen[deckID] = struct{}{}
		normalized = append(normalized, deckID)
	}
	return normalized
}

func sameQOTDIDSet(current, ordered []int64) bool {
	if len(current) != len(ordered) {
		return false
	}
	left := append([]int64(nil), current...)
	right := append([]int64(nil), ordered...)
	sort.Slice(left, func(i, j int) bool { return left[i] < left[j] })
	sort.Slice(right, func(i, j int) bool { return right[i] < right[j] })
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func scanQuestionRecordDest(scanner qotdRowScanner, dest *qotd.QuestionRecord) error {
	var scheduledFor sql.NullTime
	var usedAt sql.NullTime
	var publishedOnceAt sql.NullTime
	if err := scanner.Scan(
		&dest.ID,
		&dest.DisplayID,
		&dest.GuildID,
		&dest.DeckID,
		&dest.Body,
		&dest.Status,
		&dest.QueuePosition,
		&dest.CreatedBy,
		&scheduledFor,
		&usedAt,
		&publishedOnceAt,
		&dest.CreatedAt,
		&dest.UpdatedAt,
	); err != nil {
		return err
	}
	dest.ScheduledForDateUTC = timePtrFromNull(scheduledFor)
	dest.UsedAt = timePtrFromNull(usedAt)
	dest.PublishedOnceAt = timePtrFromNull(publishedOnceAt)
	return nil
}

func scanQuestionRecord(scanner qotdRowScanner) (*qotd.QuestionRecord, error) {
	var record qotd.QuestionRecord
	if err := scanQuestionRecordDest(scanner, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func getQOTDQuestionTx(ctx context.Context, tx pgx.Tx, guildID string, questionID int64) (*qotd.QuestionRecord, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}

	row := tx.QueryRow(ctx,
		`SELECT
			id,
			display_id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = $1 AND id = $2`,
		guildID,
		questionID,
	)
	record, err := scanQuestionRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getQOTDQuestionTx: %w", err)
	}
	return record, nil
}

func reindexQOTDQuestionDisplayIDsTx(ctx context.Context, tx pgx.Tx, guildID, deckID string) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return nil
	}

	if _, err := txExecContext(ctx, tx,
		`UPDATE qotd_questions
		SET display_id = -id
		WHERE guild_id = $1 AND deck_id = $2`,
		guildID,
		deckID,
	); err != nil {
		return err
	}

	if _, err := txExecContext(ctx, tx,
		`WITH ordered AS (
			SELECT
				id,
				ROW_NUMBER() OVER (ORDER BY queue_position ASC, id ASC)::BIGINT AS next_display_id
			FROM qotd_questions
			WHERE guild_id = $1
			  AND deck_id = $2
		)
		UPDATE qotd_questions AS questions
		SET display_id = ordered.next_display_id
		FROM ordered
		WHERE questions.id = ordered.id`,
		guildID,
		deckID,
	); err != nil {
		return err
	}
	return nil
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func timePtrFromNull(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	normalized := value.Time.UTC()
	return &normalized
}

func normalizeQOTDDateUTC(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func normalizeQOTDRequiredTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}

func normalizeQOTDDatePtr(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	normalized := normalizeQOTDDateUTC(*value)
	return &normalized
}

func normalizeQOTDTimePtr(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func zeroEmptyString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: value, Valid: true}
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

// CreateQOTDOfficialPostProvisioning creates qotdofficial post provisioning.
func (s *Store) CreateQOTDOfficialPostProvisioning(ctx context.Context, rec qotd.OfficialPostRecord) (res *qotd.OfficialPostRecord, err error) {
	normalized, err := normalizeOfficialPostRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDOfficialPostProvisioning: %w", err)
	}
	if normalized.State == "" {
		normalized.State = "provisioning"
	}

	row := s.db.QueryRow(ctx, `INSERT INTO qotd_official_posts (
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17,
			COALESCE((SELECT MAX(publish_ordinal) FROM qotd_official_posts WHERE guild_id = $18 AND deck_id = $19), 0) + 1,
			$20, $21, $22, $23, $24, $25
		)
		RETURNING
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at`,
		idgen.GenerateID(),
		normalized.GuildID,
		normalized.DeckID,
		normalized.DeckNameSnapshot,
		normalized.QuestionID,
		normalized.PublishMode,
		normalized.ConsumeAutomaticSlot,
		normalized.PublishDateUTC,
		normalized.State,
		normalized.ChannelID,
		normalized.QuestionListThreadID,
		normalized.QuestionListEntryMessageID,
		zeroEmptyString(normalized.DiscordThreadID),
		zeroEmptyString(normalized.DiscordStarterMessageID),
		normalized.AnswerChannelID,
		normalized.QuestionTextSnapshot,
		zeroEmptyString(normalized.Nonce),
		// publish_ordinal subquery binds:
		normalized.GuildID,
		normalized.DeckID,
		nullableTime(normalized.PublishedAt),
		normalized.GraceUntil.UTC(),
		normalized.ArchiveAt.UTC(),
		nullableTime(normalized.ClosedAt),
		nullableTime(normalized.ArchivedAt),
		nullableTime(normalized.LastReconciledAt),
	)
	created, err := scanOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDOfficialPostProvisioning: %w", err)
	}
	return created, nil
}

// qotd.FinalizeOfficialPostParams carries the Discord-side identifiers and the
// publish timestamp recorded when a QOTD official post finishes provisioning.

// FinalizeQOTDOfficialPost finalizes qotdofficial post.
func (s *Store) FinalizeQOTDOfficialPost(ctx context.Context, params qotd.FinalizeOfficialPostParams) (_ *qotd.OfficialPostRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("finalize qotd official post: %w", err)
		}
	}()
	if params.ID <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	questionListThreadID := strings.TrimSpace(params.QuestionListThreadID)
	questionListEntryMessageID := strings.TrimSpace(params.QuestionListEntryMessageID)
	discordThreadID := strings.TrimSpace(params.DiscordThreadID)
	starterMessageID := strings.TrimSpace(params.StarterMessageID)
	answerChannelID := strings.TrimSpace(params.AnswerChannelID)
	if starterMessageID == "" {
		return nil, fmt.Errorf("starter message id is required")
	}
	if answerChannelID == "" {
		return nil, fmt.Errorf("answer channel id is required")
	}
	if params.PublishedAt.IsZero() {
		return nil, fmt.Errorf("published_at is required")
	}

	row := s.db.QueryRow(ctx, `UPDATE qotd_official_posts
		SET
			question_list_thread_id = $1,
			question_list_entry_message_id = $2,
			discord_thread_id = $3,
			discord_starter_message_id = $4,
			answer_channel_id = $5,
			published_at = $6,
			updated_at = NOW()
		WHERE id = $7
		RETURNING
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at`,
		questionListThreadID,
		questionListEntryMessageID,
		zeroEmptyString(discordThreadID),
		starterMessageID,
		answerChannelID,
		params.PublishedAt.UTC(),
		params.ID,
	)
	updated, err := scanOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.FinalizeQOTDOfficialPost: %w", err)
	}
	return updated, nil
}

// GetQOTDOfficialPostByID gets qotdofficial post by id.
func (s *Store) GetQOTDOfficialPostByID(ctx context.Context, id int64) (res *qotd.OfficialPostRecord, err error) {
	if id <= 0 {
		return nil, nil
	}
	row := s.db.QueryRow(ctx, `SELECT
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at
		FROM qotd_official_posts
		WHERE id = $1`,
		id,
	)
	record, err := scanOfficialPostRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDOfficialPostByID: %w", err)
	}
	return record, nil
}

// GetQOTDOfficialPostByDate gets qotdofficial post by date.
func (s *Store) GetQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (res *qotd.OfficialPostRecord, err error) {
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}
	row := s.db.QueryRow(ctx, `SELECT
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at
		FROM qotd_official_posts
		WHERE guild_id = $1
		  AND publish_date_utc = $2::DATE
		ORDER BY
		  CASE WHEN archived_at IS NULL THEN 0 ELSE 1 END,
		  CASE
		    WHEN published_at IS NOT NULL
		      AND discord_thread_id IS NOT NULL
		      AND discord_starter_message_id IS NOT NULL
		      AND answer_channel_id IS NOT NULL THEN 0
		    ELSE 1
		  END,
		  COALESCE(published_at, updated_at) DESC,
		  id DESC,
		  updated_at DESC
		LIMIT 1`,
		guildID,
		publishDateUTC,
	)
	record, err := scanOfficialPostRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDOfficialPostByDate: %w", err)
	}
	return record, nil
}

// ListQOTDOfficialPostsByDate lists qotdofficial posts by date.
func (s *Store) ListQOTDOfficialPostsByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (_ iter.Seq[qotd.OfficialPostRecord], err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list qotd official posts by date: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return func(yield func(qotd.OfficialPostRecord) bool) {}, nil
	}

	rows, err := s.db.Query(ctx, `SELECT
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at
		FROM qotd_official_posts
		WHERE guild_id = $1
		  AND publish_date_utc = $2::DATE
		ORDER BY
		  CASE WHEN archived_at IS NULL THEN 0 ELSE 1 END,
		  CASE
		    WHEN published_at IS NOT NULL
		      AND discord_thread_id IS NOT NULL
		      AND discord_starter_message_id IS NOT NULL
		      AND answer_channel_id IS NOT NULL THEN 0
		    ELSE 1
		  END,
		  CASE WHEN publish_mode = 'scheduled' THEN 0 ELSE 1 END,
		  COALESCE(published_at, updated_at) DESC,
		  id DESC,
		  updated_at DESC`,
		guildID,
		publishDateUTC,
	)
	if err != nil {
		return nil, fmt.Errorf("Store.ListQOTDOfficialPostsByDate: %w", err)
	}

	return func(yield func(qotd.OfficialPostRecord) bool) {
		defer rows.Close()
		var record qotd.OfficialPostRecord
		for rows.Next() {
			record = qotd.OfficialPostRecord{}
			if err := scanOfficialPostRecordDest(rows, &record); err != nil {
				return
			}
			if !yield(record) {
				return
			}
		}
	}, nil
}

// GetAutomaticSlotQOTDOfficialPostByDate gets automatic slot qotdofficial post by date.
func (s *Store) GetAutomaticSlotQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (res *qotd.OfficialPostRecord, err error) {
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}
	row := s.db.QueryRow(ctx, `SELECT
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at
		FROM qotd_official_posts
		WHERE guild_id = $1
		  AND publish_date_utc = $2::DATE
		  AND (publish_mode = 'scheduled' OR consume_automatic_slot = TRUE)
		ORDER BY
		  CASE WHEN archived_at IS NULL THEN 0 ELSE 1 END,
		  CASE
		    WHEN published_at IS NOT NULL
		      AND discord_thread_id IS NOT NULL
		      AND discord_starter_message_id IS NOT NULL
		      AND answer_channel_id IS NOT NULL THEN 0
		    ELSE 1
		  END,
		  CASE WHEN publish_mode = 'scheduled' THEN 0 ELSE 1 END,
		  COALESCE(published_at, updated_at) DESC,
		  id DESC,
		  updated_at DESC
		LIMIT 1`,
		guildID,
		publishDateUTC,
	)
	record, err := scanOfficialPostRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetAutomaticSlotQOTDOfficialPostByDate: %w", err)
	}
	return record, nil
}

// GetScheduledQOTDOfficialPostByDate gets scheduled qotdofficial post by date.
func (s *Store) GetScheduledQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (res *qotd.OfficialPostRecord, err error) {
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}

	row := s.db.QueryRow(ctx, `SELECT
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at
		FROM qotd_official_posts
		WHERE guild_id = $1
		  AND publish_mode = 'scheduled'
		  AND publish_date_utc = $2::DATE
		ORDER BY
		  CASE WHEN archived_at IS NULL THEN 0 ELSE 1 END,
		  CASE
		    WHEN published_at IS NOT NULL
		      AND discord_thread_id IS NOT NULL
		      AND discord_starter_message_id IS NOT NULL
		      AND answer_channel_id IS NOT NULL THEN 0
		    ELSE 1
		  END,
		  updated_at DESC,
		  id DESC
		LIMIT 1`,
		guildID,
		publishDateUTC,
	)
	record, err := scanOfficialPostRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetScheduledQOTDOfficialPostByDate: %w", err)
	}
	return record, nil
}

// DeleteQOTDOfficialPostsByDeck deletes qotdofficial posts by deck.
func (s *Store) DeleteQOTDOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (count int, err error) {
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return 0, nil
	}

	result, err := s.db.Exec(ctx,
		`DELETE FROM qotd_official_posts WHERE guild_id = $1 AND deck_id = $2`,
		guildID,
		deckID,
	)
	if err != nil {
		return 0, fmt.Errorf("Store.DeleteQOTDOfficialPostsByDeck: %w", err)
	}
	deleted := result.RowsAffected()
	return int(deleted), nil
}

// DeleteQOTDOfficialPostByID deletes qotdofficial post by id.
func (s *Store) DeleteQOTDOfficialPostByID(ctx context.Context, id int64) (err error) {
	if id <= 0 {
		return nil
	}

	if _, err := s.db.Exec(ctx, `DELETE FROM qotd_official_posts WHERE id = $1`, id); err != nil {
		return fmt.Errorf("Store.DeleteQOTDOfficialPostByID: %w", err)
	}
	return nil
}

// DeleteQOTDUnpublishedOfficialPostsByDeck deletes qotdunpublished official posts by deck.
func (s *Store) DeleteQOTDUnpublishedOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (count int, err error) {
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return 0, nil
	}

	result, err := s.db.Exec(ctx,
		`DELETE FROM qotd_official_posts
		 WHERE guild_id = $1
		   AND deck_id = $2
		   AND published_at IS NULL`,
		guildID,
		deckID,
	)
	if err != nil {
		return 0, fmt.Errorf("Store.DeleteQOTDUnpublishedOfficialPostsByDeck: %w", err)
	}
	deleted := result.RowsAffected()
	return int(deleted), nil
}
func normalizeOfficialPostRecord(rec qotd.OfficialPostRecord) (qotd.OfficialPostRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.DeckNameSnapshot = strings.TrimSpace(rec.DeckNameSnapshot)
	rec.PublishMode = strings.TrimSpace(rec.PublishMode)
	rec.State = strings.TrimSpace(rec.State)
	rec.ChannelID = strings.TrimSpace(rec.ChannelID)
	rec.QuestionListThreadID = strings.TrimSpace(rec.QuestionListThreadID)
	rec.QuestionListEntryMessageID = strings.TrimSpace(rec.QuestionListEntryMessageID)
	rec.DiscordThreadID = strings.TrimSpace(rec.DiscordThreadID)
	rec.DiscordStarterMessageID = strings.TrimSpace(rec.DiscordStarterMessageID)
	rec.AnswerChannelID = strings.TrimSpace(rec.AnswerChannelID)
	rec.QuestionTextSnapshot = strings.TrimSpace(rec.QuestionTextSnapshot)
	rec.Nonce = strings.TrimSpace(rec.Nonce)
	rec.PublishDateUTC = normalizeQOTDDateUTC(rec.PublishDateUTC)
	rec.PublishedAt = normalizeQOTDTimePtr(rec.PublishedAt)
	rec.ClosedAt = normalizeQOTDTimePtr(rec.ClosedAt)
	rec.ArchivedAt = normalizeQOTDTimePtr(rec.ArchivedAt)
	rec.LastReconciledAt = normalizeQOTDTimePtr(rec.LastReconciledAt)
	rec.GraceUntil = normalizeQOTDRequiredTime(rec.GraceUntil)
	rec.ArchiveAt = normalizeQOTDRequiredTime(rec.ArchiveAt)

	if rec.PublishMode == "" {
		rec.PublishMode = "scheduled"
	}
	if rec.PublishMode == "scheduled" {
		rec.ConsumeAutomaticSlot = true
	}

	if rec.GuildID == "" {
		return qotd.OfficialPostRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.DeckID == "" {
		return qotd.OfficialPostRecord{}, fmt.Errorf("deck_id is required")
	}
	if rec.DeckNameSnapshot == "" {
		return qotd.OfficialPostRecord{}, fmt.Errorf("deck_name_snapshot is required")
	}
	if rec.QuestionID <= 0 {
		return qotd.OfficialPostRecord{}, fmt.Errorf("question_id is required")
	}
	if rec.PublishMode == "" {
		return qotd.OfficialPostRecord{}, fmt.Errorf("publish_mode is required")
	}
	if rec.PublishDateUTC.IsZero() {
		return qotd.OfficialPostRecord{}, fmt.Errorf("publish_date_utc is required")
	}
	if rec.ChannelID == "" {
		return qotd.OfficialPostRecord{}, fmt.Errorf("channel_id is required")
	}
	if rec.QuestionTextSnapshot == "" {
		return qotd.OfficialPostRecord{}, fmt.Errorf("question_text_snapshot is required")
	}
	if rec.GraceUntil.IsZero() {
		return qotd.OfficialPostRecord{}, fmt.Errorf("grace_until is required")
	}
	if rec.ArchiveAt.IsZero() {
		return qotd.OfficialPostRecord{}, fmt.Errorf("archive_at is required")
	}
	return rec, nil
}
func scanOfficialPostRecordDest(scanner qotdRowScanner, dest *qotd.OfficialPostRecord) error {
	var questionListThreadID sql.NullString
	var questionListEntryMessageID sql.NullString
	var threadID sql.NullString
	var starterMessageID sql.NullString
	var answerChannelID sql.NullString
	var consumeAutomaticSlot bool
	var nonce sql.NullString
	var publishedAt sql.NullTime
	var closedAt sql.NullTime
	var archivedAt sql.NullTime
	var reconciledAt sql.NullTime
	if err := scanner.Scan(
		&dest.ID,
		&dest.GuildID,
		&dest.DeckID,
		&dest.DeckNameSnapshot,
		&dest.QuestionID,
		&dest.PublishMode,
		&consumeAutomaticSlot,
		&dest.PublishDateUTC,
		&dest.State,
		&dest.ChannelID,
		&questionListThreadID,
		&questionListEntryMessageID,
		&threadID,
		&starterMessageID,
		&answerChannelID,
		&dest.QuestionTextSnapshot,
		&nonce,
		&dest.PublishOrdinal,
		&publishedAt,
		&dest.GraceUntil,
		&dest.ArchiveAt,
		&closedAt,
		&archivedAt,
		&reconciledAt,
		&dest.CreatedAt,
		&dest.UpdatedAt,
	); err != nil {
		return err
	}
	dest.ConsumeAutomaticSlot = consumeAutomaticSlot
	dest.QuestionListThreadID = strings.TrimSpace(questionListThreadID.String)
	dest.QuestionListEntryMessageID = strings.TrimSpace(questionListEntryMessageID.String)
	dest.DiscordThreadID = threadID.String
	dest.DiscordStarterMessageID = starterMessageID.String
	dest.AnswerChannelID = strings.TrimSpace(answerChannelID.String)
	dest.Nonce = strings.TrimSpace(nonce.String)
	dest.PublishedAt = timePtrFromNull(publishedAt)
	dest.ClosedAt = timePtrFromNull(closedAt)
	dest.ArchivedAt = timePtrFromNull(archivedAt)
	dest.LastReconciledAt = timePtrFromNull(reconciledAt)
	dest.PublishMode = strings.TrimSpace(dest.PublishMode)
	dest.PublishDateUTC = normalizeQOTDDateUTC(dest.PublishDateUTC)
	dest.GraceUntil = dest.GraceUntil.UTC()
	dest.ArchiveAt = dest.ArchiveAt.UTC()
	return nil
}

func scanOfficialPostRecord(scanner qotdRowScanner) (*qotd.OfficialPostRecord, error) {
	var record qotd.OfficialPostRecord
	if err := scanOfficialPostRecordDest(scanner, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// UpdateQOTDOfficialPostProgress updates qotdofficial post progress.
func (s *Store) UpdateQOTDOfficialPostProgress(ctx context.Context, id int64, progress qotd.OfficialPostRecord) (*qotd.OfficialPostRecord, error) {
	if id <= 0 {
		return nil, fmt.Errorf("update qotd official post progress: id is required")
	}
	progress.QuestionListThreadID = strings.TrimSpace(progress.QuestionListThreadID)
	progress.QuestionListEntryMessageID = strings.TrimSpace(progress.QuestionListEntryMessageID)
	progress.DiscordThreadID = strings.TrimSpace(progress.DiscordThreadID)
	progress.DiscordStarterMessageID = strings.TrimSpace(progress.DiscordStarterMessageID)
	progress.AnswerChannelID = strings.TrimSpace(progress.AnswerChannelID)
	progress.PublishedAt = normalizeQOTDTimePtr(progress.PublishedAt)

	row := s.db.QueryRow(ctx, `UPDATE qotd_official_posts
		SET
			question_list_thread_id = COALESCE(NULLIF($1, ''), question_list_thread_id),
			question_list_entry_message_id = COALESCE(NULLIF($2, ''), question_list_entry_message_id),
			discord_thread_id = COALESCE(NULLIF($3, ''), discord_thread_id),
			discord_starter_message_id = COALESCE(NULLIF($4, ''), discord_starter_message_id),
			answer_channel_id = COALESCE(NULLIF($5, ''), answer_channel_id),
			published_at = COALESCE($6, published_at),
			updated_at = NOW()
		WHERE id = $7
		RETURNING
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at`,
		progress.QuestionListThreadID,
		progress.QuestionListEntryMessageID,
		zeroEmptyString(progress.DiscordThreadID),
		zeroEmptyString(progress.DiscordStarterMessageID),
		progress.AnswerChannelID,
		nullableTime(progress.PublishedAt),
		id,
	)
	record, err := scanOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDOfficialPostProgress: %w", err)
	}
	return record, nil
}

// ListQOTDOfficialPostsPendingRecovery lists qotdofficial posts pending recovery.
func (s *Store) ListQOTDOfficialPostsPendingRecovery(ctx context.Context, guildID string) iter.Seq2[qotd.OfficialPostRecord, error] {
	return func(yield func(qotd.OfficialPostRecord, error) bool) {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return
		}

		rows, err := s.db.Query(ctx, `SELECT
				id,
				guild_id,
				deck_id,
				deck_name_snapshot,
				question_id,
				publish_mode,
				consume_automatic_slot,
				publish_date_utc,
				state,
				channel_id,
				question_list_thread_id,
				question_list_entry_message_id,
				discord_thread_id,
				discord_starter_message_id,
				answer_channel_id,
				question_text_snapshot,
				nonce,
				publish_ordinal,
				published_at,
				grace_until,
				archive_at,
				closed_at,
				archived_at,
				last_reconciled_at,
				created_at,
				updated_at
			FROM qotd_official_posts
			WHERE guild_id = $1
			  AND archived_at IS NULL
			  AND state IN ('provisioning', 'failed')
			ORDER BY updated_at ASC, id ASC`,
			guildID,
		)
		if err != nil {
			yield(qotd.OfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsPendingRecovery: %w", err))
			return
		}
		defer rows.Close()

		var record qotd.OfficialPostRecord
		for rows.Next() {
			record = qotd.OfficialPostRecord{}
			if err := scanOfficialPostRecordDest(rows, &record); err != nil {
				yield(qotd.OfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsPendingRecovery: %w", err))
				return
			}
			if !yield(record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(qotd.OfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsPendingRecovery: %w", err))
		}
	}
}

// GetQOTDSurfaceByDeck gets qotdsurface by deck.
func (s *Store) GetQOTDSurfaceByDeck(ctx context.Context, guildID, deckID string) (res *qotd.SurfaceRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get qotd surface by deck: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return nil, nil
	}

	row := s.db.QueryRow(ctx, `SELECT
			id,
			guild_id,
			deck_id,
			channel_id,
			question_list_thread_id,
			created_at,
			updated_at
		FROM qotd_forum_surfaces
		WHERE guild_id = $1 AND deck_id = $2`,
		guildID,
		deckID,
	)
	record, err := scanSurfaceRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDSurfaceByDeck: %w", err)
	}
	return record, nil
}

// UpsertQOTDSurface upserts qotdsurface.
func (s *Store) UpsertQOTDSurface(ctx context.Context, rec qotd.SurfaceRecord) (res *qotd.SurfaceRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("upsert qotd surface: %w", err)
		}
	}()
	normalized, err := normalizeSurfaceRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.UpsertQOTDSurface: %w", err)
	}

	row := s.db.QueryRow(ctx, `INSERT INTO qotd_forum_surfaces (
			id,
			guild_id,
			deck_id,
			channel_id,
			question_list_thread_id
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (guild_id, deck_id) DO UPDATE
		SET
			channel_id = EXCLUDED.channel_id,
			question_list_thread_id = EXCLUDED.question_list_thread_id,
			updated_at = NOW()
		RETURNING
			id,
			guild_id,
			deck_id,
			channel_id,
			question_list_thread_id,
			created_at,
			updated_at`,
		idgen.GenerateID(),
		normalized.GuildID,
		normalized.DeckID,
		normalized.ChannelID,
		zeroEmptyString(normalized.QuestionListThreadID),
	)
	updated, err := scanSurfaceRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.UpsertQOTDSurface: %w", err)
	}
	return updated, nil
}

// DeleteQOTDSurfaceByDeck deletes qotdsurface by deck.
func (s *Store) DeleteQOTDSurfaceByDeck(ctx context.Context, guildID, deckID string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("delete qotd surface by deck: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return nil
	}

	if _, err := s.db.Exec(ctx,
		`DELETE FROM qotd_forum_surfaces WHERE guild_id = $1 AND deck_id = $2`,
		guildID,
		deckID,
	); err != nil {
		return err
	}
	return nil
}

// CreateQOTDAnswerMessage creates qotdanswer message.
func (s *Store) CreateQOTDAnswerMessage(ctx context.Context, rec qotd.AnswerMessageRecord) (res *qotd.AnswerMessageRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("create qotd answer message: %w", err)
		}
	}()
	normalized, err := normalizeAnswerMessageRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDAnswerMessage: %w", err)
	}

	row := s.db.QueryRow(ctx, `INSERT INTO qotd_answer_messages (
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			closed_at,
			archived_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		idgen.GenerateID(),
		normalized.GuildID,
		normalized.OfficialPostID,
		normalized.UserID,
		normalized.State,
		normalized.AnswerChannelID,
		zeroEmptyString(normalized.DiscordMessageID),
		zeroEmptyString(normalized.CreatedViaInteractionID),
		nullableTime(normalized.ClosedAt),
		nullableTime(normalized.ArchivedAt),
	)
	created, err := scanAnswerMessageRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDAnswerMessage: %w", err)
	}
	return created, nil
}

// FinalizeQOTDAnswerMessage finalizes qotdanswer message.
func (s *Store) FinalizeQOTDAnswerMessage(ctx context.Context, id int64, discordMessageID string) (_ *qotd.AnswerMessageRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("finalize qotd answer message: %w", err)
		}
	}()
	if id <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	discordMessageID = strings.TrimSpace(discordMessageID)
	if discordMessageID == "" {
		return nil, fmt.Errorf("discord message id is required")
	}

	row := s.db.QueryRow(ctx, `UPDATE qotd_answer_messages
		SET
			discord_message_id = $1,
			updated_at = NOW()
		WHERE id = $2
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		discordMessageID,
		id,
	)
	record, err := scanAnswerMessageRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.FinalizeQOTDAnswerMessage: %w", err)
	}
	return record, nil
}

// GetQOTDAnswerMessageByOfficialPostAndUser gets qotdanswer message by official post and user.
func (s *Store) GetQOTDAnswerMessageByOfficialPostAndUser(ctx context.Context, officialPostID int64, userID string) (res *qotd.AnswerMessageRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get qotd answer message by official post and user: %w", err)
		}
	}()
	userID = strings.TrimSpace(userID)
	if officialPostID <= 0 || userID == "" {
		return nil, nil
	}

	row := s.db.QueryRow(ctx, `SELECT
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			created_at,
			updated_at,
			closed_at,
			archived_at
		FROM qotd_answer_messages
		WHERE official_post_id = $1 AND user_id = $2`,
		officialPostID,
		userID,
	)
	record, err := scanAnswerMessageRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDAnswerMessageByOfficialPostAndUser: %w", err)
	}
	return record, nil
}

// ListQOTDAnswerMessagesByOfficialPost lists qotdanswer messages by official post.
func (s *Store) ListQOTDAnswerMessagesByOfficialPost(ctx context.Context, officialPostID int64) (_ iter.Seq2[qotd.AnswerMessageRecord, error], err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list qotd answer messages by official post: %w", err)
		}
	}()
	if officialPostID <= 0 {
		return nil, nil
	}

	return func(yield func(qotd.AnswerMessageRecord, error) bool) {
		rows, err := s.db.Query(ctx, `SELECT
				id,
				guild_id,
				official_post_id,
				user_id,
				state,
				answer_channel_id,
				discord_message_id,
				created_via_interaction_id,
				created_at,
				updated_at,
				closed_at,
				archived_at
			FROM qotd_answer_messages
			WHERE official_post_id = $1
			ORDER BY id ASC`,
			officialPostID,
		)
		if err != nil {
			yield(qotd.AnswerMessageRecord{}, fmt.Errorf("Store.ListQOTDAnswerMessagesByOfficialPost: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			record, err := scanAnswerMessageRecord(rows)
			if err != nil {
				yield(qotd.AnswerMessageRecord{}, fmt.Errorf("Store.ListQOTDAnswerMessagesByOfficialPost: %w", err))
				return
			}
			if !yield(*record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(qotd.AnswerMessageRecord{}, fmt.Errorf("Store.ListQOTDAnswerMessagesByOfficialPost: %w", err))
		}
	}, nil

}

// UpdateQOTDAnswerMessageState updates qotdanswer message state.
func (s *Store) UpdateQOTDAnswerMessageState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (_ *qotd.AnswerMessageRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update qotd answer message state: %w", err)
		}
	}()
	if id <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return nil, fmt.Errorf("state is required")
	}

	row := s.db.QueryRow(ctx, `UPDATE qotd_answer_messages
		SET
			state = $1,
			closed_at = $2,
			archived_at = $3,
			updated_at = NOW()
		WHERE id = $4
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		state,
		nullableTime(closedAt),
		nullableTime(archivedAt),
		id,
	)
	record, err := scanAnswerMessageRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDAnswerMessageState: %w", err)
	}
	return record, nil
}

func normalizeSurfaceRecord(rec qotd.SurfaceRecord) (qotd.SurfaceRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.ChannelID = strings.TrimSpace(rec.ChannelID)
	rec.QuestionListThreadID = strings.TrimSpace(rec.QuestionListThreadID)

	if rec.GuildID == "" {
		return qotd.SurfaceRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.DeckID == "" {
		return qotd.SurfaceRecord{}, fmt.Errorf("deck_id is required")
	}
	if rec.ChannelID == "" {
		return qotd.SurfaceRecord{}, fmt.Errorf("channel_id is required")
	}
	return rec, nil
}

func normalizeAnswerMessageRecord(rec qotd.AnswerMessageRecord) (qotd.AnswerMessageRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.UserID = strings.TrimSpace(rec.UserID)
	rec.State = strings.TrimSpace(rec.State)
	rec.AnswerChannelID = strings.TrimSpace(rec.AnswerChannelID)
	rec.DiscordMessageID = strings.TrimSpace(rec.DiscordMessageID)
	rec.CreatedViaInteractionID = strings.TrimSpace(rec.CreatedViaInteractionID)
	rec.ClosedAt = normalizeQOTDTimePtr(rec.ClosedAt)
	rec.ArchivedAt = normalizeQOTDTimePtr(rec.ArchivedAt)

	if rec.State == "" {
		rec.State = "active"
	}
	if rec.GuildID == "" {
		return qotd.AnswerMessageRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.OfficialPostID <= 0 {
		return qotd.AnswerMessageRecord{}, fmt.Errorf("official_post_id is required")
	}
	if rec.UserID == "" {
		return qotd.AnswerMessageRecord{}, fmt.Errorf("user_id is required")
	}
	if rec.AnswerChannelID == "" {
		return qotd.AnswerMessageRecord{}, fmt.Errorf("answer_channel_id is required")
	}
	return rec, nil
}

func scanSurfaceRecord(scanner qotdRowScanner) (*qotd.SurfaceRecord, error) {
	var record qotd.SurfaceRecord
	var questionListThreadID sql.NullString
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.DeckID,
		&record.ChannelID,
		&questionListThreadID,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.QuestionListThreadID = strings.TrimSpace(questionListThreadID.String)
	return &record, nil
}

func scanAnswerMessageRecord(scanner qotdRowScanner) (*qotd.AnswerMessageRecord, error) {
	var record qotd.AnswerMessageRecord
	var discordMessageID sql.NullString
	var interactionID sql.NullString
	var closedAt sql.NullTime
	var archivedAt sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.OfficialPostID,
		&record.UserID,
		&record.State,
		&record.AnswerChannelID,
		&discordMessageID,
		&interactionID,
		&record.CreatedAt,
		&record.UpdatedAt,
		&closedAt,
		&archivedAt,
	); err != nil {
		return nil, err
	}
	record.DiscordMessageID = strings.TrimSpace(discordMessageID.String)
	record.CreatedViaInteractionID = strings.TrimSpace(interactionID.String)
	record.ClosedAt = timePtrFromNull(closedAt)
	record.ArchivedAt = timePtrFromNull(archivedAt)
	return &record, nil
}

// GetCurrentAndPreviousQOTDPosts gets current and previous qotdposts.
func (s *Store) GetCurrentAndPreviousQOTDPosts(ctx context.Context, guildID string, now time.Time) iter.Seq2[qotd.OfficialPostRecord, error] {
	return func(yield func(qotd.OfficialPostRecord, error) bool) {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return
		}
		if now.IsZero() {
			now = time.Now().UTC()
		} else {
			now = now.UTC()
		}

		rows, err := s.db.Query(ctx, `SELECT
				id,
				guild_id,
				deck_id,
				deck_name_snapshot,
				question_id,
				publish_mode,
				consume_automatic_slot,
				publish_date_utc,
				state,
				channel_id,
				question_list_thread_id,
				question_list_entry_message_id,
				discord_thread_id,
				discord_starter_message_id,
				answer_channel_id,
				question_text_snapshot,
				nonce,
				publish_ordinal,
				published_at,
				grace_until,
				archive_at,
				closed_at,
				archived_at,
				last_reconciled_at,
				created_at,
				updated_at
			FROM qotd_official_posts
			WHERE guild_id = $1
			  AND published_at IS NOT NULL
			  AND archived_at IS NULL
			  AND archive_at > $2
			ORDER BY publish_date_utc DESC, published_at DESC, id DESC
			LIMIT 2`,
			guildID,
			now,
		)
		if err != nil {
			yield(qotd.OfficialPostRecord{}, fmt.Errorf("Store.GetCurrentAndPreviousQOTDPosts: %w", err))
			return
		}
		defer rows.Close()

		var record qotd.OfficialPostRecord
		for rows.Next() {
			record = qotd.OfficialPostRecord{}
			if err := scanOfficialPostRecordDest(rows, &record); err != nil {
				yield(qotd.OfficialPostRecord{}, fmt.Errorf("Store.GetCurrentAndPreviousQOTDPosts: %w", err))
				return
			}
			if !yield(record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(qotd.OfficialPostRecord{}, fmt.Errorf("Store.GetCurrentAndPreviousQOTDPosts: %w", err))
		}
	}
}

// ListQOTDOfficialPostsNeedingArchive lists qotdofficial posts needing archive.
func (s *Store) ListQOTDOfficialPostsNeedingArchive(ctx context.Context, now time.Time) iter.Seq2[qotd.OfficialPostRecord, error] {
	return func(yield func(qotd.OfficialPostRecord, error) bool) {
		if now.IsZero() {
			now = time.Now().UTC()
		} else {
			now = now.UTC()
		}

		rows, err := s.db.Query(ctx, `SELECT
				id,
				guild_id,
				deck_id,
				deck_name_snapshot,
				question_id,
				publish_mode,
				consume_automatic_slot,
				publish_date_utc,
				state,
				channel_id,
				question_list_thread_id,
				question_list_entry_message_id,
				discord_thread_id,
				discord_starter_message_id,
				answer_channel_id,
				question_text_snapshot,
				nonce,
				publish_ordinal,
				published_at,
				grace_until,
				archive_at,
				closed_at,
				archived_at,
				last_reconciled_at,
				created_at,
				updated_at
			FROM qotd_official_posts
			WHERE published_at IS NOT NULL
			  AND archived_at IS NULL
			  AND archive_at <= $1
			ORDER BY archive_at ASC, id ASC`,
			now,
		)
		if err != nil {
			yield(qotd.OfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsNeedingArchive: %w", err))
			return
		}
		defer rows.Close()

		var record qotd.OfficialPostRecord
		for rows.Next() {
			record = qotd.OfficialPostRecord{}
			if err := scanOfficialPostRecordDest(rows, &record); err != nil {
				yield(qotd.OfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsNeedingArchive: %w", err))
				return
			}
			if !yield(record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(qotd.OfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsNeedingArchive: %w", err))
		}
	}
}

// UpdateQOTDOfficialPostState updates qotdofficial post state.
func (s *Store) UpdateQOTDOfficialPostState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (_ *qotd.OfficialPostRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update qotd official post state: %w", err)
		}
	}()
	if id <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return nil, fmt.Errorf("state is required")
	}

	row := s.db.QueryRow(ctx, `UPDATE qotd_official_posts
		SET
			state = $1,
			closed_at = $2,
			archived_at = $3,
			last_reconciled_at = NOW(),
			updated_at = NOW()
		WHERE id = $4
		RETURNING
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at`,
		state,
		nullableTime(closedAt),
		nullableTime(archivedAt),
		id,
	)
	record, err := scanOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDOfficialPostState: %w", err)
	}
	return record, nil
}

```

// === FILE: pkg/storage/postgres/qotd_test.go ===
```go
//go:build integration

package postgres

import (
	"context"
	"fmt"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
	"iter"
	"slices"
	"testing"
	"time"
)

func TestQOTDTablesInitialized(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)

	required := []string{
		"qotd_questions",
		"qotd_official_posts",
		"qotd_forum_surfaces",
		"qotd_answer_messages",
	}
	for _, tableName := range required {
		var exists bool
		if err := store.db.QueryRow(context.Background(),
			`SELECT EXISTS(
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = $1
			)`,
			tableName,
		).Scan(&exists); err != nil {
			t.Fatalf("query table %s existence: %v", tableName, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist", tableName)
		}
	}

	legacyTables := []string{"qotd_reply_threads", "qotd_thread_archives", "qotd_message_archives"}
	for _, tableName := range legacyTables {
		var exists bool
		if err := store.db.QueryRow(context.Background(),
			`SELECT EXISTS(
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = $1
			)`,
			tableName,
		).Scan(&exists); err != nil {
			t.Fatalf("query legacy table %s existence: %v", tableName, err)
		}
		if exists {
			t.Fatalf("expected legacy table %s to be removed", tableName)
		}
	}

	legacyColumns := []struct {
		tableName  string
		columnName string
	}{
		{tableName: "qotd_official_posts", columnName: "response_channel_id_snapshot"},
		{tableName: "qotd_official_posts", columnName: "is_pinned"},
	}
	for _, legacyColumn := range legacyColumns {
		var exists bool
		if err := store.db.QueryRow(context.Background(),
			`SELECT EXISTS(
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = $1
				  AND column_name = $2
			)`,
			legacyColumn.tableName,
			legacyColumn.columnName,
		).Scan(&exists); err != nil {
			t.Fatalf("query legacy column %s.%s existence: %v", legacyColumn.tableName, legacyColumn.columnName, err)
		}
		if exists {
			t.Fatalf("expected legacy column %s.%s to be removed", legacyColumn.tableName, legacyColumn.columnName)
		}
	}

	var publishedOnceExists bool
	if err := store.db.QueryRow(context.Background(),
		`SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'qotd_questions'
			  AND column_name = 'published_once_at'
		)`,
	).Scan(&publishedOnceExists); err != nil {
		t.Fatalf("query qotd_questions.published_once_at existence: %v", err)
	}
	if !publishedOnceExists {
		t.Fatal("expected qotd_questions.published_once_at to exist")
	}
}

func TestInitResetsQOTDQuestionSequenceWhenTableEmpty(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	first, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "First question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	if first.DisplayID != 1 {
		t.Fatalf("expected fresh isolated database to start question display IDs at 1, got %d", first.DisplayID)
	}

	if err := store.DeleteQOTDQuestion(ctx, "g1", first.ID); err != nil {
		t.Fatalf("DeleteQOTDQuestion() failed: %v", err)
	}
	if err := store.Init(); err != nil {
		t.Fatalf("Init() after emptying qotd_questions failed: %v", err)
	}

	second, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Second question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}
	if second.DisplayID != 1 {
		t.Fatalf("expected question display ID sequence to reset to 1 after Init() on an empty table, got %d", second.DisplayID)
	}
}

func TestDeleteQOTDQuestionReindexesDisplayIDs(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	first, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "First question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	second, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Second question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}
	if first.DisplayID != 1 || second.DisplayID != 2 {
		t.Fatalf("expected sequential visible ids after create, got first=%d second=%d", first.DisplayID, second.DisplayID)
	}

	if err := store.DeleteQOTDQuestion(ctx, "g1", first.ID); err != nil {
		t.Fatalf("DeleteQOTDQuestion() failed: %v", err)
	}

	remainingSeq, err := store.ListQOTDQuestions(ctx, "g1", "default")
	remaining := collectQuestions(t, remainingSeq)
	if err != nil {
		t.Fatalf("ListQOTDQuestions() failed: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected one remaining question, got %+v", remaining)
	}
	if remaining[0].ID != second.ID {
		t.Fatalf("expected second question to remain, got %+v", remaining[0])
	}
	if remaining[0].DisplayID != 1 {
		t.Fatalf("expected remaining question display id to be renumbered to 1, got %d", remaining[0].DisplayID)
	}

	third, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Third question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(third) failed: %v", err)
	}
	if third.DisplayID != 2 {
		t.Fatalf("expected next visible id to continue from 2 after reindex, got %d", third.DisplayID)
	}
}

func TestReserveNextQOTDQuestionUsesQueueOrder(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	if _, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Second question",
		Status:        "ready",
		QueuePosition: 2,
	}); err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}
	first, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "First question",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	reserved, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", publishDate, qotd.QuestionSelectorQueue)
	if err != nil {
		t.Fatalf("ReserveNextQOTDQuestion() failed: %v", err)
	}
	if reserved == nil {
		t.Fatal("expected a reserved question record")
	}
	if reserved.ID != first.ID {
		t.Fatalf("expected lowest queue position to be reserved first, got id=%d want=%d", reserved.ID, first.ID)
	}
	if reserved.Status != "reserved" {
		t.Fatalf("expected reserved status, got %q", reserved.Status)
	}
	if reserved.ScheduledForDateUTC == nil || !reserved.ScheduledForDateUTC.Equal(publishDate) {
		t.Fatalf("expected scheduled publish date %s, got %+v", publishDate.Format(time.RFC3339), reserved.ScheduledForDateUTC)
	}
}

func TestReserveNextQOTDQuestionSkipsPublishedOnceQuestion(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()
	publishedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)

	first, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Already published question",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	first.PublishedOnceAt = &publishedAt
	if first, err = store.UpdateQOTDQuestion(ctx, *first); err != nil {
		t.Fatalf("UpdateQOTDQuestion(first) failed: %v", err)
	}

	second, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Still publishable question",
		Status:        "ready",
		QueuePosition: 2,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	reserved, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", publishDate, qotd.QuestionSelectorQueue)
	if err != nil {
		t.Fatalf("ReserveNextQOTDQuestion() failed: %v", err)
	}
	if reserved == nil || reserved.ID != second.ID {
		t.Fatalf("expected scheduled reservation to skip already-published question, got %+v", reserved)
	}
}

func TestReserveNextReadyQOTDQuestionSkipsPublishedOnceQuestion(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()
	publishedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)

	first, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Already published question",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	first.PublishedOnceAt = &publishedAt
	if first, err = store.UpdateQOTDQuestion(ctx, *first); err != nil {
		t.Fatalf("UpdateQOTDQuestion(first) failed: %v", err)
	}

	second, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Still publishable question",
		Status:        "ready",
		QueuePosition: 2,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}

	reserved, err := store.ReserveNextReadyQOTDQuestion(ctx, "g1", "default", qotd.QuestionSelectorQueue)
	if err != nil {
		t.Fatalf("ReserveNextReadyQOTDQuestion() failed: %v", err)
	}
	if reserved == nil || reserved.ID != second.ID {
		t.Fatalf("expected manual reservation to skip already-published question, got %+v", reserved)
	}
}

// TestReserveNextReadyQOTDQuestionRandomCoversAllReadyQuestions exercises
// random selection by reserving repeatedly until the pool is exhausted. Any
// eligible question must eventually be picked, otherwise the random selector
// is silently degenerating to a deterministic order.
func TestReserveNextReadyQOTDQuestionRandomCoversAllReadyQuestions(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	const totalQuestions = 5
	createdIDs := make(map[int64]struct{}, totalQuestions)
	for i := 0; i < totalQuestions; i++ {
		question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
			GuildID:       "g1",
			DeckID:        "default",
			Body:          fmt.Sprintf("Question %d", i+1),
			Status:        "ready",
			QueuePosition: int64(i + 1),
		})
		if err != nil {
			t.Fatalf("CreateQOTDQuestion(%d) failed: %v", i+1, err)
		}
		createdIDs[question.ID] = struct{}{}
	}

	pickedIDs := make(map[int64]struct{}, totalQuestions)
	for i := 0; i < totalQuestions; i++ {
		picked, err := store.ReserveNextReadyQOTDQuestion(ctx, "g1", "default", qotd.QuestionSelectorRandom)
		if err != nil {
			t.Fatalf("ReserveNextReadyQOTDQuestion(random, iteration %d) failed: %v", i, err)
		}
		if picked == nil {
			t.Fatalf("expected random reservation to find a ready question on iteration %d", i)
		}
		if _, known := createdIDs[picked.ID]; !known {
			t.Fatalf("random reservation returned a question id not in the created set: %d", picked.ID)
		}
		if _, dup := pickedIDs[picked.ID]; dup {
			t.Fatalf("random reservation returned the same question id twice: %d", picked.ID)
		}
		pickedIDs[picked.ID] = struct{}{}
	}

	if len(pickedIDs) != totalQuestions {
		t.Fatalf("expected random reservation to eventually cover every ready question, got %d/%d", len(pickedIDs), totalQuestions)
	}

	// With every ready question now reserved, the random selector must
	// degrade to nil (no eligible rows left) rather than ignore the WHERE
	// clause.
	exhausted, err := store.ReserveNextReadyQOTDQuestion(ctx, "g1", "default", qotd.QuestionSelectorRandom)
	if err != nil {
		t.Fatalf("ReserveNextReadyQOTDQuestion(random, exhausted) failed: %v", err)
	}
	if exhausted != nil {
		t.Fatalf("expected no question available after pool was exhausted, got %+v", exhausted)
	}
}

// TestCreateQOTDOfficialPostProvisioningAssignsMonotonicPublishOrdinal is the
// load-bearing invariant for the visible thread numbering: each provisioning
// insert must increment publish_ordinal per (guild_id, deck_id), never
// recycling a value, even across decks within the same guild.
func TestCreateQOTDOfficialPostProvisioningAssignsMonotonicPublishOrdinal(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	// The scheduled publish date unique index is per (guild_id,
	// publish_date_utc), so tests that exercise both decks within one
	// guild must hand out distinct days. We allocate from a monotonically
	// increasing counter and let each deck take whatever slot comes next.
	dayCounter := 0
	makePost := func(deckID string) *qotd.OfficialPostRecord {
		dayCounter++
		question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
			GuildID:       "g1",
			DeckID:        deckID,
			Body:          fmt.Sprintf("Question %s/%d", deckID, dayCounter),
			Status:        "ready",
			QueuePosition: int64(dayCounter),
		})
		if err != nil {
			t.Fatalf("CreateQOTDQuestion(%s/%d) failed: %v", deckID, dayCounter, err)
		}
		publishDate := time.Date(2026, 5, dayCounter, 0, 0, 0, 0, time.UTC)
		post, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
			GuildID:              "g1",
			DeckID:               deckID,
			DeckNameSnapshot:     deckID,
			QuestionID:           question.ID,
			PublishMode:          "scheduled",
			ConsumeAutomaticSlot: true,
			PublishDateUTC:       publishDate,
			State:                "provisioning",
			ChannelID:            "123456789012345678",
			QuestionTextSnapshot: question.Body,
			GraceUntil:           publishDate.Add(24 * time.Hour),
			ArchiveAt:            publishDate.Add(48 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateQOTDOfficialPostProvisioning(%s/%d) failed: %v", deckID, dayCounter, err)
		}
		return post
	}

	deckADay1 := makePost("deck-a")
	deckADay2 := makePost("deck-a")
	deckBDay1 := makePost("deck-b")
	deckADay3 := makePost("deck-a")
	deckBDay2 := makePost("deck-b")

	if deckADay1.PublishOrdinal != 1 {
		t.Fatalf("expected deck-a first publish ordinal=1, got %d", deckADay1.PublishOrdinal)
	}
	if deckADay2.PublishOrdinal != 2 {
		t.Fatalf("expected deck-a second publish ordinal=2, got %d", deckADay2.PublishOrdinal)
	}
	if deckADay3.PublishOrdinal != 3 {
		t.Fatalf("expected deck-a third publish ordinal=3, got %d", deckADay3.PublishOrdinal)
	}
	if deckBDay1.PublishOrdinal != 1 {
		t.Fatalf("expected deck-b first publish ordinal=1 (sequence is per-deck), got %d", deckBDay1.PublishOrdinal)
	}
	if deckBDay2.PublishOrdinal != 2 {
		t.Fatalf("expected deck-b second publish ordinal=2, got %d", deckBDay2.PublishOrdinal)
	}
}

// TestCreateQOTDOfficialPostProvisioningOrdinalSurvivesUpdates locks down
// the contract that the publish ordinal is assigned exactly once at
// provisioning and is not mutated by subsequent state transitions
// (FinalizeQOTDOfficialPost, UpdateQOTDOfficialPostState,
// UpdateQOTDOfficialPostProgress). Resume flows depend on this stability
// because they re-derive the visible thread name from the persisted ordinal.
func TestCreateQOTDOfficialPostProvisioningOrdinalSurvivesUpdates(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()
	publishDate := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)

	question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Stable ordinal question",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}

	post, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		ConsumeAutomaticSlot: true,
		PublishDateUTC:       publishDate,
		State:                "provisioning",
		ChannelID:            "123456789012345678",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           publishDate.Add(24 * time.Hour),
		ArchiveAt:            publishDate.Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	originalOrdinal := post.PublishOrdinal
	if originalOrdinal != 1 {
		t.Fatalf("expected first provisioning to receive ordinal=1, got %d", originalOrdinal)
	}

	// Each lifecycle write below has historically refreshed every
	// returnable column. We assert ordinal stays put through all of them.
	progressed, err := store.UpdateQOTDOfficialPostProgress(ctx, post.ID, qotd.OfficialPostRecord{
		QuestionListThreadID:       "qlist-thread",
		QuestionListEntryMessageID: "qlist-entry",
		DiscordThreadID:            "thread-1",
		DiscordStarterMessageID:    "starter-1",
		AnswerChannelID:            "thread-1",
	})
	if err != nil {
		t.Fatalf("UpdateQOTDOfficialPostProgress() failed: %v", err)
	}
	if progressed.PublishOrdinal != originalOrdinal {
		t.Fatalf("UpdateQOTDOfficialPostProgress mutated publish_ordinal: %d -> %d", originalOrdinal, progressed.PublishOrdinal)
	}

	finalized, err := store.FinalizeQOTDOfficialPost(ctx, qotd.FinalizeOfficialPostParams{
		ID:                         post.ID,
		QuestionListThreadID:       "qlist-thread",
		QuestionListEntryMessageID: "qlist-entry",
		DiscordThreadID:            "thread-1",
		StarterMessageID:           "starter-1",
		AnswerChannelID:            "thread-1",
		PublishedAt:                publishDate.Add(13 * time.Hour),
	})
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if finalized.PublishOrdinal != originalOrdinal {
		t.Fatalf("FinalizeQOTDOfficialPost mutated publish_ordinal: %d -> %d", originalOrdinal, finalized.PublishOrdinal)
	}

	stateUpdated, err := store.UpdateQOTDOfficialPostState(ctx, post.ID, "current", nil, nil)
	if err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState() failed: %v", err)
	}
	if stateUpdated.PublishOrdinal != originalOrdinal {
		t.Fatalf("UpdateQOTDOfficialPostState mutated publish_ordinal: %d -> %d", originalOrdinal, stateUpdated.PublishOrdinal)
	}

	reread, err := store.GetQOTDOfficialPostByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByID() failed: %v", err)
	}
	if reread == nil || reread.PublishOrdinal != originalOrdinal {
		t.Fatalf("GetQOTDOfficialPostByID returned different ordinal: want %d, got %+v", originalOrdinal, reread)
	}
}

// TestCreateQOTDOfficialPostProvisioningOrdinalSharedAcrossPublishModes
// guards the invariant that scheduled and manual publishes draw from the
// same per-deck sequence. Otherwise the visible thread numbering would
// reset whenever an admin used /qotd-publish (manual) instead of the
// scheduler.
func TestCreateQOTDOfficialPostProvisioningOrdinalSharedAcrossPublishModes(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	makePost := func(mode string, day int) *qotd.OfficialPostRecord {
		question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
			GuildID:       "g1",
			DeckID:        "default",
			Body:          fmt.Sprintf("Question %s/%d", mode, day),
			Status:        "ready",
			QueuePosition: int64(day),
		})
		if err != nil {
			t.Fatalf("CreateQOTDQuestion(%s/%d) failed: %v", mode, day, err)
		}
		publishDate := time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC)
		post, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
			GuildID:              "g1",
			DeckID:               "default",
			DeckNameSnapshot:     "Default",
			QuestionID:           question.ID,
			PublishMode:          mode,
			ConsumeAutomaticSlot: mode == "scheduled",
			PublishDateUTC:       publishDate,
			State:                "provisioning",
			ChannelID:            "123456789012345678",
			QuestionTextSnapshot: question.Body,
			GraceUntil:           publishDate.Add(24 * time.Hour),
			ArchiveAt:            publishDate.Add(48 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateQOTDOfficialPostProvisioning(%s/%d) failed: %v", mode, day, err)
		}
		return post
	}

	scheduled := makePost("scheduled", 1)
	manual := makePost("manual", 2)
	scheduledNext := makePost("scheduled", 3)

	if scheduled.PublishOrdinal != 1 {
		t.Fatalf("expected first scheduled publish ordinal=1, got %d", scheduled.PublishOrdinal)
	}
	if manual.PublishOrdinal != 2 {
		t.Fatalf("expected manual publish to take the next ordinal=2 (shared sequence), got %d", manual.PublishOrdinal)
	}
	if scheduledNext.PublishOrdinal != 3 {
		t.Fatalf("expected scheduled publish after manual to take ordinal=3, got %d", scheduledNext.PublishOrdinal)
	}
}

func TestReorderQOTDQuestionsAllowsQueuePositionSwap(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	first, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "First question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	second, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Second question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}

	if err := store.ReorderQOTDQuestions(ctx, "g1", "default", []int64{second.ID, first.ID}); err != nil {
		t.Fatalf("ReorderQOTDQuestions() failed: %v", err)
	}

	questionsSeq, err := store.ListQOTDQuestions(ctx, "g1", "default")
	questions := collectQuestions(t, questionsSeq)
	if err != nil {
		t.Fatalf("ListQOTDQuestions() failed: %v", err)
	}
	if len(questions) != 2 {
		t.Fatalf("expected two questions after reorder, got %+v", questions)
	}
	if questions[0].ID != second.ID || questions[0].QueuePosition != 1 || questions[0].DisplayID != 1 {
		t.Fatalf("expected second question to move into slot 1, got %+v", questions)
	}
	if questions[1].ID != first.ID || questions[1].QueuePosition != 2 || questions[1].DisplayID != 2 {
		t.Fatalf("expected first question to move into slot 2, got %+v", questions)
	}
}

func TestGetQOTDOfficialPostByDatePrefersPublishedPostAcrossModes(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question one",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	second, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question two",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)
	publishedAt := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)

	manual, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           second.ID,
		PublishMode:          "manual",
		PublishDateUTC:       publishDate,
		State:                "current",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: second.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(manual) failed: %v", err)
	}
	manual, err = store.FinalizeQOTDOfficialPost(ctx, qotd.FinalizeOfficialPostParams{
		ID:                         manual.ID,
		QuestionListThreadID:       "questions-list-thread",
		QuestionListEntryMessageID: "questions-list-entry-manual",
		DiscordThreadID:            "manual-thread",
		StarterMessageID:           "manual-message",
		AnswerChannelID:            "manual-thread",
		PublishedAt:                publishedAt,
	})
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost(manual) failed: %v", err)
	}

	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "provisioning",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(scheduled) failed: %v", err)
	}

	record, err := store.GetQOTDOfficialPostByDate(ctx, "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if record == nil || record.ID != manual.ID {
		t.Fatalf("expected published manual post to win the day lookup, got %+v", record)
	}
	if record.PublishMode != "manual" || record.PublishedAt == nil {
		t.Fatalf("expected published manual post from the same day, got %+v", record)
	}
}

// TestGetQOTDOfficialPostByDateRoundTrip writes a provisioned official post and
// reads it back by (guild, publish date), exercising the scheduled-publish
// lookup path end-to-end against Postgres. It is the behavioral counterpart to
// the static placeholder guard in query_placeholders_test.go: this query
// previously used "?" binds and failed under the pgx driver with SQLSTATE 42601
// ("syntax error at or near \"AND\"").
func TestGetQOTDOfficialPostByDateRoundTrip(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Round-trip question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}

	publishDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	created, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "provisioning",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	got, err := store.GetQOTDOfficialPostByDate(ctx, "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetQOTDOfficialPostByDate() returned no record for the inserted publish date")
	}
	if got.ID != created.ID {
		t.Fatalf("GetQOTDOfficialPostByDate() returned id %d, want %d", got.ID, created.ID)
	}
	if got.GuildID != "g1" || got.DeckID != "default" || !got.PublishDateUTC.Equal(created.PublishDateUTC) {
		t.Fatalf("GetQOTDOfficialPostByDate() returned mismatched record: %+v", got)
	}

	// A date with no official post must miss cleanly (nil, nil) rather than error.
	missing, err := store.GetQOTDOfficialPostByDate(ctx, "g1", publishDate.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(unused date) failed: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected no record for an unused date, got %+v", missing)
	}
}

func TestGetScheduledQOTDOfficialPostByDateIgnoresManualPost(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	scheduledQuestion, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Scheduled question",
		Status:  "reserved",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(scheduled) failed: %v", err)
	}
	manualQuestion, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Manual question",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(manual) failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)
	publishedAt := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)

	manual, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           manualQuestion.ID,
		PublishMode:          "manual",
		PublishDateUTC:       publishDate,
		State:                "current",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: manualQuestion.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(manual) failed: %v", err)
	}
	if _, err := store.FinalizeQOTDOfficialPost(ctx, qotd.FinalizeOfficialPostParams{
		ID:                         manual.ID,
		QuestionListThreadID:       "questions-list-thread",
		QuestionListEntryMessageID: "questions-list-entry-manual",
		DiscordThreadID:            "manual-thread",
		StarterMessageID:           "manual-message",
		AnswerChannelID:            "manual-thread",
		PublishedAt:                publishedAt,
	}); err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost(manual) failed: %v", err)
	}

	scheduled, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           scheduledQuestion.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "provisioning",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: scheduledQuestion.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(scheduled) failed: %v", err)
	}

	record, err := store.GetScheduledQOTDOfficialPostByDate(ctx, "g1", publishDate)
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate() failed: %v", err)
	}
	if record == nil || record.ID != scheduled.ID {
		t.Fatalf("expected scheduled lookup to ignore the manual post, got %+v", record)
	}
	if record.PublishMode != "scheduled" || record.QuestionID != scheduledQuestion.ID {
		t.Fatalf("expected scheduled lookup to return the scheduled slot record, got %+v", record)
	}
	if record.PublishedAt != nil {
		t.Fatalf("expected scheduled lookup to return the scheduled provisioning record before finalize, got %+v", record)
	}
}

func TestReclaimOrphanReservedQOTDQuestionsReleasesPastReservationsWithoutPosts(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	pastDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	todayUTC := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	orphan, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Stuck after crash",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(orphan) failed: %v", err)
	}
	if _, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", pastDate, qotd.QuestionSelectorQueue); err != nil {
		t.Fatalf("ReserveNextQOTDQuestion(orphan) failed: %v", err)
	}

	var freed []int64
	for id, err := range store.ReclaimOrphanReservedQOTDQuestions(ctx, "g1", todayUTC) {
		if err != nil {
			t.Fatalf("ReclaimOrphanReservedQOTDQuestions() failed: %v", err)
		}
		freed = append(freed, id)
	}
	if len(freed) != 1 || freed[0] != orphan.ID {
		t.Fatalf("expected orphan reservation to be released, got %+v", freed)
	}

	restored, err := store.GetQOTDQuestion(ctx, "g1", orphan.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(orphan) failed: %v", err)
	}
	if restored == nil || restored.Status != "ready" {
		t.Fatalf("expected orphan to be restored to ready, got %+v", restored)
	}
	if restored.ScheduledForDateUTC != nil {
		t.Fatalf("expected scheduled_for_date_utc to be cleared on the orphan, got %+v", restored.ScheduledForDateUTC)
	}
}

func TestReclaimOrphanReservedQOTDQuestionsKeepsTodayReservation(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	todayUTC := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Active reservation",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}
	if _, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", todayUTC, qotd.QuestionSelectorQueue); err != nil {
		t.Fatalf("ReserveNextQOTDQuestion() failed: %v", err)
	}

	var freed []int64
	for id, err := range store.ReclaimOrphanReservedQOTDQuestions(ctx, "g1", todayUTC) {
		if err != nil {
			t.Fatalf("ReclaimOrphanReservedQOTDQuestions() failed: %v", err)
		}
		freed = append(freed, id)
	}
	if len(freed) != 0 {
		t.Fatalf("expected today's reservation to stay in place (publish may be in flight), got freed=%+v", freed)
	}

	stored, err := store.GetQOTDQuestion(ctx, "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if stored == nil || stored.Status != "reserved" {
		t.Fatalf("expected today's reservation to remain reserved, got %+v", stored)
	}
}

func TestReclaimOrphanReservedQOTDQuestionsLeavesQuestionsWithLinkedPosts(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	pastDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	todayUTC := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Linked to a real post",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}
	reserved, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", pastDate, qotd.QuestionSelectorQueue)
	if err != nil || reserved == nil {
		t.Fatalf("ReserveNextQOTDQuestion() failed: %v", err)
	}
	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       pastDate,
		State:                "provisioning",
		ChannelID:            "channel-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           pastDate.Add(24 * time.Hour),
		ArchiveAt:            pastDate.Add(48 * time.Hour),
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	var freed []int64
	for id, err := range store.ReclaimOrphanReservedQOTDQuestions(ctx, "g1", todayUTC) {
		if err != nil {
			t.Fatalf("ReclaimOrphanReservedQOTDQuestions() failed: %v", err)
		}
		freed = append(freed, id)
	}
	if len(freed) != 0 {
		t.Fatalf("expected reservations linked to a publish record to be left alone, got %+v", freed)
	}

	stored, err := store.GetQOTDQuestion(ctx, "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if stored == nil || stored.Status != "reserved" {
		t.Fatalf("expected reservation linked to a post to stay reserved, got %+v", stored)
	}
}

func TestQOTDOfficialPostProgressAndPendingRecoveryLifecycle(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question one",
		Status:  "reserved",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}

	official, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		State:                "provisioning",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	progress, err := store.UpdateQOTDOfficialPostProgress(ctx, official.ID, qotd.OfficialPostRecord{
		QuestionListThreadID:    "questions-list-thread",
		DiscordThreadID:         "official-thread-1",
		DiscordStarterMessageID: "starter-message-1",
		AnswerChannelID:         "official-thread-1",
	})
	if err != nil {
		t.Fatalf("UpdateQOTDOfficialPostProgress() failed: %v", err)
	}
	if progress.QuestionListThreadID != "questions-list-thread" || progress.DiscordThreadID != "official-thread-1" || progress.DiscordStarterMessageID != "starter-message-1" {
		t.Fatalf("expected progress update to persist partial artifacts, got %+v", progress)
	}
	if progress.PublishedAt != nil {
		t.Fatalf("expected progress update to keep published_at unset until finalize, got %+v", progress)
	}

	if _, err := store.UpdateQOTDOfficialPostState(ctx, official.ID, "failed", nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState(failed) failed: %v", err)
	}

	var pending []qotd.OfficialPostRecord
	for post, err := range store.ListQOTDOfficialPostsPendingRecovery(ctx, "g1") {
		if err != nil {
			t.Fatalf("ListQOTDOfficialPostsPendingRecovery() failed: %v", err)
		}
		pending = append(pending, post)
	}
	if len(pending) != 1 || pending[0].ID != official.ID {
		t.Fatalf("expected failed provisioning record to be listed for recovery, got %+v", pending)
	}

	finalizedAt := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	finalized, err := store.FinalizeQOTDOfficialPost(ctx, qotd.FinalizeOfficialPostParams{
		ID:                         official.ID,
		QuestionListThreadID:       "questions-list-thread",
		QuestionListEntryMessageID: "list-entry-1",
		DiscordThreadID:            "official-thread-1",
		StarterMessageID:           "starter-message-1",
		AnswerChannelID:            "official-thread-1",
		PublishedAt:                finalizedAt,
	})
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if finalized.PublishedAt == nil || !finalized.PublishedAt.Equal(finalizedAt) {
		t.Fatalf("expected finalize to persist published_at, got %+v", finalized)
	}

	if _, err := store.UpdateQOTDOfficialPostState(ctx, official.ID, "current", nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState(current) failed: %v", err)
	}
	pending = nil
	for post, err := range store.ListQOTDOfficialPostsPendingRecovery(ctx, "g1") {
		if err != nil {
			t.Fatalf("ListQOTDOfficialPostsPendingRecovery(after finalize) failed: %v", err)
		}
		pending = append(pending, post)
	}
	if len(pending) != 0 {
		t.Fatalf("expected finalized post to disappear from pending recovery list, got %+v", pending)
	}
}

func TestDeleteQOTDQuestionsByDecksPreservesOfficialPostHistory(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "deck-a",
		Body:    "Question one",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}
	official, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "deck-a",
		DeckNameSnapshot:     "Deck A",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		State:                "published",
		ChannelID:            "question-channel-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	if err := store.DeleteQOTDQuestionsByDecks(ctx, "g1", []string{"deck-a"}); err != nil {
		t.Fatalf("DeleteQOTDQuestionsByDecks() failed: %v", err)
	}

	deletedQuestion, err := store.GetQOTDQuestion(ctx, "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if deletedQuestion != nil {
		t.Fatalf("expected question to be deleted, got %+v", deletedQuestion)
	}

	questionsSeq, err := store.ListQOTDQuestions(ctx, "g1", "deck-a")
	questions := collectQuestions(t, questionsSeq)
	if err != nil {
		t.Fatalf("ListQOTDQuestions() failed: %v", err)
	}
	if len(questions) != 0 {
		t.Fatalf("expected deck questions to be deleted, got %+v", questions)
	}

	preservedOfficial, err := store.GetQOTDOfficialPostByDate(ctx, "g1", official.PublishDateUTC)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if preservedOfficial == nil {
		t.Fatal("expected official post history to remain after deleting deck questions")
	}
	if preservedOfficial.QuestionID != question.ID || preservedOfficial.QuestionTextSnapshot != question.Body {
		t.Fatalf("expected official post snapshot to remain intact, got %+v", preservedOfficial)
	}
}

func TestDeleteQOTDOfficialPostsByDeckRemovesOnlyMatchingDeck(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	deckAQuestion, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "deck-a",
		Body:    "Deck A question",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(deck-a) failed: %v", err)
	}
	deckBQuestion, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "deck-b",
		Body:    "Deck B question",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(deck-b) failed: %v", err)
	}

	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "deck-a",
		DeckNameSnapshot:     "Deck A",
		QuestionID:           deckAQuestion.ID,
		PublishMode:          "manual",
		PublishDateUTC:       time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		State:                "current",
		ChannelID:            "question-channel-a",
		QuestionTextSnapshot: deckAQuestion.Body,
		GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(deck-a) failed: %v", err)
	}
	deckBOfficial, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "deck-b",
		DeckNameSnapshot:     "Deck B",
		QuestionID:           deckBQuestion.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC),
		State:                "provisioning",
		ChannelID:            "question-channel-b",
		QuestionTextSnapshot: deckBQuestion.Body,
		GraceUntil:           time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 6, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(deck-b) failed: %v", err)
	}

	deleted, err := store.DeleteQOTDOfficialPostsByDeck(ctx, "g1", "deck-a")
	if err != nil {
		t.Fatalf("DeleteQOTDOfficialPostsByDeck() failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected one deck-a official post to be deleted, got %d", deleted)
	}

	deletedRecord, err := store.GetQOTDOfficialPostByDate(ctx, "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(deck-a) failed: %v", err)
	}
	if deletedRecord != nil {
		t.Fatalf("expected deck-a official post to be removed, got %+v", deletedRecord)
	}

	preservedRecord, err := store.GetQOTDOfficialPostByDate(ctx, "g1", time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(deck-b) failed: %v", err)
	}
	if preservedRecord == nil || preservedRecord.ID != deckBOfficial.ID {
		t.Fatalf("expected deck-b official post to remain, got %+v", preservedRecord)
	}
	if preservedRecord.DeckID != "deck-b" {
		t.Fatalf("expected only deck-a official posts to be deleted, got %+v", preservedRecord)
	}
}

func TestListQOTDOfficialPostsByDateReturnsAllMatchingRecords(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	questionA, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question A",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(questionA) failed: %v", err)
	}
	questionB, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question B",
		Status:  "reserved",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(questionB) failed: %v", err)
	}

	publishDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 5, 8, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 5, 9, 12, 43, 0, 0, time.UTC)

	first, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           questionA.ID,
		PublishMode:          "manual",
		PublishDateUTC:       publishDate,
		State:                "current",
		ChannelID:            "question-channel",
		QuestionTextSnapshot: questionA.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(first) failed: %v", err)
	}
	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           questionB.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "failed",
		ChannelID:            "question-channel",
		QuestionTextSnapshot: questionB.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(second) failed: %v", err)
	}

	postsIter, err := store.ListQOTDOfficialPostsByDate(ctx, "g1", publishDate)
	if err != nil {
		t.Fatalf("ListQOTDOfficialPostsByDate() failed: %v", err)
	}
	posts := slices.Collect(postsIter)
	if len(posts) != 2 {
		t.Fatalf("expected two official posts for the same date, got %+v", posts)
	}
	if posts[0].ID == 0 || posts[1].ID == 0 {
		t.Fatalf("expected persisted official post ids, got %+v", posts)
	}
	if posts[0].ID == posts[1].ID {
		t.Fatalf("expected distinct official post records, got %+v", posts)
	}
	if posts[0].ID != first.ID && posts[1].ID != first.ID {
		t.Fatalf("expected first post to be present in list, got %+v", posts)
	}
}

func TestDeleteQOTDOfficialPostByIDRemovesOnlyTargetRecord(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	questionA, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question A",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(questionA) failed: %v", err)
	}
	questionB, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question B",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(questionB) failed: %v", err)
	}

	publishDateA := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	publishDateB := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 5, 9, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 5, 10, 12, 43, 0, 0, time.UTC)

	target, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           questionA.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDateA,
		State:                "abandoned",
		ChannelID:            "question-channel",
		QuestionTextSnapshot: questionA.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(target) failed: %v", err)
	}
	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           questionB.ID,
		PublishMode:          "manual",
		PublishDateUTC:       publishDateB,
		State:                "current",
		ChannelID:            "question-channel",
		QuestionTextSnapshot: questionB.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(other) failed: %v", err)
	}

	if err := store.DeleteQOTDOfficialPostByID(ctx, target.ID); err != nil {
		t.Fatalf("DeleteQOTDOfficialPostByID() failed: %v", err)
	}

	deleted, err := store.GetQOTDOfficialPostByID(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByID(target) failed: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected target official post to be deleted, got %+v", deleted)
	}

	preserved, err := store.GetQOTDOfficialPostByDate(ctx, "g1", publishDateB)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(other) failed: %v", err)
	}
	if preserved == nil || preserved.QuestionID != questionB.ID {
		t.Fatalf("expected non-target official post to remain, got %+v", preserved)
	}
}

func newTempStore(t *testing.T) *Store {
	ctx := context.Background()
	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		t.Skip("skipping integration test, testdb not configured")
	}
	pool, cleanup, err := testdb.OpenIsolatedDatabase(ctx, baseDSN)
	if err != nil {
		t.Fatalf("failed to open isolated database: %v", err)
	}
	t.Cleanup(func() { _ = cleanup() })
	store, _ := NewStore(pool, nil)
	return store
}
func collectQuestions(t *testing.T, seq iter.Seq2[qotd.QuestionRecord, error]) []qotd.QuestionRecord {
	var res []qotd.QuestionRecord
	for q, err := range seq {
		if err != nil {
			t.Fatalf("iteration error: %v", err)
		}
		res = append(res, q)
	}
	return res
}

func TestQOTDSurfaces(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	// 1. Get non-existent surface
	surface, err := store.GetQOTDSurfaceByDeck(ctx, "g1", "deck-1")
	if err != nil {
		t.Fatalf("GetQOTDSurfaceByDeck failed: %v", err)
	}
	if surface != nil {
		t.Fatalf("expected nil surface, got %+v", surface)
	}

	// 2. Upsert a surface
	inserted, err := store.UpsertQOTDSurface(ctx, qotd.SurfaceRecord{
		GuildID:              "g1",
		DeckID:               "deck-1",
		ChannelID:            "channel-1",
		QuestionListThreadID: "thread-1",
	})
	if err != nil {
		t.Fatalf("UpsertQOTDSurface failed: %v", err)
	}
	if inserted.GuildID != "g1" || inserted.DeckID != "deck-1" || inserted.ChannelID != "channel-1" || inserted.QuestionListThreadID != "thread-1" {
		t.Fatalf("upserted record mismatch: %+v", inserted)
	}

	// 3. Get the upserted surface
	retrieved, err := store.GetQOTDSurfaceByDeck(ctx, "g1", "deck-1")
	if err != nil {
		t.Fatalf("GetQOTDSurfaceByDeck failed: %v", err)
	}
	if retrieved == nil || retrieved.ID != inserted.ID {
		t.Fatalf("expected retrieved surface with ID %d, got %+v", inserted.ID, retrieved)
	}

	// 4. Update the surface via Upsert
	updated, err := store.UpsertQOTDSurface(ctx, qotd.SurfaceRecord{
		GuildID:              "g1",
		DeckID:               "deck-1",
		ChannelID:            "channel-updated",
		QuestionListThreadID: "thread-updated",
	})
	if err != nil {
		t.Fatalf("UpsertQOTDSurface update failed: %v", err)
	}
	if updated.ID != inserted.ID || updated.ChannelID != "channel-updated" || updated.QuestionListThreadID != "thread-updated" {
		t.Fatalf("updated record mismatch: %+v", updated)
	}

	// 5. Delete the surface
	err = store.DeleteQOTDSurfaceByDeck(ctx, "g1", "deck-1")
	if err != nil {
		t.Fatalf("DeleteQOTDSurfaceByDeck failed: %v", err)
	}

	// 6. Verify deletion
	deleted, err := store.GetQOTDSurfaceByDeck(ctx, "g1", "deck-1")
	if err != nil {
		t.Fatalf("GetQOTDSurfaceByDeck after delete failed: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected nil surface after delete, got %+v", deleted)
	}
}

func TestQOTDAnswerMessages(t *testing.T) {
	t.Parallel()
	store := newTempStore(t)
	ctx := context.Background()

	// First we need a question and an official post to link answer messages
	question, err := store.CreateQOTDQuestion(ctx, qotd.QuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Answer message question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	post, err := store.CreateQOTDOfficialPostProvisioning(ctx, qotd.OfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		ConsumeAutomaticSlot: true,
		PublishDateUTC:       publishDate,
		State:                "provisioning",
		ChannelID:            "123456789012345678",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           publishDate.Add(24 * time.Hour),
		ArchiveAt:            publishDate.Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning failed: %v", err)
	}

	// 1. Get non-existent answer message
	msg, err := store.GetQOTDAnswerMessageByOfficialPostAndUser(ctx, post.ID, "user-1")
	if err != nil {
		t.Fatalf("GetQOTDAnswerMessageByOfficialPostAndUser failed: %v", err)
	}
	if msg != nil {
		t.Fatalf("expected nil answer message, got %+v", msg)
	}

	// 2. Create answer message
	created, err := store.CreateQOTDAnswerMessage(ctx, qotd.AnswerMessageRecord{
		GuildID:         "g1",
		OfficialPostID:  post.ID,
		UserID:          "user-1",
		State:           "provisioning",
		AnswerChannelID: "thread-1",
	})
	if err != nil {
		t.Fatalf("CreateQOTDAnswerMessage failed: %v", err)
	}
	if created.GuildID != "g1" || created.OfficialPostID != post.ID || created.UserID != "user-1" || created.State != "provisioning" {
		t.Fatalf("created record mismatch: %+v", created)
	}

	// 3. Finalize answer message
	finalized, err := store.FinalizeQOTDAnswerMessage(ctx, created.ID, "msg-1234")
	if err != nil {
		t.Fatalf("FinalizeQOTDAnswerMessage failed: %v", err)
	}
	if finalized.ID != created.ID || finalized.DiscordMessageID != "msg-1234" {
		t.Fatalf("finalized record mismatch: %+v", finalized)
	}

	// 4. Get by post and user
	retrieved, err := store.GetQOTDAnswerMessageByOfficialPostAndUser(ctx, post.ID, "user-1")
	if err != nil {
		t.Fatalf("GetQOTDAnswerMessageByOfficialPostAndUser failed: %v", err)
	}
	if retrieved == nil || retrieved.ID != created.ID {
		t.Fatalf("expected retrieved record, got %+v", retrieved)
	}

	// 5. Update state
	closedAt := time.Now().UTC().Truncate(time.Microsecond)
	archivedAt := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	updated, err := store.UpdateQOTDAnswerMessageState(ctx, created.ID, "archived", &closedAt, &archivedAt)
	if err != nil {
		t.Fatalf("UpdateQOTDAnswerMessageState failed: %v", err)
	}
	if updated.State != "archived" || updated.ClosedAt == nil || !updated.ClosedAt.Equal(closedAt) || updated.ArchivedAt == nil || !updated.ArchivedAt.Equal(archivedAt) {
		t.Fatalf("updated record mismatch: %+v", updated)
	}

	// 6. List answer messages by official post
	listSeq, err := store.ListQOTDAnswerMessagesByOfficialPost(ctx, post.ID)
	if err != nil {
		t.Fatalf("ListQOTDAnswerMessagesByOfficialPost failed: %v", err)
	}
	var listed []qotd.AnswerMessageRecord
	for m, err := range listSeq {
		if err != nil {
			t.Fatalf("iteration failed: %v", err)
		}
		listed = append(listed, m)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed answer messages mismatch: expected 1 with ID %d, got %+v", created.ID, listed)
	}
}

```

// === FILE: pkg/storage/postgres/query_placeholders_test.go ===
```go
package postgres

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestStorageQueriesUsePositionalPlaceholders fails when any SQL string in the
// storage package uses a MySQL/SQLite-style "?" bind placeholder. The package
// talks to PostgreSQL through the pgx stdlib driver, which accepts only
// positional "$1, $2" placeholders; a "?" is forwarded to the server verbatim
// and fails at execution time with SQLSTATE 42601 (syntax_error) — for example
// `syntax error at or near "AND"` when the token after the "?" is a keyword.
//
// The check is a static source scan with no build tag, so it runs under the
// default `go test ./...`. The behavioral store tests are gated behind
// //go:build integration and a configured DATABASE_URL, so they are skipped
// wherever no live database is reachable — which is how "?" placeholders reached
// production unnoticed. This guard closes that gap.
//
// Every "?" inside an SQL string is treated as a bind placeholder. A future
// query needing the PostgreSQL JSONB key-existence operator should use the
// jsonb_exists() function form instead of "?", so this guard stays reliable.
func TestStorageQueriesUsePositionalPlaceholders(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob storage package go files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no go files found in storage package working directory")
	}

	looksLikeSQL := func(literal string) bool {
		upper := strings.ToUpper(literal)
		for _, keyword := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WHERE", "VALUES"} {
			if strings.Contains(upper, keyword) {
				return true
			}
		}
		return false
	}

	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || !looksLikeSQL(lit.Value) {
				return true
			}
			start := fset.Position(lit.Pos())
			for offset, line := range strings.Split(lit.Value, "\n") {
				if strings.Contains(line, "?") {
					t.Errorf("%s:%d: SQL uses a '?' bind placeholder; the pgx/PostgreSQL driver requires positional '$N' placeholders", start.Filename, start.Line+offset)
				}
			}
			return true
		})
	}
}

```

// === FILE: pkg/storage/postgres/schema.go ===
```go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var requiredSchemaTables = []string{
	"messages",
	"member_joins",
	"avatars_current",
	"avatars_history",
	"messages_history",
	"message_version_counters",
	"guild_meta",
	"runtime_meta",
	"moderation_cases",
	"moderation_warnings",
	"roles_current",
	"persistent_cache",
	"daily_message_metrics",
	"daily_reaction_metrics",
	"daily_member_leaves",
	"ticket_sequences",
	"guild_configs",
	"user_preferences",
	"qotd_questions", // included since we need it in reset
}

// ColumnDef represents an expected schema column.
type ColumnDef struct {
	Name     string
	DataType string // e.g., "text", "timestamp with time zone", "boolean"
}

// requiredSchemaColumns maps tables to their expected columns and types to detect parametric deletion and type regressions.
var requiredSchemaColumns = map[string][]ColumnDef{
	"member_joins": {
		{Name: "last_seen_at", DataType: "timestamp with time zone"},
		{Name: "is_bot", DataType: "boolean"},
		{Name: "left_at", DataType: "timestamp with time zone"},
	},
	"avatars_current": {
		{Name: "guild_id", DataType: "text"},
		{Name: "updated_at", DataType: "timestamp with time zone"},
	},
}

// Init ensures the migrated schema is present and primes per-deployment state.
func (s *Store) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.log().Info("storage subsystem initializing", slog.String("phase", "schema_verify"))

	if err := s.ensureMemberJoinColumns(ctx); err != nil {
		s.log().Error("failed to ensure member join columns", slog.String("error", err.Error()))
		return fmt.Errorf("ensure member join state columns: %w", err)
	}
	if err := s.validateSchema(ctx); err != nil {
		s.log().Error("schema validation failed", slog.String("error", err.Error()))
		return fmt.Errorf("validate schema: %w", err)
	}
	if err := s.resetQOTDQuestionSequenceWhenEmpty(ctx); err != nil {
		s.log().Error("failed to reset QOTD sequence", slog.String("error", err.Error()))
		return fmt.Errorf("reset qotd question sequence: %w", err)
	}

	s.log().Info("storage subsystem initialized successfully")
	return nil
}

func (s *Store) ensureMemberJoinColumns(ctx context.Context) (err error) {
	missing, err := s.missingColumns(ctx, "member_joins", []string{"last_seen_at", "is_bot", "left_at"})
	if err != nil {
		return fmt.Errorf("Store.ensureMemberJoinColumns: %w", err)
	}
	if len(missing) == 0 {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin member_joins bootstrap tx: %w", err)
	}
	defer func() {
		// Intercept rollback explicitly evaluating tx closed errors
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	if _, err := tx.Exec(ctx, `ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`); err != nil {
		return fmt.Errorf("add member_joins.last_seen_at column: %w", err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS is_bot BOOLEAN`); err != nil {
		return fmt.Errorf("add member_joins.is_bot column: %w", err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ`); err != nil {
		return fmt.Errorf("add member_joins.left_at column: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE member_joins
		   SET last_seen_at = COALESCE(last_seen_at, joined_at)
		 WHERE last_seen_at IS NULL
	`); err != nil {
		return fmt.Errorf("backfill member_joins.last_seen_at: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_member_joins_active ON member_joins(guild_id, left_at)`); err != nil {
		return fmt.Errorf("create member_joins active index: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit member_joins bootstrap: %w", err)
	}
	return nil
}

func (s *Store) missingColumns(ctx context.Context, table string, columns []string) ([]string, error) {
	missing := make([]string, 0)
	for _, column := range columns {
		var exists bool
		if err := s.db.QueryRow(
			ctx,
			`SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = $1
				  AND column_name = $2
			)`,
			table,
			column,
		).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check %s.%s existence: %w", table, column, err)
		}
		if !exists {
			missing = append(missing, column)
		}
	}
	return missing, nil
}

// validateSchema checks table presence, and strongly types columns against regressions.
func (s *Store) validateSchema(ctx context.Context) error {
	missingTables := make([]string, 0)
	for _, table := range requiredSchemaTables {
		var regclass *string
		if err := s.db.QueryRow(ctx, `SELECT to_regclass($1)`, table).Scan(&regclass); err != nil {
			return fmt.Errorf("check table %s existence: %w", table, err)
		}
		if regclass == nil || strings.TrimSpace(*regclass) == "" {
			missingTables = append(missingTables, table)
		}
	}
	if len(missingTables) > 0 {
		return fmt.Errorf("missing migrated tables (%s); apply postgres migrations before initializing store", strings.Join(missingTables, ", "))
	}

	var schemaErrors []string
	for table, columns := range requiredSchemaColumns {
		for _, col := range columns {
			var dataType string
			err := s.db.QueryRow(
				ctx,
				`SELECT data_type
				 FROM information_schema.columns
				 WHERE table_schema = current_schema()
				   AND table_name = $1
				   AND column_name = $2`,
				table,
				col.Name,
			).Scan(&dataType)

			if err != nil {
				if err == pgx.ErrNoRows {
					schemaErrors = append(schemaErrors, fmt.Sprintf("column %s.%s missing", table, col.Name))
					continue
				}
				return fmt.Errorf("check column %s.%s existence: %w", table, col.Name, err)
			}

			if dataType != col.DataType {
				schemaErrors = append(schemaErrors, fmt.Sprintf("column %s.%s type mismatch: expected %s, got %s", table, col.Name, col.DataType, dataType))
			}
		}
	}

	if len(schemaErrors) > 0 {
		return fmt.Errorf("schema validation failed: %s", strings.Join(schemaErrors, "; "))
	}
	return nil
}

func (s *Store) resetQOTDQuestionSequenceWhenEmpty(ctx context.Context) error {
	var hasRows bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM qotd_questions LIMIT 1)`).Scan(&hasRows); err != nil {
		return fmt.Errorf("Store.resetQOTDQuestionSequenceWhenEmpty: %w", err)
	}
	if hasRows {
		return nil
	}

	if _, err := s.db.Exec(ctx, `
		SELECT setval(
			pg_get_serial_sequence(format('%I.%I', current_schema(), 'qotd_questions'), 'id'),
			1,
			false
		)
	`); err != nil {
		return err
	}
	return nil
}

```

// === FILE: pkg/storage/postgres/schema_test.go ===
```go
package postgres

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDB starts a postgres testcontainer, creates the full DDL schema, and returns a db connection string.
func setupTestDB(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	cleanup := func() {
		if err := postgresContainer.Terminate(context.Background()); err != nil {
			t.Fatalf("failed to terminate container: %v", err)
		}
	}

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanup()
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		cleanup()
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Apply base DDL mapping from schema_test.go
	ddl := `
CREATE TABLE member_current (
	guild_id text NOT NULL,
	user_id text NOT NULL,
	roles text[],
	updated_at timestamptz
);
CREATE TABLE avatars_current (
	guild_id text NOT NULL,
	user_id text NOT NULL,
	avatar_hash text,
	updated_at timestamptz
);
CREATE TABLE member_joins (
	guild_id text NOT NULL,
	user_id text NOT NULL,
	joined_at timestamptz,
	last_seen_at timestamptz,
	is_bot boolean,
	left_at timestamptz
);
CREATE TABLE messages_history (
	guild_id text NOT NULL,
	message_id text NOT NULL,
	channel_id text NOT NULL,
	author_id text NOT NULL,
	version int NOT NULL,
	event_type text NOT NULL,
	content text,
	attachments int,
	embeds int,
	stickers int,
	created_at timestamptz NOT NULL
);
CREATE TABLE message_version_counters (
	guild_id text NOT NULL,
	message_id text NOT NULL
);
CREATE TABLE guild_meta (
	guild_id text NOT NULL,
	heartbeat timestamptz,
	last_event timestamptz
);
CREATE TABLE runtime_meta (
	guild_id text NOT NULL,
	bot_heartbeat timestamptz,
	bot_last_event timestamptz
);
CREATE TABLE moderation_cases (
	guild_id text NOT NULL,
	case_id int NOT NULL
);
CREATE TABLE moderation_warnings (
	guild_id text NOT NULL,
	warning_id int NOT NULL
);
CREATE TABLE qotd_questions (
	guild_id text NOT NULL,
	question_id int NOT NULL
);
CREATE TABLE qotd_answers (
	guild_id text NOT NULL,
	answer_id int NOT NULL
);
CREATE TABLE system_events (
	guild_id text NOT NULL,
	event_id int NOT NULL
);
CREATE TABLE app_meta (
	key text NOT NULL,
	value text NOT NULL
);
CREATE TABLE daily_member_joins (
	guild_id text NOT NULL,
	date date NOT NULL,
	count int NOT NULL
);
CREATE TABLE daily_member_leaves (
	guild_id text NOT NULL,
	date date NOT NULL,
	count int NOT NULL
);
CREATE TABLE daily_message_metrics (
	guild_id text NOT NULL,
	date date NOT NULL,
	count int NOT NULL
);
`
	if _, err := pool.Exec(ctx, ddl); err != nil {
		cleanup()
		t.Fatalf("failed to apply DDL: %v", err)
	}

	return connStr, cleanup
}

func TestStore_Schema_ParametricDeletion(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("skipping testcontainers-go tests on local windows environment due to rootless docker limitation")
	}
	// REQUIREMENT: Deleção Paramétrica usando testcontainers-go
	ctx := context.Background()
	connStr, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect pool: %v", err)
	}
	defer pool.Close()

	// Simulate parametric deletion
	if _, err := pool.Exec(ctx, "ALTER TABLE avatars_current DROP COLUMN updated_at"); err != nil {
		t.Fatalf("failed to drop column: %v", err)
	}

	store, err := NewStore(pool, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	err = store.Init()
	if err == nil {
		t.Fatal("expected Init() to fail due to missing column, but it succeeded")
	}
	if !strings.Contains(err.Error(), "column avatars_current.updated_at missing") {
		t.Errorf("expected missing column error, got: %v", err)
	}
}

func TestStore_Schema_TypeRegression(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("skipping testcontainers-go tests on local windows environment due to rootless docker limitation")
	}
	// REQUIREMENT: Regressão de Tipo usando testcontainers-go
	ctx := context.Background()
	connStr, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect pool: %v", err)
	}
	defer pool.Close()

	// Simulate type regression
	if _, err := pool.Exec(ctx, "ALTER TABLE avatars_current ALTER COLUMN guild_id TYPE bigint USING guild_id::bigint"); err != nil {
		// Because it's an empty table, the cast works.
	}

	store, err := NewStore(pool, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	err = store.Init()
	if err == nil {
		t.Fatal("expected Init() to fail due to type regression, but it succeeded")
	}
	if !strings.Contains(err.Error(), "type mismatch") {
		t.Errorf("expected type mismatch error, got: %v", err)
	}
}

```

// === FILE: pkg/storage/postgres/schema_unit_test.go ===
```go
package postgres

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestStore_Init_Success(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()
	mock.MatchExpectationsInOrder(false)

	store, err := NewStore(mock, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Mock ensureMemberJoinColumns: columns exist
	for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", col).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	}

	// Mock validateSchema: tables exist
	for _, table := range requiredSchemaTables {
		tblName := table
		mock.ExpectQuery(`SELECT to_regclass`).
			WithArgs(table).
			WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
	}

	// Mock validateSchema columns
	for table, columns := range requiredSchemaColumns {
		for _, col := range columns {
			mock.ExpectQuery(`SELECT data_type`).
				WithArgs(table, col.Name).
				WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow(col.DataType))
		}
	}

	// Mock resetQOTDQuestionSequenceWhenEmpty: has rows
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM qotd_questions`).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	err = store.Init()
	if err != nil {
		t.Errorf("expected Init to succeed, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_Init_MissingColumnsAndEmptyQOTD(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()
	mock.MatchExpectationsInOrder(false)

	store, err := NewStore(mock, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Mock check: columns missing
	for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", col).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	}

	mock.ExpectBegin()
	mock.ExpectExec(`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))
	mock.ExpectExec(`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS is_bot BOOLEAN`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))
	mock.ExpectExec(`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))
	mock.ExpectExec(`UPDATE member_joins`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec(`CREATE INDEX IF NOT EXISTS idx_member_joins_active`).
		WillReturnResult(pgxmock.NewResult("CREATE", 1))
	mock.ExpectCommit()
	mock.ExpectRollback()

	// Validate schema checks: tables exist
	for _, table := range requiredSchemaTables {
		tblName := table
		mock.ExpectQuery(`SELECT to_regclass`).
			WithArgs(table).
			WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
	}

	// Mock validateSchema columns
	for table, columns := range requiredSchemaColumns {
		for _, col := range columns {
			mock.ExpectQuery(`SELECT data_type`).
				WithArgs(table, col.Name).
				WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow(col.DataType))
		}
	}

	// Mock resetQOTDQuestionSequenceWhenEmpty: empty, so setval
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM qotd_questions`).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`SELECT setval`).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))

	err = store.Init()
	if err != nil {
		t.Errorf("expected Init to succeed with migration, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_Init_Failures(t *testing.T) {
	t.Parallel()
	t.Run("check columns error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", "last_seen_at").
			WillReturnError(errors.New("query error"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on column check failure")
		}
		if !strings.Contains(err.Error(), "ensure member join state columns") {
			t.Errorf("expected ensure member join columns error, got: %v", err)
		}
	})

	t.Run("migration alter table error and rollback error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", "last_seen_at").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", "is_bot").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", "left_at").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectBegin()
		mock.ExpectExec(`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`).
			WillReturnError(errors.New("alter table failure"))
		mock.ExpectRollback().WillReturnError(errors.New("rollback failure"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on migration failure")
		}
		if !strings.Contains(err.Error(), "alter table failure") || !strings.Contains(err.Error(), "rollback failure") {
			t.Errorf("expected combined alter table and rollback error, got: %v", err)
		}
	})

	t.Run("validate schema table missing", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// ensureMemberJoinColumns: exist
		for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("member_joins", col).
				WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		}

		// validateSchema: second table is missing
		for i, table := range requiredSchemaTables {
			if i == 1 {
				mock.ExpectQuery(`SELECT to_regclass`).
					WithArgs(table).
					WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(nil))
			} else {
				tblName := table
				mock.ExpectQuery(`SELECT to_regclass`).
					WithArgs(table).
					WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
			}
		}

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on missing table")
		}
		if !strings.Contains(err.Error(), "missing migrated tables") {
			t.Errorf("expected missing migrated tables error, got: %v", err)
		}
	})

	t.Run("validate schema column type mismatch", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()
		mock.MatchExpectationsInOrder(false)

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// ensureMemberJoinColumns: exist
		for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("member_joins", col).
				WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		}

		// validateSchema: tables exist
		for _, table := range requiredSchemaTables {
			tblName := table
			mock.ExpectQuery(`SELECT to_regclass`).
				WithArgs(table).
				WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
		}

		// validateSchema columns: first has error or mismatch
		// let's return a mismatch type
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "last_seen_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("text")) // mismatch

		// other columns
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "is_bot").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("boolean"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "left_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("timestamp with time zone"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("avatars_current", "guild_id").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("text"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("avatars_current", "updated_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("timestamp with time zone"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on type mismatch")
		}
		if !strings.Contains(err.Error(), "type mismatch") {
			t.Errorf("expected type mismatch error, got: %v", err)
		}
	})

	t.Run("validate schema column missing", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()
		mock.MatchExpectationsInOrder(false)

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// ensureMemberJoinColumns: exist
		for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("member_joins", col).
				WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		}

		// validateSchema: tables exist
		for _, table := range requiredSchemaTables {
			tblName := table
			mock.ExpectQuery(`SELECT to_regclass`).
				WithArgs(table).
				WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
		}

		// validateSchema columns: first column is missing (pgx.ErrNoRows)
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "last_seen_at").
			WillReturnError(pgx.ErrNoRows)

		// other columns
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "is_bot").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("boolean"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "left_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("timestamp with time zone"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("avatars_current", "guild_id").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("text"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("avatars_current", "updated_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("timestamp with time zone"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on missing column")
		}
		if !strings.Contains(err.Error(), "missing") {
			t.Errorf("expected missing column error, got: %v", err)
		}
	})

	t.Run("reset qotd query failure", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()
		mock.MatchExpectationsInOrder(false)

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// ensureMemberJoinColumns: exist
		for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("member_joins", col).
				WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		}

		// validateSchema: tables exist
		for _, table := range requiredSchemaTables {
			tblName := table
			mock.ExpectQuery(`SELECT to_regclass`).
				WithArgs(table).
				WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
		}

		// validateSchema columns
		for table, columns := range requiredSchemaColumns {
			for _, col := range columns {
				mock.ExpectQuery(`SELECT data_type`).
					WithArgs(table, col.Name).
					WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow(col.DataType))
			}
		}

		// Mock resetQOTDQuestionSequenceWhenEmpty: query failure
		mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM qotd_questions`).
			WillReturnError(errors.New("qotd check failed"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on QOTD check error")
		}
		if !strings.Contains(err.Error(), "reset qotd question sequence") {
			t.Errorf("expected reset qotd question sequence error, got: %v", err)
		}
	})
}

```

// === FILE: pkg/storage/postgres/storagetest/failing.go ===
```go
// Package storagetest provides test helpers for code that depends on
// *postgres.Store. It is intended for import from _test.go files only.
package storagetest

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
)

// NewFailingStore returns a *postgres.Store backed by a connector that always
// returns an error. Every Store method that touches the database surfaces
// the connector error, exercising the persistence-unavailable branch without
// requiring a real Postgres connection.
func NewFailingStore() *postgres.Store {
	config, _ := pgxpool.ParseConfig("postgres://postgres:password@localhost:5432/postgres")
	config.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, errors.New("storagetest: connector always fails")
	}
	pool, _ := pgxpool.NewWithConfig(context.Background(), config)
	store, _ := postgres.NewStore(pool, nil)
	return store
}

```

// === FILE: pkg/storage/postgres/storagetest/failing_test.go ===
```go
package storagetest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/system"
)

func TestFailingStore(t *testing.T) {
	t.Parallel()
	store := NewFailingStore()
	err := store.UpsertCacheEntriesContext(context.Background(), []system.CacheEntryRecord{
		{
			Key: "test", CacheType: "test", Data: "test", ExpiresAt: time.Now(),
		},
	})
	fmt.Printf("err = %v\n", err)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

```

// === FILE: pkg/storage/postgres/store.go ===
```go
package postgres

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB defines the interface for PostgreSQL connection pooling.
// It is fully compatible with pgxpool.Pool and pgxmock.PgxPoolIface.
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
	Ping(ctx context.Context) error
	Close()
}

// Store wraps a PostgreSQL database interface for durable caching and persistence.
//
// Concurrency: Safe for concurrent use by multiple goroutines.
// Lifecycle: Call Init() after creation before executing queries. Call Close() to release resources.
type Store struct {
	db     DB
	logger *slog.Logger
}

// NewStore creates a new Store using an existing SQL connection interface.
// Returns an error if the provided db is nil, avoiding runtime panics for invariant failures.
func NewStore(db DB, logger *slog.Logger) (*Store, error) {
	if db == nil {
		return nil, errors.New("storage: NewStore requires a non-nil DB interface")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Store{db: db, logger: logger}, nil
}

// log provides safe access to the configured logger.
func (s *Store) log() *slog.Logger {
	return s.logger
}

// Close gracefully releases the underlying database connections.
func (s *Store) Close() error {
	s.db.Close()
	return nil
}

// internal query helpers abstract the receiver (db vs tx)

func txExecContext(ctx context.Context, tx pgx.Tx, query string, args ...any) (pgconn.CommandTag, error) {
	return tx.Exec(ctx, query, args...)
}

func txQueryContext(ctx context.Context, tx pgx.Tx, query string, args ...any) (pgx.Rows, error) {
	return tx.Query(ctx, query, args...)
}

func txQueryRowContext(ctx context.Context, tx pgx.Tx, query string, args ...any) pgx.Row {
	return tx.QueryRow(ctx, query, args...)
}

```

// === FILE: pkg/storage/postgres/store_test.go ===
```go
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/members"
)

func TestStore_TransactionalLifecycle_CommitValidation(t *testing.T) {
	t.Parallel()
	// Validação de Commit e Ignição de Rollback Silencioso
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close()

	store, err := NewStore(mock, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	guildID := "12345"
	snapshots := []members.Snapshot{
		{UserID: "1", JoinedAt: time.Now(), HasBot: true, IsBot: false},
	}
	updatedAt := time.Now()

	mock.ExpectBegin()
	// Mock the single batch query inside UpsertGuildMemberSnapshotsContext (joinRows)
	mock.ExpectExec("INSERT INTO member_joins").
		WithArgs(guildID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectCommit()
	mock.ExpectRollback()

	err = store.UpsertGuildMemberSnapshotsContext(context.Background(), guildID, snapshots, updatedAt)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestStore_TransactionalLifecycle_HybridRollbackFailures(t *testing.T) {
	t.Parallel()
	// Propagação Híbrida de Falhas de Rollback
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close()

	store, err := NewStore(mock, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	guildID := "12345"
	snapshots := []members.Snapshot{
		{UserID: "1", JoinedAt: time.Now(), HasBot: true, IsBot: false},
	}
	updatedAt := time.Now()

	originalErr := errors.New("foreign key constraint violation")
	rollbackErr := errors.New("network interrupt during rollback")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO member_joins").
		WithArgs(guildID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(originalErr)

	mock.ExpectRollback().WillReturnError(rollbackErr)

	err = store.UpsertGuildMemberSnapshotsContext(context.Background(), guildID, snapshots, updatedAt)

	// Assert using errors.As/errors.Is
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, originalErr) {
		t.Errorf("expected error tree to contain original error: %v, got: %v", originalErr, err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Errorf("expected error tree to contain rollback error: %v, got: %v", rollbackErr, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestNewStore_NilDB(t *testing.T) {
	t.Parallel()
	s, err := NewStore(nil, nil)
	if err == nil {
		t.Error("expected error when passing nil DB, got nil")
	}
	if s != nil {
		t.Error("expected store to be nil when DB is nil")
	}
}

```

// === FILE: pkg/storage/postgres/system.go ===
```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/small-frappuccino/discordcore/pkg/system"
)

// SetBotSince sets the bot_since timestamp for a guild.
func (s *Store) SetBotSince(ctx context.Context, guildID string, t time.Time) error {
	if guildID == "" {
		return nil
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO guild_meta (guild_id, bot_since)
         VALUES ($1, $2)
         ON CONFLICT(guild_id) DO UPDATE SET
           bot_since = CASE
             WHEN guild_meta.bot_since IS NULL OR $3 < guild_meta.bot_since THEN $4
             ELSE guild_meta.bot_since
           END`,
		guildID, t, t, t,
	)
	return err
}

// BotSince returns when the bot was first seen in a guild.
func (s *Store) BotSince(ctx context.Context, guildID string) (time.Time, bool, error) {
	row := s.db.QueryRow(ctx, `SELECT bot_since FROM guild_meta WHERE guild_id=$1`, guildID)
	var t sql.NullTime
	if err := row.Scan(&t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("Store.BotSince: %w", err)
	}
	if !t.Valid {
		return time.Time{}, false, nil
	}
	return t.Time, true, nil
}

func (s *Store) setRuntimeTimestamp(ctx context.Context, key string, t time.Time) error {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO runtime_meta (key, ts) VALUES ($1, $2)
         ON CONFLICT(key) DO UPDATE SET ts=excluded.ts`,
		key, t.UTC(),
	)
	return err
}

func (s *Store) getRuntimeTimestamp(ctx context.Context, key string) (time.Time, bool, error) {
	var ts time.Time
	if err := s.db.QueryRow(ctx, `SELECT ts FROM runtime_meta WHERE key=$1`, key).Scan(&ts); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("Store.getRuntimeTimestamp: %w", err)
	}
	return ts, true, nil
}

// SetHeartbeatForBot records the last-known "bot is running" timestamp for a specific instance.
func (s *Store) SetHeartbeatForBot(ctx context.Context, instanceID string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "heartbeat_"+instanceID, t)
}

// SetLastEventForBot records the last event timestamp for a specific instance.
func (s *Store) SetLastEventForBot(ctx context.Context, instanceID string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "last_event_"+instanceID, t)
}

// Heartbeat returns the last recorded heartbeat timestamp.
func (s *Store) Heartbeat(ctx context.Context) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "heartbeat")
}

// NextTicketID atomically increments and returns the next available ticket sequence ID.
func (s *Store) NextTicketID(ctx context.Context, guildID string) (int64, error) {
	var nextID int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO ticket_sequences (guild_id, last_id)
		VALUES ($1, 1)
		ON CONFLICT (guild_id) DO UPDATE
		SET last_id = ticket_sequences.last_id + 1
		RETURNING last_id
	`, guildID).Scan(&nextID)
	if err != nil {
		return 0, fmt.Errorf("Store.NextTicketID: %w", err)
	}
	return nextID, nil
}

// UpsertCacheEntriesContext upserts cache entries.
func (s *Store) UpsertCacheEntriesContext(ctx context.Context, entries []system.CacheEntryRecord) error {
	cachedAt := time.Now().UTC()

	for _, entry := range entries {
		if entry.Key == "" || entry.CacheType == "" || entry.Data == "" {
			continue
		}

		var err error
		if entry.GuildID != "" {
			_, err = s.db.Exec(ctx,
				`INSERT INTO persistent_cache (cache_key, cache_type, guild_id, data, expires_at, cached_at)
				 VALUES ($1, $2, $3, $4, $5, $6)
				 ON CONFLICT(cache_key) DO UPDATE SET
					guild_id=excluded.guild_id,
					data=excluded.data,
					expires_at=excluded.expires_at,
					cached_at=excluded.cached_at`,
				entry.Key, entry.CacheType, entry.GuildID, entry.Data, entry.ExpiresAt, cachedAt,
			)
		} else {
			_, err = s.db.Exec(ctx,
				`INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at)
				 VALUES ($1, $2, $3, $4, $5)
				 ON CONFLICT(cache_key) DO UPDATE SET
					data=excluded.data,
					expires_at=excluded.expires_at,
					cached_at=excluded.cached_at`,
				entry.Key, entry.CacheType, entry.Data, entry.ExpiresAt, cachedAt,
			)
		}

		if err != nil {
			if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "foreign key constraint") {
				continue
			}
			return err
		}
	}
	return nil
}

// GetCacheEntry retrieves a cache entry from persistent storage.
func (s *Store) GetCacheEntry(ctx context.Context, key string) (cacheType, data string, expiresAt time.Time, ok bool, err error) {
	err = s.db.QueryRow(ctx, `SELECT cache_type, data, expires_at FROM persistent_cache WHERE cache_key=$1`, key).Scan(&cacheType, &data, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", time.Time{}, false, nil
		}
		return "", "", time.Time{}, false, fmt.Errorf("Store.GetCacheEntry: %w", err)
	}
	if time.Now().After(expiresAt) {
		return "", "", time.Time{}, false, nil
	}
	return cacheType, data, expiresAt, true, nil
}

// GetCacheEntriesByType streams cache entries via iter.Seq2.
func (s *Store) GetCacheEntriesByType(ctx context.Context, cacheType string) iter.Seq2[system.CacheEntry, error] {
	return func(yield func(system.CacheEntry, error) bool) {
		rows, err := s.db.Query(ctx, `SELECT cache_key, data, expires_at FROM persistent_cache WHERE cache_type=$1 AND expires_at > $2`, cacheType, time.Now().UTC())
		if err != nil {
			yield(system.CacheEntry{}, fmt.Errorf("Store.GetCacheEntriesByType: %w", err))
			return
		}
		defer rows.Close()

		var entry system.CacheEntry
		for rows.Next() {
			entry = system.CacheEntry{}
			if err := rows.Scan(&entry.Key, &entry.Data, &entry.ExpiresAt); err != nil {
				yield(system.CacheEntry{}, fmt.Errorf("Store.GetCacheEntriesByType: %w", err))
				return
			}
			if !yield(entry, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(system.CacheEntry{}, fmt.Errorf("Store.GetCacheEntriesByType: %w", err))
		}
	}
}

// CleanupExpiredCacheEntries removes all expired cache entries.
func (s *Store) CleanupExpiredCacheEntries(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `DELETE FROM persistent_cache WHERE expires_at <= $1`, time.Now().UTC())
	return err
}

// GetCacheStatsContext returns persistent_cache stats.
func (s *Store) GetCacheStatsContext(ctx context.Context) (system.PersistentCacheStats, error) {
	rows, err := s.db.Query(ctx, `SELECT cache_type, COUNT(*) FROM persistent_cache WHERE expires_at > $1 GROUP BY cache_type`, time.Now().UTC())
	if err != nil {
		return system.PersistentCacheStats{}, fmt.Errorf("Store.GetCacheStatsContext: %w", err)
	}
	defer rows.Close()

	stats := system.PersistentCacheStats{ByType: make(map[string]int)}
	for rows.Next() {
		var cacheType string
		var count int
		if err := rows.Scan(&cacheType, &count); err != nil {
			return system.PersistentCacheStats{}, fmt.Errorf("Store.GetCacheStatsContext: %w", err)
		}
		stats.ByType[cacheType] = count
		stats.Total += count
	}
	if err := rows.Err(); err != nil {
		return system.PersistentCacheStats{}, fmt.Errorf("Store.GetCacheStatsContext: %w", err)
	}
	if len(stats.ByType) == 0 {
		stats.ByType = nil
	}
	return stats, nil
}

// PurgeGuildModerationData drops all moderation warnings and resets the case counter.
func (s *Store) PurgeGuildModerationData(ctx context.Context, guildID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	if _, err := tx.Exec(ctx, `DELETE FROM moderation_warnings WHERE guild_id = $1`, guildID); err != nil {
		return fmt.Errorf("delete moderation_warnings: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM moderation_cases WHERE guild_id = $1`, guildID); err != nil {
		return fmt.Errorf("delete moderation_cases: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// IncrementDailyMemberJoinContext atomically increments the daily member join counter.
func (s *Store) IncrementDailyMemberJoinContext(ctx context.Context, guildID, userID string, timestamp time.Time) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO daily_member_joins (guild_id, date, count)
		VALUES ($1, CURRENT_DATE, 1)
		ON CONFLICT(guild_id, date) DO UPDATE SET count = daily_member_joins.count + 1
	`, guildID)
	return err
}

// IncrementDailyMemberLeaveContext atomically increments the daily member leave counter.
func (s *Store) IncrementDailyMemberLeaveContext(ctx context.Context, guildID, userID string, timestamp time.Time) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO daily_member_leaves (guild_id, date, count)
		VALUES ($1, CURRENT_DATE, 1)
		ON CONFLICT(guild_id, date) DO UPDATE SET count = daily_member_leaves.count + 1
	`, guildID)
	return err
}

// HeartbeatForBot returns the last recorded heartbeat timestamp for a specific instance.
func (s *Store) HeartbeatForBot(ctx context.Context, instanceID string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "heartbeat_"+instanceID)
}

// LastEventForBot returns the last event timestamp for a specific instance.
func (s *Store) LastEventForBot(ctx context.Context, instanceID string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "last_event_"+instanceID)
}

// Metadata returns an arbitrary metadata timestamp.
func (s *Store) Metadata(ctx context.Context, key string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "meta_"+key)
}

// SetMetadata sets an arbitrary metadata timestamp.
func (s *Store) SetMetadata(ctx context.Context, key string, at time.Time) error {
	return s.setRuntimeTimestamp(ctx, "meta_"+key, at)
}

```

// === FILE: pkg/storage/postgres/system_test.go ===
```go
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/system"
)

func TestStore_System_NextTicketID(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("INSERT INTO ticket_sequences").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"last_id"}).AddRow(int64(1)))

	store.NextTicketID(context.Background(), "guild1")
}

func TestStore_System_BotSince_ErrNoRows(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT bot_since FROM guild_meta").WithArgs(pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	store.BotSince(context.Background(), "guild1")
}

func TestStore_System_GetCacheEntry_ErrNoRows(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT cache_type, data, expires_at FROM persistent_cache").WithArgs(pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	store.GetCacheEntry(context.Background(), "key1")
}

func TestStore_System_GetCacheEntry_Expired(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	expiredTime := time.Now().Add(-1 * time.Hour)
	mock.ExpectQuery("SELECT cache_type, data, expires_at FROM persistent_cache").WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"cache_type", "data", "expires_at"}).AddRow("type1", "data1", expiredTime))

	store.GetCacheEntry(context.Background(), "key1")
}

func TestStore_System_BotSince(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		mock.ExpectExec(`INSERT INTO guild_meta`).
			WithArgs("g1", now, now, now).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.SetBotSince(context.Background(), "g1", now)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		mock.ExpectQuery(`SELECT bot_since FROM guild_meta`).
			WithArgs("g1").
			WillReturnRows(pgxmock.NewRows([]string{"bot_since"}).AddRow(now))

		ts, ok, err := store.BotSince(context.Background(), "g1")
		if err != nil || !ok || !ts.Equal(now) {
			t.Errorf("unexpected result: ts=%v, ok=%v, err=%v", ts, ok, err)
		}
	})

	t.Run("empty guildID", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.SetBotSince(context.Background(), "", time.Now())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestStore_System_RuntimeMeta(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	now := time.Now()

	// SetHeartbeatForBot
	mock.ExpectExec(`INSERT INTO runtime_meta`).
		WithArgs("heartbeat_inst1", now.UTC()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err := store.SetHeartbeatForBot(context.Background(), "inst1", now)
	if err != nil {
		t.Errorf("SetHeartbeatForBot: %v", err)
	}

	// SetLastEventForBot
	mock.ExpectExec(`INSERT INTO runtime_meta`).
		WithArgs("last_event_inst1", now.UTC()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err = store.SetLastEventForBot(context.Background(), "inst1", now)
	if err != nil {
		t.Errorf("SetLastEventForBot: %v", err)
	}

	// SetMetadata
	mock.ExpectExec(`INSERT INTO runtime_meta`).
		WithArgs("meta_key1", now.UTC()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err = store.SetMetadata(context.Background(), "key1", now)
	if err != nil {
		t.Errorf("SetMetadata: %v", err)
	}

	// Heartbeat (getRuntimeTimestamp "heartbeat")
	mock.ExpectQuery(`SELECT ts FROM runtime_meta`).
		WithArgs("heartbeat").
		WillReturnRows(pgxmock.NewRows([]string{"ts"}).AddRow(now))
	ts, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok || !ts.Equal(now) {
		t.Errorf("Heartbeat: ts=%v, ok=%v, err=%v", ts, ok, err)
	}

	// HeartbeatForBot
	mock.ExpectQuery(`SELECT ts FROM runtime_meta`).
		WithArgs("heartbeat_inst1").
		WillReturnRows(pgxmock.NewRows([]string{"ts"}).AddRow(now))
	ts, ok, err = store.HeartbeatForBot(context.Background(), "inst1")
	if err != nil || !ok || !ts.Equal(now) {
		t.Errorf("HeartbeatForBot: ts=%v, ok=%v, err=%v", ts, ok, err)
	}

	// LastEventForBot
	mock.ExpectQuery(`SELECT ts FROM runtime_meta`).
		WithArgs("last_event_inst1").
		WillReturnRows(pgxmock.NewRows([]string{"ts"}).AddRow(now))
	ts, ok, err = store.LastEventForBot(context.Background(), "inst1")
	if err != nil || !ok || !ts.Equal(now) {
		t.Errorf("LastEventForBot: ts=%v, ok=%v, err=%v", ts, ok, err)
	}

	// Metadata
	mock.ExpectQuery(`SELECT ts FROM runtime_meta`).
		WithArgs("meta_key1").
		WillReturnRows(pgxmock.NewRows([]string{"ts"}).AddRow(now))
	ts, ok, err = store.Metadata(context.Background(), "key1")
	if err != nil || !ok || !ts.Equal(now) {
		t.Errorf("Metadata: ts=%v, ok=%v, err=%v", ts, ok, err)
	}
}

func TestStore_System_UpsertCacheEntriesContext(t *testing.T) {
	t.Parallel()
	t.Run("empty entries", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.UpsertCacheEntriesContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("entries with and without guild_id", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		entries := []system.CacheEntryRecord{
			{Key: "k1", CacheType: "t1", GuildID: "g1", Data: "d1", ExpiresAt: now},
			{Key: "k2", CacheType: "t2", GuildID: "", Data: "d2", ExpiresAt: now},
			{Key: "", CacheType: "t3"}, // invalid, should be filtered
		}

		mock.ExpectExec(`INSERT INTO persistent_cache`).
			WithArgs("k1", "t1", "g1", "d1", now, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		mock.ExpectExec(`INSERT INTO persistent_cache`).
			WithArgs("k2", "t2", "d2", now, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.UpsertCacheEntriesContext(context.Background(), entries)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("foreign key error should be ignored/mitigated", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		entries := []system.CacheEntryRecord{
			{Key: "k1", CacheType: "t1", GuildID: "g1", Data: "d1"},
		}

		mock.ExpectExec(`INSERT INTO persistent_cache`).
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnError(errors.New("23503: foreign key constraint violation"))

		err := store.UpsertCacheEntriesContext(context.Background(), entries)
		if err != nil {
			t.Errorf("expected foreign key constraint violation to be ignored, got: %v", err)
		}
	})
}

func TestStore_System_GetCacheEntriesByType(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		rows := pgxmock.NewRows([]string{"cache_key", "data", "expires_at"}).
			AddRow("k1", "d1", now).
			AddRow("k2", "d2", now)

		mock.ExpectQuery(`SELECT cache_key, data, expires_at FROM persistent_cache`).
			WithArgs("t1", pgxmock.AnyArg()).
			WillReturnRows(rows)

		seq := store.GetCacheEntriesByType(context.Background(), "t1")
		var results []system.CacheEntry
		var err error
		for entry, e := range seq {
			if e != nil {
				err = e
				break
			}
			results = append(results, entry)
		}

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 || results[0].Key != "k1" || results[1].Key != "k2" {
			t.Errorf("unexpected results: %+v", results)
		}
	})

	t.Run("query error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT cache_key, data, expires_at FROM persistent_cache`).
			WillReturnError(errors.New("query error"))

		seq := store.GetCacheEntriesByType(context.Background(), "t1")
		var results []system.CacheEntry
		var err error
		for entry, e := range seq {
			if e != nil {
				err = e
				break
			}
			results = append(results, entry)
		}

		if err == nil {
			t.Error("expected error, got nil")
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestStore_System_CleanupExpiredCacheEntries(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectExec(`DELETE FROM persistent_cache WHERE expires_at <=`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("DELETE", 3))

	err := store.CleanupExpiredCacheEntries(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStore_System_GetCacheStatsContext(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		rows := pgxmock.NewRows([]string{"cache_type", "count"}).
			AddRow("t1", 5).
			AddRow("t2", 10)

		mock.ExpectQuery(`SELECT cache_type, COUNT\(\*\)`).
			WithArgs(pgxmock.AnyArg()).
			WillReturnRows(rows)

		stats, err := store.GetCacheStatsContext(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.Total != 15 || stats.ByType["t1"] != 5 || stats.ByType["t2"] != 10 {
			t.Errorf("unexpected stats: %+v", stats)
		}
	})

	t.Run("query error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT cache_type, COUNT\(\*\)`).
			WillReturnError(errors.New("db error"))

		_, err := store.GetCacheStatsContext(context.Background())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_System_PurgeGuildModerationData(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectExec(`DELETE FROM moderation_warnings WHERE guild_id =`).
			WithArgs("g1").
			WillReturnResult(pgxmock.NewResult("DELETE", 2))
		mock.ExpectExec(`DELETE FROM moderation_cases WHERE guild_id =`).
			WithArgs("g1").
			WillReturnResult(pgxmock.NewResult("DELETE", 4))
		mock.ExpectCommit()
		mock.ExpectRollback()

		err := store.PurgeGuildModerationData(context.Background(), "g1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("exec error and rollback", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectExec(`DELETE FROM moderation_warnings WHERE guild_id =`).
			WillReturnError(errors.New("db error"))
		mock.ExpectRollback()

		err := store.PurgeGuildModerationData(context.Background(), "g1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_System_IncrementDailyMemberEvents(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	// IncrementDailyMemberJoinContext
	mock.ExpectExec(`INSERT INTO daily_member_joins`).
		WithArgs("g1").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.IncrementDailyMemberJoinContext(context.Background(), "g1", "u1", time.Now())
	if err != nil {
		t.Errorf("IncrementDailyMemberJoinContext: %v", err)
	}

	// IncrementDailyMemberLeaveContext
	mock.ExpectExec(`INSERT INTO daily_member_leaves`).
		WithArgs("g1").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = store.IncrementDailyMemberLeaveContext(context.Background(), "g1", "u1", time.Now())
	if err != nil {
		t.Errorf("IncrementDailyMemberLeaveContext: %v", err)
	}
}

```

