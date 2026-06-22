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

		for rows.Next() {
			var (
				userID     string
				joinedAt   time.Time
				lastSeenAt *time.Time
				isBot      *bool
				roleID     *string
			)
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

		for rows.Next() {
			var userID, roleID string
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
