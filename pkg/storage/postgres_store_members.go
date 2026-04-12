package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// UpsertGuildMemberSnapshotsContext persists one page of guild member snapshots in a single transaction.
func (s *Store) UpsertGuildMemberSnapshotsContext(ctx context.Context, guildID string, snapshots []GuildMemberSnapshot, updatedAt time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || len(snapshots) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertGuildMemberSnapshotBatch(ctx, tx, guildID, normalized, updatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func upsertGuildMemberSnapshotBatch(ctx context.Context, tx *sql.Tx, guildID string, snapshots []GuildMemberSnapshot, updatedAt time.Time) error {
	avatarRows := make([]GuildMemberSnapshot, 0, len(snapshots))
	roleRows := make([]GuildMemberSnapshot, 0, len(snapshots))
	joinRows := make([]GuildMemberSnapshot, 0, len(snapshots))
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
		if err := insertAvatarHistoryBatch(ctx, tx, guildID, avatarRows, currentHashes, updatedAt); err != nil {
			return fmt.Errorf("insert avatar history batch: %w", err)
		}
		if err := upsertAvatarCurrentBatch(ctx, tx, guildID, avatarRows, updatedAt); err != nil {
			return fmt.Errorf("upsert avatar current batch: %w", err)
		}
	}

	if len(roleRows) > 0 {
		if err := deleteRolesForUsersBatch(ctx, tx, guildID, roleUserIDs); err != nil {
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

func normalizeGuildMemberSnapshots(snapshots []GuildMemberSnapshot) []GuildMemberSnapshot {
	if len(snapshots) == 0 {
		return nil
	}

	order := make([]string, 0, len(snapshots))
	byUser := make(map[string]*GuildMemberSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		userID := strings.TrimSpace(snapshot.UserID)
		if userID == "" {
			continue
		}
		existing, ok := byUser[userID]
		if !ok {
			existing = &GuildMemberSnapshot{UserID: userID}
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

	normalized := make([]GuildMemberSnapshot, 0, len(order))
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

func queryCurrentAvatarHashesByUserID(ctx context.Context, tx *sql.Tx, guildID string, userIDs []string) (map[string]string, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var b strings.Builder
	b.WriteString(`SELECT user_id, avatar_hash FROM avatars_current WHERE guild_id=? AND user_id IN (`)
	args := make([]any, 0, len(userIDs)+1)
	args = append(args, guildID)
	for i, userID := range userIDs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('?')
		args = append(args, userID)
	}
	b.WriteByte(')')

	rows, err := txQueryContext(ctx, tx, b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hashes := make(map[string]string, len(userIDs))
	for rows.Next() {
		var userID, avatarHash string
		if err := rows.Scan(&userID, &avatarHash); err != nil {
			return nil, err
		}
		hashes[userID] = avatarHash
	}
	return hashes, rows.Err()
}

func insertAvatarHistoryBatch(ctx context.Context, tx *sql.Tx, guildID string, snapshots []GuildMemberSnapshot, currentHashes map[string]string, updatedAt time.Time) error {
	type avatarHistoryRow struct {
		UserID    string
		OldHash   string
		NewHash   string
		ChangedAt time.Time
	}
	rows := make([]avatarHistoryRow, 0, len(snapshots))
	for _, snapshot := range snapshots {
		oldHash, ok := currentHashes[snapshot.UserID]
		if !ok || oldHash == snapshot.AvatarHash {
			continue
		}
		rows = append(rows, avatarHistoryRow{
			UserID:    snapshot.UserID,
			OldHash:   oldHash,
			NewHash:   snapshot.AvatarHash,
			ChangedAt: updatedAt,
		})
	}
	return execValuesInChunks(ctx, tx,
		`INSERT INTO avatars_history (guild_id, user_id, old_hash, new_hash, changed_at) VALUES `,
		``,
		len(rows),
		5,
		func(args []any, rowIndex int) []any {
			row := rows[rowIndex]
			return append(args, guildID, row.UserID, row.OldHash, row.NewHash, row.ChangedAt)
		},
	)
}

func upsertAvatarCurrentBatch(ctx context.Context, tx *sql.Tx, guildID string, snapshots []GuildMemberSnapshot, updatedAt time.Time) error {
	return execValuesInChunks(ctx, tx,
		`INSERT INTO avatars_current (guild_id, user_id, avatar_hash, updated_at) VALUES `,
		` ON CONFLICT(guild_id, user_id) DO UPDATE SET avatar_hash=excluded.avatar_hash, updated_at=excluded.updated_at`,
		len(snapshots),
		4,
		func(args []any, rowIndex int) []any {
			row := snapshots[rowIndex]
			return append(args, guildID, row.UserID, row.AvatarHash, updatedAt)
		},
	)
}

func deleteRolesForUsersBatch(ctx context.Context, tx *sql.Tx, guildID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(`DELETE FROM roles_current WHERE guild_id=? AND user_id IN (`)
	args := make([]any, 0, len(userIDs)+1)
	args = append(args, guildID)
	for i, userID := range userIDs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('?')
		args = append(args, userID)
	}
	b.WriteByte(')')
	_, err := txExecContext(ctx, tx, b.String(), args...)
	return err
}

func insertMemberRolesBatch(ctx context.Context, tx *sql.Tx, guildID string, snapshots []GuildMemberSnapshot, updatedAt time.Time) error {
	type roleRow struct {
		UserID    string
		RoleID    string
		UpdatedAt time.Time
	}
	rows := make([]roleRow, 0, len(snapshots))
	for _, snapshot := range snapshots {
		for _, roleID := range snapshot.Roles {
			rows = append(rows, roleRow{
				UserID:    snapshot.UserID,
				RoleID:    roleID,
				UpdatedAt: updatedAt,
			})
		}
	}
	return execValuesInChunks(ctx, tx,
		`INSERT INTO roles_current (guild_id, user_id, role_id, updated_at) VALUES `,
		``,
		len(rows),
		4,
		func(args []any, rowIndex int) []any {
			row := rows[rowIndex]
			return append(args, guildID, row.UserID, row.RoleID, row.UpdatedAt)
		},
	)
}

func upsertMemberJoinsBatch(ctx context.Context, tx *sql.Tx, guildID string, snapshots []GuildMemberSnapshot, seenAt time.Time) error {
	return execValuesInChunks(ctx, tx,
		`INSERT INTO member_joins (guild_id, user_id, joined_at, last_seen_at, is_bot, left_at) VALUES `,
		` ON CONFLICT(guild_id, user_id) DO UPDATE SET
           joined_at = CASE
             WHEN excluded.joined_at < member_joins.joined_at THEN excluded.joined_at
             ELSE member_joins.joined_at
           END,
           last_seen_at = CASE
             WHEN member_joins.last_seen_at IS NULL OR excluded.last_seen_at > member_joins.last_seen_at THEN excluded.last_seen_at
             ELSE member_joins.last_seen_at
           END,
           is_bot = COALESCE(excluded.is_bot, member_joins.is_bot),
           left_at = NULL`,
		len(snapshots),
		6,
		func(args []any, rowIndex int) []any {
			row := snapshots[rowIndex]
			joinedAt := seenAt
			if !row.JoinedAt.IsZero() {
				joinedAt = row.JoinedAt.UTC()
			}
			var isBot any
			if row.HasBot {
				isBot = row.IsBot
			}
			return append(args, guildID, row.UserID, joinedAt, seenAt, isBot, nil)
		},
	)
}

// UpsertMemberJoin records the earliest known join time for a member in a guild.
func (s *Store) UpsertMemberJoin(guildID, userID string, joinedAt time.Time) error {
	return s.UpsertMemberJoinContext(context.Background(), guildID, userID, joinedAt)
}

// UpsertMemberJoinContext records the earliest known join time for a member in a guild with context support.
func (s *Store) UpsertMemberJoinContext(ctx context.Context, guildID, userID string, joinedAt time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || userID == "" || joinedAt.IsZero() {
		return nil
	}
	joinedAt = joinedAt.UTC()
	seenAt := time.Now().UTC()
	_, err := s.execContext(
		ctx,
		`INSERT INTO member_joins (guild_id, user_id, joined_at, last_seen_at, left_at)
         VALUES (?, ?, ?, ?, NULL)
         ON CONFLICT(guild_id, user_id) DO UPDATE SET
           joined_at = CASE
             WHEN excluded.joined_at < member_joins.joined_at THEN excluded.joined_at
             ELSE member_joins.joined_at
           END,
           last_seen_at = CASE
             WHEN member_joins.last_seen_at IS NULL OR excluded.last_seen_at > member_joins.last_seen_at THEN excluded.last_seen_at
             ELSE member_joins.last_seen_at
           END,
           left_at = NULL`,
		guildID, userID, joinedAt, seenAt,
	)
	return err
}

// UpsertMemberPresenceContext records that a member is currently present in a guild.
// When joinedAt is unknown, seenAt is used as a fallback seed for the row.
func (s *Store) UpsertMemberPresenceContext(ctx context.Context, guildID, userID string, joinedAt, seenAt time.Time, isBot bool) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if seenAt.IsZero() {
		seenAt = time.Now().UTC()
	} else {
		seenAt = seenAt.UTC()
	}
	if joinedAt.IsZero() {
		joinedAt = seenAt
	} else {
		joinedAt = joinedAt.UTC()
	}

	_, err := s.execContext(
		ctx,
		`INSERT INTO member_joins (guild_id, user_id, joined_at, last_seen_at, is_bot, left_at)
         VALUES (?, ?, ?, ?, ?, NULL)
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
           left_at = NULL`,
		guildID, userID, joinedAt, seenAt, isBot,
	)
	return err
}

// MarkMemberLeftContext records that a member is no longer active in a guild and clears current roles.
func (s *Store) MarkMemberLeftContext(ctx context.Context, guildID, userID string, leftAt time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if leftAt.IsZero() {
		leftAt = time.Now().UTC()
	} else {
		leftAt = leftAt.UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := txExecContext(
		ctx,
		tx,
		`UPDATE member_joins
		    SET left_at = CASE
		          WHEN left_at IS NULL OR ? > left_at THEN ?
		          ELSE left_at
		        END,
		        last_seen_at = CASE
		          WHEN last_seen_at IS NULL OR ? > last_seen_at THEN ?
		          ELSE last_seen_at
		        END
		  WHERE guild_id=? AND user_id=?`,
		leftAt, leftAt, leftAt, leftAt, guildID, userID,
	); err != nil {
		return err
	}
	if _, err := txExecContext(ctx, tx, `DELETE FROM roles_current WHERE guild_id=? AND user_id=?`, guildID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// GetMemberJoin returns the stored join time for a member, if any.
func (s *Store) GetMemberJoin(guildID, userID string) (time.Time, bool, error) {
	return s.GetMemberJoinContext(context.Background(), guildID, userID)
}

// GetMemberJoinContext returns the stored join time for a member, if any, with context support.
func (s *Store) GetMemberJoinContext(ctx context.Context, guildID, userID string) (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.queryRowContext(ctx, `SELECT joined_at FROM member_joins WHERE guild_id=? AND user_id=?`, guildID, userID)
	var jt time.Time
	if err := row.Scan(&jt); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return jt, true, nil
}

// GetActiveGuildMemberStatesContext returns the persisted current member state for all active members in a guild.
func (s *Store) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) ([]GuildMemberCurrentState, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rows, err := s.queryContext(ctx, `
		SELECT mj.user_id, mj.joined_at, mj.last_seen_at, mj.is_bot, rc.role_id
		  FROM member_joins mj
		  LEFT JOIN roles_current rc
		    ON rc.guild_id = mj.guild_id
		   AND rc.user_id = mj.user_id
		 WHERE mj.guild_id = ?
		   AND mj.left_at IS NULL
		 ORDER BY mj.user_id, rc.role_id
	`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]GuildMemberCurrentState, 0)
	indexByUser := make(map[string]int)
	for rows.Next() {
		var (
			userID     string
			joinedAt   time.Time
			lastSeenAt sql.NullTime
			isBot      sql.NullBool
			roleID     sql.NullString
		)
		if err := rows.Scan(&userID, &joinedAt, &lastSeenAt, &isBot, &roleID); err != nil {
			return nil, err
		}
		idx, ok := indexByUser[userID]
		if !ok {
			state := GuildMemberCurrentState{
				UserID:   userID,
				JoinedAt: joinedAt.UTC(),
				Active:   true,
			}
			if lastSeenAt.Valid {
				state.LastSeenAt = lastSeenAt.Time.UTC()
			}
			if isBot.Valid {
				state.HasBot = true
				state.IsBot = isBot.Bool
			}
			states = append(states, state)
			idx = len(states) - 1
			indexByUser[userID] = idx
		}
		if roleID.Valid && strings.TrimSpace(roleID.String) != "" {
			states[idx].Roles = append(states[idx].Roles, roleID.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

// UpsertAvatar sets the current avatar hash for a member in a guild.
// If the hash changed, it records a row in avatars_history.
// Returns (changed, oldHash, err).
func (s *Store) UpsertAvatar(guildID, userID, newHash string, updatedAt time.Time) (bool, string, error) {
	if s.db == nil {
		return false, "", fmt.Errorf("store not initialized")
	}
	if guildID == "" || userID == "" {
		return false, "", nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return false, "", err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var curHash string
	var hasCur bool
	if err := txQueryRow(tx,
		`SELECT avatar_hash FROM avatars_current WHERE guild_id=? AND user_id=?`,
		guildID, userID,
	).Scan(&curHash); err != nil {
		if err != sql.ErrNoRows {
			return false, "", err
		}
	} else {
		hasCur = true
	}

	changed := !hasCur || curHash != newHash
	if changed && hasCur {
		if _, err := txExec(tx,
			`INSERT INTO avatars_history (guild_id, user_id, old_hash, new_hash, changed_at)
             VALUES (?, ?, ?, ?, ?)`,
			guildID, userID, curHash, newHash, updatedAt,
		); err != nil {
			return false, "", err
		}
	}

	if _, err := txExec(tx,
		`INSERT INTO avatars_current (guild_id, user_id, avatar_hash, updated_at)
         VALUES (?, ?, ?, ?)
         ON CONFLICT(guild_id, user_id) DO UPDATE SET
           avatar_hash=excluded.avatar_hash,
           updated_at=excluded.updated_at`,
		guildID, userID, newHash, updatedAt,
	); err != nil {
		return false, "", err
	}

	if err := tx.Commit(); err != nil {
		return false, "", err
	}
	return changed, curHash, nil
}

// GetAvatar returns the current avatar hash for a user in a guild, if any.
func (s *Store) GetAvatar(guildID, userID string) (hash string, updatedAt time.Time, ok bool, err error) {
	if s.db == nil {
		return "", time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.queryRow(
		`SELECT avatar_hash, updated_at FROM avatars_current WHERE guild_id=? AND user_id=?`,
		guildID, userID,
	)
	var h string
	var t time.Time
	if scanErr := row.Scan(&h, &t); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return "", time.Time{}, false, nil
		}
		return "", time.Time{}, false, scanErr
	}
	return h, t, true, nil
}

// GetAllMemberJoins retrieves all member join records for a guild
func (s *Store) GetAllMemberJoins(guildID string) (map[string]time.Time, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.query(`SELECT user_id, joined_at FROM member_joins WHERE guild_id=?`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make(map[string]time.Time)
	for rows.Next() {
		var userID string
		var joinedAt time.Time
		if err := rows.Scan(&userID, &joinedAt); err != nil {
			return nil, err
		}
		members[userID] = joinedAt
	}
	return members, rows.Err()
}

// GetAllGuildMemberRoles retrieves all member roles for a guild
func (s *Store) GetAllGuildMemberRoles(guildID string) (map[string][]string, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.query(`SELECT user_id, role_id FROM roles_current WHERE guild_id=?`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memberRoles := make(map[string][]string)
	for rows.Next() {
		var userID, roleID string
		if err := rows.Scan(&userID, &roleID); err != nil {
			return nil, err
		}
		memberRoles[userID] = append(memberRoles[userID], roleID)
	}
	return memberRoles, rows.Err()
}

// TouchMemberJoin refreshes member presence freshness without mutating joined_at.
func (s *Store) TouchMemberJoin(guildID, userID string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || userID == "" {
		return nil
	}
	seenAt := time.Now().UTC()
	_, err := s.exec(
		`UPDATE member_joins
         SET last_seen_at = CASE
           WHEN last_seen_at IS NULL OR ? > last_seen_at THEN ?
           ELSE last_seen_at
         END,
             left_at = NULL
         WHERE guild_id=? AND user_id=?`,
		seenAt, seenAt, guildID, userID,
	)
	return err
}

// TouchMemberRoles updates the updated_at timestamp for all member roles (keeps them fresh)
func (s *Store) TouchMemberRoles(guildID, userID string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || userID == "" {
		return nil
	}
	_, err := s.exec(
		`UPDATE roles_current SET updated_at=? WHERE guild_id=? AND user_id=?`,
		time.Now().UTC(), guildID, userID,
	)
	return err
}
