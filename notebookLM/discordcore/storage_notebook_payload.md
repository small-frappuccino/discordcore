# Domain Architecture: storage

## Layout Topology
```text
storage/
└── postgres
    ├── storagetest
    │   └── failing.go
    ├── members.go
    ├── messages.go
    ├── moderation.go
    ├── qotd.go
    ├── schema.go
    ├── store.go
    └── system.go
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

