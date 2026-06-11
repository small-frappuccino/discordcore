package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"
)

// GuildMemberSnapshot represents the persisted snapshot for one guild member.
// Fields are opt-in so callers can batch only the parts they need to refresh.
type GuildMemberSnapshot struct {
	UserID     string
	AvatarHash string
	HasAvatar  bool
	Roles      []string
	HasRoles   bool
	JoinedAt   time.Time
	IsBot      bool
	HasBot     bool
}

// GuildMemberCurrentState is the persisted current membership state for a user
// in a guild, including join/leave timestamps and the last known role set.
// LeftAt is the zero time while Active is true.
type GuildMemberCurrentState struct {
	UserID     string
	JoinedAt   time.Time
	LastSeenAt time.Time
	LeftAt     time.Time
	Active     bool
	IsBot      bool
	HasBot     bool
	Roles      []string
}

// UpsertGuildMemberSnapshotsContext persists one page of guild member snapshots in a single transaction.
func (s *Store) UpsertGuildMemberSnapshotsContext(ctx context.Context, guildID string, snapshots []GuildMemberSnapshot, updatedAt time.Time) (err error) {
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Store.UpsertGuildMemberSnapshotsContext: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(); rerr != nil && !errors.Is(rerr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	if err := upsertGuildMemberSnapshotBatch(ctx, tx, guildID, normalized, updatedAt); err != nil {
		return fmt.Errorf("Store.UpsertGuildMemberSnapshotsContext: %w", err)
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
	Snapshots     []GuildMemberSnapshot
	CurrentHashes map[string]string
	UpdatedAt     time.Time
}

func insertAvatarHistoryBatch(ctx context.Context, tx *sql.Tx, batch avatarHistoryBatch) error {
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
	guildIDs := make([]string, len(rows))
	userIDs := make([]string, len(rows))
	oldHashes := make([]string, len(rows))
	newHashes := make([]string, len(rows))
	changedAts := make([]time.Time, len(rows))

	for i, row := range rows {
		guildIDs[i] = batch.GuildID
		userIDs[i] = row.UserID
		oldHashes[i] = row.OldHash
		newHashes[i] = row.NewHash
		changedAts[i] = row.ChangedAt
	}

	if len(rows) > 0 {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO avatars_history (guild_id, user_id, old_hash, new_hash, changed_at)
             SELECT * FROM UNNEST($1::text[], $2::text[], $3::text[], $4::text[], $5::timestamptz[])`,
			guildIDs, userIDs, oldHashes, newHashes, changedAts,
		)
		return err
	}
	return nil
}

func upsertAvatarCurrentBatch(ctx context.Context, tx *sql.Tx, guildID string, snapshots []GuildMemberSnapshot, updatedAt time.Time) error {
	userIDs := make([]string, len(snapshots))
	avatarHashes := make([]string, len(snapshots))
	updatedAts := make([]time.Time, len(snapshots))

	for i, row := range snapshots {
		userIDs[i] = row.UserID
		avatarHashes[i] = row.AvatarHash
		updatedAts[i] = updatedAt
	}

	if len(snapshots) > 0 {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO avatars_current (guild_id, user_id, avatar_hash, updated_at)
             SELECT $1::text, * FROM UNNEST($2::text[], $3::text[], $4::timestamptz[])
             ON CONFLICT(guild_id, user_id) DO UPDATE SET avatar_hash=excluded.avatar_hash, updated_at=excluded.updated_at
             WHERE avatars_current.updated_at < excluded.updated_at`,
			guildID, userIDs, avatarHashes, updatedAts,
		)
		return err
	}
	return nil
}

func deleteRolesForUsersBatch(ctx context.Context, tx *sql.Tx, guildID string, userIDs []string, updatedAt time.Time) error {
	if len(userIDs) == 0 {
		return nil
	}

	_, err := tx.ExecContext(ctx,
		`UPDATE roles_current SET deleted_at = $3, updated_at = $3 WHERE guild_id=$1 AND user_id = ANY($2::text[]) AND updated_at < $3`,
		guildID, userIDs, updatedAt,
	)
	return err
}

func insertMemberRolesBatch(ctx context.Context, tx *sql.Tx, guildID string, snapshots []GuildMemberSnapshot, updatedAt time.Time) error {
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

	_, err := tx.ExecContext(ctx,
		`INSERT INTO roles_current (guild_id, user_id, role_id, updated_at)
         SELECT $1::text, * FROM UNNEST($2::text[], $3::text[], $4::timestamptz[])
         ON CONFLICT(guild_id, user_id, role_id) DO UPDATE SET updated_at=excluded.updated_at, deleted_at=NULL
         WHERE roles_current.updated_at < excluded.updated_at`,
		guildID, userIDs, roleIDs, updatedAts,
	)
	return err
}

func upsertMemberJoinsBatch(ctx context.Context, tx *sql.Tx, guildID string, snapshots []GuildMemberSnapshot, seenAt time.Time) error {
	userIDs := make([]string, len(snapshots))
	joinedAts := make([]time.Time, len(snapshots))
	seenAts := make([]time.Time, len(snapshots))
	isBots := make([]sql.NullBool, len(snapshots))

	for i, row := range snapshots {
		userIDs[i] = row.UserID

		joinedAt := seenAt
		if !row.JoinedAt.IsZero() {
			joinedAt = row.JoinedAt.UTC()
		}
		joinedAts[i] = joinedAt
		seenAts[i] = seenAt

		var isBot sql.NullBool
		if row.HasBot {
			isBot = sql.NullBool{Valid: true, Bool: row.IsBot}
		}
		isBots[i] = isBot
	}

	if len(snapshots) > 0 {
		_, err := tx.ExecContext(ctx,
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
               left_at = NULL`,
			guildID, userIDs, joinedAts, seenAts, isBots,
		)
		return err
	}
	return nil
}

// UpsertMemberJoin records the earliest known join time for a member in a guild.
func (s *Store) UpsertMemberJoin(guildID, userID string, joinedAt time.Time) error {
	return s.UpsertMemberJoinContext(context.Background(), guildID, userID, joinedAt)
}

// UpsertMemberJoinContext records the earliest known join time for a member in a guild with context support.
func (s *Store) UpsertMemberJoinContext(ctx context.Context, guildID, userID string, joinedAt time.Time) error {
	if guildID == "" || userID == "" || joinedAt.IsZero() {
		return nil
	}
	joinedAt = joinedAt.UTC()
	seenAt := time.Now().UTC()
	_, err := s.execContext(
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
           left_at = NULL`,
		guildID, userID, joinedAt, seenAt,
	)
	return err
}

// MemberPresenceInput describes a member presence upsert. When JoinedAt is
// zero it is seeded from SeenAt; a zero SeenAt defaults to now (UTC).
type MemberPresenceInput struct {
	GuildID  string
	UserID   string
	JoinedAt time.Time
	SeenAt   time.Time
	IsBot    bool
}

// UpsertMemberPresenceContext records that a member is currently present in a guild.
// When JoinedAt is unknown, SeenAt is used as a fallback seed for the row.
func (s *Store) UpsertMemberPresenceContext(ctx context.Context, input MemberPresenceInput) error {
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

	_, err := s.execContext(
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
           left_at = NULL`,
		input.GuildID, input.UserID, input.JoinedAt, input.SeenAt, input.IsBot,
	)
	return err
}

// MarkMemberLeftContext records that a member is no longer active in a guild and clears current roles.
func (s *Store) MarkMemberLeftContext(ctx context.Context, guildID, userID string, leftAt time.Time) (err error) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return nil
	}
	if leftAt.IsZero() {
		leftAt = time.Now().UTC()
	} else {
		leftAt = leftAt.UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Store.MarkMemberLeftContext: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(); rerr != nil && !errors.Is(rerr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	if _, err := txExecContext(
		ctx,
		tx,
		`UPDATE member_joins
		    SET left_at = CASE
		          WHEN left_at IS NULL OR $1 > left_at THEN $2
		          ELSE left_at
		        END,
		        last_seen_at = CASE
		          WHEN last_seen_at IS NULL OR $3 > last_seen_at THEN $4
		          ELSE last_seen_at
		        END
		  WHERE guild_id=$5 AND user_id=$6`,
		leftAt, leftAt, leftAt, leftAt, guildID, userID,
	); err != nil {
		return err
	}
	if _, err := txExecContext(ctx, tx, `UPDATE roles_current SET deleted_at = $3, updated_at = $3 WHERE guild_id=$1 AND user_id=$2 AND updated_at < $3`, guildID, userID, leftAt); err != nil {
		return fmt.Errorf("Store.MarkMemberLeftContext: %w", err)
	}
	return tx.Commit()
}

// MemberJoin returns the stored join time for a member, if any.
func (s *Store) MemberJoin(ctx context.Context, guildID, userID string) (time.Time, bool, error) {
	row := s.queryRowContext(ctx, `SELECT joined_at FROM member_joins WHERE guild_id=$1 AND user_id=$2`, guildID, userID)
	var jt time.Time
	if err := row.Scan(&jt); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("Store.MemberJoin: %w", err)
	}
	return jt, true, nil
}

// GetActiveGuildMemberStatesContext returns the persisted current member state for all active members in a guild.
func (s *Store) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[GuildMemberCurrentState, error] {
	return func(yield func(GuildMemberCurrentState, error) bool) {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return
		}

		rows, err := s.queryContext(ctx, `
			SELECT mj.user_id, mj.joined_at, mj.last_seen_at, mj.is_bot, rc.role_id
			  FROM member_joins mj
			  LEFT JOIN roles_current rc
			    ON rc.guild_id = mj.guild_id
			   AND rc.user_id = mj.user_id
			   AND rc.deleted_at IS NULL
			 WHERE mj.guild_id = $1
			   AND mj.left_at IS NULL
			 ORDER BY mj.user_id, rc.role_id
		`, guildID)
		if err != nil {
			yield(GuildMemberCurrentState{}, fmt.Errorf("Store.GetActiveGuildMemberStatesContext: %w", err))
			return
		}
		defer rows.Close()

		var currentState *GuildMemberCurrentState

		for rows.Next() {
			var (
				userID     string
				joinedAt   time.Time
				lastSeenAt sql.NullTime
				isBot      sql.NullBool
				roleID     sql.NullString
			)
			if err := rows.Scan(&userID, &joinedAt, &lastSeenAt, &isBot, &roleID); err != nil {
				yield(GuildMemberCurrentState{}, fmt.Errorf("Store.GetActiveGuildMemberStatesContext: %w", err))
				return
			}

			if currentState != nil && currentState.UserID != userID {
				if !yield(*currentState, nil) {
					return
				}
				currentState = nil
			}

			if currentState == nil {
				currentState = &GuildMemberCurrentState{
					UserID:   userID,
					JoinedAt: joinedAt.UTC(),
					Active:   true,
				}
				if lastSeenAt.Valid {
					currentState.LastSeenAt = lastSeenAt.Time.UTC()
				}
				if isBot.Valid {
					currentState.HasBot = true
					currentState.IsBot = isBot.Bool
				}
			}

			if roleID.Valid && strings.TrimSpace(roleID.String) != "" {
				currentState.Roles = append(currentState.Roles, roleID.String)
			}
		}

		if err := rows.Err(); err != nil {
			yield(GuildMemberCurrentState{}, fmt.Errorf("Store.GetActiveGuildMemberStatesContext: %w", err))
			return
		}

		if currentState != nil {
			yield(*currentState, nil)
		}
	}
}

// UpsertAvatar sets the current avatar hash for a member in a guild.
// If the hash changed, it records a row in avatars_history.
// Returns (changed, oldHash, err).
func (s *Store) UpsertAvatar(guildID, userID, newHash string, updatedAt time.Time) (changed bool, oldHash string, err error) {
	if guildID == "" || userID == "" {
		return false, "", nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return false, "", fmt.Errorf("Store.UpsertAvatar: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(); rerr != nil && !errors.Is(rerr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	var curHash string
	var hasCur bool
	if err := txQueryRow(tx,
		`SELECT avatar_hash FROM avatars_current WHERE guild_id=$1 AND user_id=$2`,
		guildID, userID,
	).Scan(&curHash); err != nil {
		if err != sql.ErrNoRows {
			return false, "", err
		}
	} else {
		hasCur = true
	}

	changed = !hasCur || curHash != newHash
	if changed && hasCur {
		if _, err := txExec(tx,
			`INSERT INTO avatars_history (guild_id, user_id, old_hash, new_hash, changed_at)
             VALUES ($1, $2, $3, $4, $5)`,
			guildID, userID, curHash, newHash, updatedAt,
		); err != nil {
			return false, "", err
		}
	}

	if _, err := txExec(tx,
		`INSERT INTO avatars_current (guild_id, user_id, avatar_hash, updated_at)
         VALUES ($1, $2, $3, $4)
         ON CONFLICT(guild_id, user_id) DO UPDATE SET
           avatar_hash=excluded.avatar_hash,
           updated_at=excluded.updated_at`,
		guildID, userID, newHash, updatedAt,
	); err != nil {
		return false, "", err
	}

	if err := tx.Commit(); err != nil {
		return false, "", fmt.Errorf("Store.UpsertAvatar: %w", err)
	}
	return changed, curHash, nil
}

// GetAvatar returns the current avatar hash for a user in a guild, if any.
func (s *Store) GetAvatar(guildID, userID string) (hash string, updatedAt time.Time, ok bool, err error) {
	row := s.queryRow(
		`SELECT avatar_hash, updated_at FROM avatars_current WHERE guild_id=$1 AND user_id=$2`,
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
	rows, err := s.query(`SELECT user_id, joined_at FROM member_joins WHERE guild_id=$1`, guildID)
	if err != nil {
		return nil, fmt.Errorf("Store.GetAllMemberJoins: %w", err)
	}
	defer rows.Close()

	members := make(map[string]time.Time)
	for rows.Next() {
		var userID string
		var joinedAt time.Time
		if err := rows.Scan(&userID, &joinedAt); err != nil {
			return nil, fmt.Errorf("Store.GetAllMemberJoins: %w", err)
		}
		members[userID] = joinedAt
	}
	return members, rows.Err()
}

// StreamAllGuildMemberRoles retrieves all member roles for a guild sequentially, eliding map allocations.
func (s *Store) StreamAllGuildMemberRoles(guildID string) (iter.Seq2[string, []string], error) {
	rows, err := s.query(`SELECT user_id, role_id FROM roles_current WHERE guild_id=$1 ORDER BY user_id`, guildID)
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
				currentRoles = currentRoles[:0] // Reuse slice backing array
			}
			currentUser = userID
			currentRoles = append(currentRoles, roleID)
		}
		if currentUser != "" {
			yield(currentUser, currentRoles)
		}
	}, nil
}

// TouchMemberJoin refreshes member presence freshness without mutating joined_at.
func (s *Store) TouchMemberJoin(guildID, userID string) error {
	if guildID == "" || userID == "" {
		return nil
	}
	seenAt := time.Now().UTC()
	_, err := s.exec(
		`UPDATE member_joins
         SET last_seen_at = CASE
           WHEN last_seen_at IS NULL OR $1 > last_seen_at THEN $2
           ELSE last_seen_at
         END,
             left_at = NULL
         WHERE guild_id=$3 AND user_id=$4`,
		seenAt, seenAt, guildID, userID,
	)
	return err
}

// TouchMemberRoles updates the updated_at timestamp for all member roles (keeps them fresh)
func (s *Store) TouchMemberRoles(guildID, userID string) error {
	if guildID == "" || userID == "" {
		return nil
	}
	_, err := s.exec(
		`UPDATE roles_current SET updated_at=$1 WHERE guild_id=$2 AND user_id=$3`,
		time.Now().UTC(), guildID, userID,
	)
	return err
}
