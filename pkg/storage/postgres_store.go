package storage

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"
)

// Store wraps a PostgreSQL database for durable caching of messages,
// avatar hashes (current and history), guild metadata (e.g., bot_since) and member joins.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store using an existing SQL connection. Call Init() before using it.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Init validates the database handle and ensures the migrated schema is present.
func (s *Store) Init() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store database handle is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := validateSchema(ctx, s.db); err != nil {
		return fmt.Errorf("validate schema: %w", err)
	}
	return nil
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(rebind(query), args...)
}

func (s *Store) execContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.db.ExecContext(ctx, rebind(query), args...)
}

func (s *Store) query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(rebind(query), args...)
}

func (s *Store) queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.db.QueryContext(ctx, rebind(query), args...)
}

func (s *Store) queryRow(query string, args ...any) *sql.Row {
	return s.db.QueryRow(rebind(query), args...)
}

func (s *Store) queryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.db.QueryRowContext(ctx, rebind(query), args...)
}

func txExec(tx *sql.Tx, query string, args ...any) (sql.Result, error) {
	return tx.Exec(rebind(query), args...)
}

func txQueryRow(tx *sql.Tx, query string, args ...any) *sql.Row {
	return tx.QueryRow(rebind(query), args...)
}

// rebind converts question-mark placeholders to PostgreSQL-style numbered placeholders.
func rebind(query string) string {
	if query == "" {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	index := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			b.WriteString(fmt.Sprintf("$%d", index))
			index++
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// MessageRecord represents a cached Discord message snapshot for edit/delete notifications.
type MessageRecord struct {
	GuildID        string
	MessageID      string
	ChannelID      string
	AuthorID       string
	AuthorUsername string
	AuthorAvatar   string
	Content        string
	CachedAt       time.Time
	ExpiresAt      time.Time
	HasExpiry      bool
}

// UpsertMessage inserts or updates a message record (write-through).
func (s *Store) UpsertMessage(m MessageRecord) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}

	var expires any
	if m.HasExpiry {
		expires = m.ExpiresAt.UTC()
	} else {
		expires = nil
	}
	_, err := s.exec(
		`INSERT INTO messages (guild_id, message_id, channel_id, author_id, author_username, author_avatar, content, cached_at, expires_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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

// GetMessage returns a non-expired message if present; nil if not found or expired.
func (s *Store) GetMessage(guildID, messageID string) (*MessageRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	row := s.queryRow(
		`SELECT guild_id, message_id, channel_id, author_id, author_username, author_avatar, content, cached_at, expires_at
         FROM messages
         WHERE guild_id=? AND message_id=? AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`,
		guildID, messageID,
	)

	var rec MessageRecord
	var expires sql.NullTime
	if err := row.Scan(
		&rec.GuildID,
		&rec.MessageID,
		&rec.ChannelID,
		&rec.AuthorID,
		&rec.AuthorUsername,
		&rec.AuthorAvatar,
		&rec.Content,
		&rec.CachedAt,
		&expires,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if expires.Valid {
		rec.HasExpiry = true
		rec.ExpiresAt = expires.Time
	}
	return &rec, nil
}

// DeleteMessage removes a message record (no error if absent).
func (s *Store) DeleteMessage(guildID, messageID string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	_, err := s.exec(`DELETE FROM messages WHERE guild_id=? AND message_id=?`, guildID, messageID)
	return err
}

// CleanupExpiredMessages deletes all expired messages.
func (s *Store) CleanupExpiredMessages() error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	_, err := s.exec(`DELETE FROM messages WHERE expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP`)
	return err
}

// CleanupObsoleteMemberJoins previously removed member join records based on age.
//
// IMPORTANT: we keep member join timestamps as long-lived historical data so that leave embeds
// can compute "time on server" even for members who joined long before this feature existed.
//
// For now, this is a no-op.
// If storage growth becomes a concern later, implement a cleanup policy based on an explicit
// "left_at" / "last_seen_at" signal rather than `joined_at`.
func (s *Store) CleanupObsoleteMemberJoins(_ int) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	return 0, nil
}

// CleanupObsoleteMemberRoles removes role records older than retentionDays
func (s *Store) CleanupObsoleteMemberRoles(retentionDays int) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if retentionDays <= 0 {
		retentionDays = 30 // default: keep 30 days of role history
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	result, err := s.exec(`DELETE FROM roles_current WHERE updated_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanupObsoleteAvatars removes avatar records older than retentionDays
func (s *Store) CleanupObsoleteAvatars(retentionDays int) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if retentionDays <= 0 {
		retentionDays = 180 // default: keep 6 months of avatar history
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	result, err := s.exec(`DELETE FROM avatars_history WHERE changed_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanupAllObsoleteData performs cleanup of all obsolete data with default retention periods
func (s *Store) CleanupAllObsoleteData() error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}

	// Cleanup expired messages
	if err := s.CleanupExpiredMessages(); err != nil {
		return fmt.Errorf("cleanup messages: %w", err)
	}

	// Cleanup expired cache entries
	if err := s.CleanupExpiredCacheEntries(); err != nil {
		return fmt.Errorf("cleanup cache: %w", err)
	}

	// Cleanup obsolete member joins (90 days)
	if _, err := s.CleanupObsoleteMemberJoins(90); err != nil {
		return fmt.Errorf("cleanup member joins: %w", err)
	}

	// Cleanup obsolete member roles (30 days)
	if _, err := s.CleanupObsoleteMemberRoles(30); err != nil {
		return fmt.Errorf("cleanup member roles: %w", err)
	}

	// Cleanup obsolete avatars (180 days)
	if _, err := s.CleanupObsoleteAvatars(180); err != nil {
		return fmt.Errorf("cleanup avatars: %w", err)
	}

	return nil
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
		`INSERT INTO member_joins (guild_id, user_id, joined_at, last_seen_at)
         VALUES (?, ?, ?, ?)
         ON CONFLICT(guild_id, user_id) DO UPDATE SET
           joined_at = CASE
             WHEN excluded.joined_at < member_joins.joined_at THEN excluded.joined_at
             ELSE member_joins.joined_at
           END,
           last_seen_at = CASE
             WHEN member_joins.last_seen_at IS NULL OR excluded.last_seen_at > member_joins.last_seen_at THEN excluded.last_seen_at
             ELSE member_joins.last_seen_at
           END`,
		guildID, userID, joinedAt, seenAt,
	)
	return err
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

	// Read current
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
		// Record history
		if _, err := txExec(tx,
			`INSERT INTO avatars_history (guild_id, user_id, old_hash, new_hash, changed_at)
             VALUES (?, ?, ?, ?, ?)`,
			guildID, userID, curHash, newHash, updatedAt,
		); err != nil {
			return false, "", err
		}
	}

	// Upsert current
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

// SetBotSince sets the bot_since timestamp for a guild (keeps the earliest time).
func (s *Store) SetBotSince(guildID string, t time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" {
		return nil
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.exec(
		`INSERT INTO guild_meta (guild_id, bot_since)
         VALUES (?, ?)
         ON CONFLICT(guild_id) DO UPDATE SET
           bot_since = CASE
             WHEN guild_meta.bot_since IS NULL OR ? < guild_meta.bot_since THEN ?
             ELSE guild_meta.bot_since
           END`,
		guildID, t, t, t,
	)
	return err
}

// GetBotSince returns when the bot was first seen in a guild, if available.
func (s *Store) GetBotSince(guildID string) (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.queryRow(`SELECT bot_since FROM guild_meta WHERE guild_id=?`, guildID)
	var t sql.NullTime
	if err := row.Scan(&t); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	if !t.Valid {
		return time.Time{}, false, nil
	}
	return t.Time, true, nil
}

func (s *Store) setRuntimeTimestamp(ctx context.Context, key string, t time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.execContext(
		ctx,
		`INSERT INTO runtime_meta (key, ts) VALUES (?, ?)
         ON CONFLICT(key) DO UPDATE SET ts=excluded.ts`,
		key, t.UTC(),
	)
	return err
}

func (s *Store) getRuntimeTimestamp(ctx context.Context, key string) (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.queryRowContext(ctx, `SELECT ts FROM runtime_meta WHERE key=?`, key)
	var ts time.Time
	if err := row.Scan(&ts); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return ts, true, nil
}

// SetHeartbeat records the last-known "bot is running" timestamp.
func (s *Store) SetHeartbeat(t time.Time) error {
	return s.SetHeartbeatContext(context.Background(), t)
}

// SetHeartbeatContext records the last-known "bot is running" timestamp with context support.
func (s *Store) SetHeartbeatContext(ctx context.Context, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "heartbeat", t)
}

// GetHeartbeat returns the last recorded heartbeat timestamp, if any.
func (s *Store) GetHeartbeat() (time.Time, bool, error) {
	return s.getRuntimeTimestamp(context.Background(), "heartbeat")
}

// SetLastEvent records the last time a relevant Discord event was processed.
func (s *Store) SetLastEvent(t time.Time) error {
	return s.SetLastEventContext(context.Background(), t)
}

// SetLastEventContext records the last time a relevant Discord event was processed with context support.
func (s *Store) SetLastEventContext(ctx context.Context, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "last_event", t)
}

// GetLastEvent returns the last recorded event timestamp, if any.
func (s *Store) GetLastEvent() (time.Time, bool, error) {
	return s.getRuntimeTimestamp(context.Background(), "last_event")
}

// SetMetadata records a timestamp associated with a specific key.
func (s *Store) SetMetadata(key string, t time.Time) error {
	return s.SetMetadataContext(context.Background(), key, t)
}

// SetMetadataContext records a timestamp associated with a specific key with context support.
func (s *Store) SetMetadataContext(ctx context.Context, key string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, key, t)
}

// GetMetadata retrieves the timestamp for a specific key.
func (s *Store) GetMetadata(key string) (time.Time, bool, error) {
	return s.GetMetadataContext(context.Background(), key)
}

// GetMetadataContext retrieves the timestamp for a specific key with context support.
func (s *Store) GetMetadataContext(ctx context.Context, key string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, key)
}

// NextModerationCaseNumber atomically increments and returns the next moderation case number for a guild.
func (s *Store) NextModerationCaseNumber(guildID string) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return 0, fmt.Errorf("guildID is empty")
	}

	var next int64
	if err := s.queryRow(
		`INSERT INTO moderation_cases (guild_id, last_case_number)
         VALUES (?, 1)
         ON CONFLICT(guild_id) DO UPDATE
         SET last_case_number = moderation_cases.last_case_number + 1
         RETURNING last_case_number`,
		guildID,
	).Scan(&next); err != nil {
		return 0, err
	}
	return next, nil
}

// SetGuildOwnerID sets or updates the cached owner ID for a guild.
func (s *Store) SetGuildOwnerID(guildID, ownerID string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || ownerID == "" {
		return nil
	}
	_, err := s.exec(
		`INSERT INTO guild_meta (guild_id, owner_id)
         VALUES (?, ?)
         ON CONFLICT(guild_id) DO UPDATE SET
           owner_id=excluded.owner_id`,
		guildID, ownerID,
	)
	return err
}

// GetGuildOwnerID retrieves the cached owner ID for a guild, if any.
func (s *Store) GetGuildOwnerID(guildID string) (string, bool, error) {
	if s.db == nil {
		return "", false, fmt.Errorf("store not initialized")
	}
	row := s.queryRow(`SELECT owner_id FROM guild_meta WHERE guild_id=?`, guildID)
	var owner sql.NullString
	if err := row.Scan(&owner); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	if !owner.Valid || owner.String == "" {
		return "", false, nil
	}
	return owner.String, true, nil
}

// UpsertMemberRoles replaces the current set of roles for a member in a guild atomically.
func (s *Store) UpsertMemberRoles(guildID, userID string, roles []string, updatedAt time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || userID == "" {
		return nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing roles for member
	if _, err := txExec(tx, `DELETE FROM roles_current WHERE guild_id=? AND user_id=?`, guildID, userID); err != nil {
		return err
	}
	// Insert new set
	for _, rid := range roles {
		if rid == "" {
			continue
		}
		if _, err := txExec(tx,
			`INSERT INTO roles_current (guild_id, user_id, role_id, updated_at) VALUES (?, ?, ?, ?)`,
			guildID, userID, rid, updatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetMemberRoles returns the current cached roles for a member in a guild.
func (s *Store) GetMemberRoles(guildID, userID string) ([]string, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.query(`SELECT role_id FROM roles_current WHERE guild_id=? AND user_id=?`, guildID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var rid string
		if err := rows.Scan(&rid); err != nil {
			return nil, err
		}
		if rid != "" {
			roles = append(roles, rid)
		}
	}
	return roles, rows.Err()
}

// DiffMemberRoles compares the cached set of roles with the provided current set and returns deltas.
func (s *Store) DiffMemberRoles(guildID, userID string, current []string) (added []string, removed []string, err error) {
	cached, err := s.GetMemberRoles(guildID, userID)
	if err != nil {
		return nil, nil, err
	}
	curSet := make(map[string]struct{}, len(current))
	for _, r := range current {
		if r != "" {
			curSet[r] = struct{}{}
		}
	}
	cacheSet := make(map[string]struct{}, len(cached))
	for _, r := range cached {
		if r != "" {
			cacheSet[r] = struct{}{}
		}
	}
	for r := range curSet {
		if _, ok := cacheSet[r]; !ok {
			added = append(added, r)
		}
	}
	for r := range cacheSet {
		if _, ok := curSet[r]; !ok {
			removed = append(removed, r)
		}
	}
	return added, removed, nil
}

var requiredSchemaTables = []string{
	"messages",
	"member_joins",
	"avatars_current",
	"avatars_history",
	"messages_history",
	"guild_meta",
	"runtime_meta",
	"moderation_cases",
	"roles_current",
	"persistent_cache",
	"daily_message_metrics",
	"daily_reaction_metrics",
	"daily_member_joins",
	"daily_member_leaves",
}

var requiredSchemaColumns = map[string][]string{
	"member_joins": []string{"last_seen_at"},
}

func validateSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	missing := make([]string, 0)
	for _, table := range requiredSchemaTables {
		var regclass sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1)`, table).Scan(&regclass); err != nil {
			return fmt.Errorf("check table %s existence: %w", table, err)
		}
		if !regclass.Valid || strings.TrimSpace(regclass.String) == "" {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing migrated tables (%s); apply postgres migrations before initializing store", strings.Join(missing, ", "))
	}

	missingColumns := make([]string, 0)
	for table, columns := range requiredSchemaColumns {
		for _, column := range columns {
			var exists bool
			if err := db.QueryRowContext(
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
				return fmt.Errorf("check column %s.%s existence: %w", table, column, err)
			}
			if !exists {
				missingColumns = append(missingColumns, table+"."+column)
			}
		}
	}
	if len(missingColumns) > 0 {
		return fmt.Errorf("missing migrated columns (%s); apply postgres migrations before initializing store", strings.Join(missingColumns, ", "))
	}
	return nil
}

// Message Versioning (history)

type MessageVersion struct {
	GuildID     string
	MessageID   string
	ChannelID   string
	AuthorID    string
	Version     int
	EventType   string // "create" | "edit" | "delete"
	Content     string
	Attachments int
	Embeds      int
	Stickers    int
	CreatedAt   time.Time
}

// InsertMessageVersion inserts a new version row for a message.
// If Version <= 0, it will compute next version as (MAX(version)+1) within (guild_id, message_id).
func (s *Store) InsertMessageVersion(v MessageVersion) error {
	return s.InsertMessageVersionContext(context.Background(), v)
}

// InsertMessageVersionContext inserts a new version row for a message with context support.
// If Version <= 0, it computes the next version atomically under an advisory transaction lock.
func (s *Store) InsertMessageVersionContext(ctx context.Context, v MessageVersion) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// basic validation
	if v.GuildID == "" || v.MessageID == "" || v.ChannelID == "" || v.AuthorID == "" || v.EventType == "" {
		return nil
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin message version tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	lockKey := messageVersionAdvisoryLockKey(v.GuildID, v.MessageID)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
		return fmt.Errorf("acquire message version advisory lock: %w", err)
	}

	// Compute next version if not provided
	if v.Version <= 0 {
		var cur sql.NullInt64
		if err := tx.QueryRowContext(ctx, rebind(
			`SELECT COALESCE(MAX(version),0) FROM messages_history WHERE guild_id=? AND message_id=?`,
		),
			v.GuildID, v.MessageID,
		).Scan(&cur); err != nil {
			return fmt.Errorf("read max message version: %w", err)
		}
		v.Version = int(cur.Int64) + 1
	}

	// Insert history row
	if _, err := tx.ExecContext(ctx, rebind(
		`INSERT INTO messages_history
         (guild_id, message_id, channel_id, author_id, version, event_type, content, attachments, embeds_count, stickers, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	),
		v.GuildID, v.MessageID, v.ChannelID, v.AuthorID, v.Version, v.EventType, v.Content, v.Attachments, v.Embeds, v.Stickers, v.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert message version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit message version tx: %w", err)
	}
	return nil
}

func messageVersionAdvisoryLockKey(guildID, messageID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(guildID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(messageID))
	return int64(h.Sum64())
}

// Persistent Cache Methods

// UpsertCacheEntry saves a cache entry to persistent storage
func (s *Store) UpsertCacheEntry(key, cacheType, data string, expiresAt time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if key == "" || cacheType == "" || data == "" {
		return nil
	}
	_, err := s.exec(
		`INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(cache_key) DO UPDATE SET
           data=excluded.data,
           expires_at=excluded.expires_at,
           cached_at=excluded.cached_at`,
		key, cacheType, data, expiresAt, time.Now().UTC(),
	)
	return err
}

// GetCacheEntry retrieves a cache entry from persistent storage
func (s *Store) GetCacheEntry(key string) (cacheType, data string, expiresAt time.Time, ok bool, err error) {
	if s.db == nil {
		return "", "", time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.queryRow(
		`SELECT cache_type, data, expires_at FROM persistent_cache WHERE cache_key=?`,
		key,
	)
	err = row.Scan(&cacheType, &data, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", time.Time{}, false, nil
		}
		return "", "", time.Time{}, false, err
	}
	// Check if expired
	if time.Now().After(expiresAt) {
		return "", "", time.Time{}, false, nil
	}
	return cacheType, data, expiresAt, true, nil
}

// GetCacheEntriesByType retrieves all cache entries of a specific type
func (s *Store) GetCacheEntriesByType(cacheType string) ([]struct {
	Key       string
	Data      string
	ExpiresAt time.Time
}, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.query(
		`SELECT cache_key, data, expires_at FROM persistent_cache
         WHERE cache_type=? AND expires_at > ?`,
		cacheType, time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []struct {
		Key       string
		Data      string
		ExpiresAt time.Time
	}
	for rows.Next() {
		var entry struct {
			Key       string
			Data      string
			ExpiresAt time.Time
		}
		if err := rows.Scan(&entry.Key, &entry.Data, &entry.ExpiresAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// DeleteCacheEntry removes a cache entry from persistent storage
func (s *Store) DeleteCacheEntry(key string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE cache_key=?`, key)
	return err
}

// CleanupExpiredCacheEntries removes all expired cache entries
func (s *Store) CleanupExpiredCacheEntries() error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE expires_at <= ?`, time.Now().UTC())
	return err
}

// DeleteCacheEntriesByPrefix deletes all cache entries with keys starting with the given prefix
func (s *Store) DeleteCacheEntriesByPrefix(prefix string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if prefix == "" {
		return nil
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE cache_key LIKE ?`, prefix+"%")
	return err
}

// DeleteCacheEntriesByTypeAndPrefix deletes cache entries filtered by cache_type and key prefix
func (s *Store) DeleteCacheEntriesByTypeAndPrefix(cacheType, keyPrefix string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if cacheType == "" || keyPrefix == "" {
		return nil
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE cache_type=? AND cache_key LIKE ?`, cacheType, keyPrefix+"%")
	return err
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
//
// Historical member joins remain immutable because downstream features use joined_at
// as the canonical "first seen in guild" timestamp. Callers with an authoritative
// join time should use UpsertMemberJoin, which preserves the earliest known value.
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
         END
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

// GetCacheStats returns statistics about the persistent cache
func (s *Store) GetCacheStats() (map[string]int, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.query(
		`SELECT cache_type, COUNT(*) as count FROM persistent_cache
         WHERE expires_at > ? GROUP BY cache_type`,
		time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var cacheType string
		var count int
		if err := rows.Scan(&cacheType, &count); err != nil {
			return nil, err
		}
		stats[cacheType] = count
	}
	return stats, rows.Err()
}

// IncrementDailyMessageCount increments the per-day message count for a user in a channel.
func (s *Store) IncrementDailyMessageCount(guildID, channelID, userID string, at time.Time) error {
	return s.IncrementDailyMessageCountContext(context.Background(), guildID, channelID, userID, at)
}

func dailyMetricDay(at time.Time) string {
	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func (s *Store) incrementDailyMetricByChannelAndUser(ctx context.Context, tableName, guildID, channelID, userID string, at time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	query := fmt.Sprintf(
		`INSERT INTO %s (guild_id, channel_id, user_id, day, count)
         VALUES (?, ?, ?, ?, 1)
         ON CONFLICT(guild_id, channel_id, user_id, day) DO UPDATE SET
           count = %s.count + 1`,
		tableName,
		tableName,
	)
	_, err := s.execContext(ctx, query, guildID, channelID, userID, dailyMetricDay(at))
	return err
}

func (s *Store) incrementDailyMetricByUser(ctx context.Context, tableName, guildID, userID string, at time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	query := fmt.Sprintf(
		`INSERT INTO %s (guild_id, user_id, day, count)
         VALUES (?, ?, ?, 1)
         ON CONFLICT(guild_id, user_id, day) DO UPDATE SET
           count = %s.count + 1`,
		tableName,
		tableName,
	)
	_, err := s.execContext(ctx, query, guildID, userID, dailyMetricDay(at))
	return err
}

// IncrementDailyMessageCountContext increments the per-day message count for a user in a channel with context support.
func (s *Store) IncrementDailyMessageCountContext(ctx context.Context, guildID, channelID, userID string, at time.Time) error {
	if guildID == "" || channelID == "" || userID == "" {
		return nil
	}
	return s.incrementDailyMetricByChannelAndUser(ctx, "daily_message_metrics", guildID, channelID, userID, at)
}

// IncrementDailyReactionCount increments the per-day reaction count for a user in a channel.
func (s *Store) IncrementDailyReactionCount(guildID, channelID, userID string, at time.Time) error {
	return s.IncrementDailyReactionCountContext(context.Background(), guildID, channelID, userID, at)
}

// IncrementDailyReactionCountContext increments the per-day reaction count for a user in a channel with context support.
func (s *Store) IncrementDailyReactionCountContext(ctx context.Context, guildID, channelID, userID string, at time.Time) error {
	if guildID == "" || channelID == "" || userID == "" {
		return nil
	}
	return s.incrementDailyMetricByChannelAndUser(ctx, "daily_reaction_metrics", guildID, channelID, userID, at)
}

// IncrementDailyMemberJoin increments the per-day member join counter (per user).
func (s *Store) IncrementDailyMemberJoin(guildID, userID string, at time.Time) error {
	return s.IncrementDailyMemberJoinContext(context.Background(), guildID, userID, at)
}

// IncrementDailyMemberJoinContext increments the per-day member join counter (per user) with context support.
func (s *Store) IncrementDailyMemberJoinContext(ctx context.Context, guildID, userID string, at time.Time) error {
	if guildID == "" || userID == "" {
		return nil
	}
	return s.incrementDailyMetricByUser(ctx, "daily_member_joins", guildID, userID, at)
}

// IncrementDailyMemberLeave increments the per-day member leave counter (per user).
func (s *Store) IncrementDailyMemberLeave(guildID, userID string, at time.Time) error {
	return s.IncrementDailyMemberLeaveContext(context.Background(), guildID, userID, at)
}

// IncrementDailyMemberLeaveContext increments the per-day member leave counter (per user) with context support.
func (s *Store) IncrementDailyMemberLeaveContext(ctx context.Context, guildID, userID string, at time.Time) error {
	if guildID == "" || userID == "" {
		return nil
	}
	return s.incrementDailyMetricByUser(ctx, "daily_member_leaves", guildID, userID, at)
}

// MetricTotal represents an aggregated metric value for a specific key.
type MetricTotal struct {
	Key   string
	Total int64
}

func (s *Store) metricTotalsByDimension(ctx context.Context, tableName, dimension, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if guildID == "" || cutoffDay == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	baseSQL := fmt.Sprintf(
		"SELECT %s, SUM(count) FROM %s WHERE guild_id=? AND day>=?",
		dimension, tableName,
	)
	args := []any{guildID, cutoffDay}
	if channelID != "" {
		baseSQL += " AND channel_id=?"
		args = append(args, channelID)
	}
	baseSQL += fmt.Sprintf(" GROUP BY %s", dimension)

	rows, err := s.db.QueryContext(ctx, rebind(baseSQL), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MetricTotal, 0, 16)
	for rows.Next() {
		var key string
		var total sql.NullInt64
		if err := rows.Scan(&key, &total); err != nil {
			return nil, err
		}
		if total.Valid {
			out = append(out, MetricTotal{Key: key, Total: total.Int64})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out, nil
}

func (s *Store) MessageTotalsByChannel(ctx context.Context, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	return s.metricTotalsByDimension(ctx, "daily_message_metrics", "channel_id", guildID, cutoffDay, channelID)
}

func (s *Store) MessageTotalsByUser(ctx context.Context, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	return s.metricTotalsByDimension(ctx, "daily_message_metrics", "user_id", guildID, cutoffDay, channelID)
}

func (s *Store) ReactionTotalsByChannel(ctx context.Context, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	return s.metricTotalsByDimension(ctx, "daily_reaction_metrics", "channel_id", guildID, cutoffDay, channelID)
}

func (s *Store) ReactionTotalsByUser(ctx context.Context, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	return s.metricTotalsByDimension(ctx, "daily_reaction_metrics", "user_id", guildID, cutoffDay, channelID)
}

func (s *Store) CountDistinctMemberJoins(ctx context.Context, guildID string) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if guildID == "" {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var total sql.NullInt64
	if err := s.db.QueryRowContext(ctx, rebind(`SELECT COUNT(DISTINCT user_id) FROM member_joins WHERE guild_id=?`), guildID).Scan(&total); err != nil {
		return 0, err
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}

func (s *Store) ListDistinctMemberJoinUserIDs(ctx context.Context, guildID string) ([]string, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if guildID == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rows, err := s.db.QueryContext(ctx, rebind(`SELECT DISTINCT user_id FROM member_joins WHERE guild_id=?`), guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, 128)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(userID) != "" {
			out = append(out, userID)
		}
	}
	return out, rows.Err()
}

func (s *Store) SumDailyMemberJoinsSince(ctx context.Context, guildID, cutoffDay string) (int64, error) {
	return s.sumMetricSince(ctx, `SELECT SUM(count) FROM daily_member_joins WHERE guild_id=? AND day>=?`, guildID, cutoffDay)
}

func (s *Store) SumDailyMemberLeavesSince(ctx context.Context, guildID, cutoffDay string) (int64, error) {
	return s.sumMetricSince(ctx, `SELECT SUM(count) FROM daily_member_leaves WHERE guild_id=? AND day>=?`, guildID, cutoffDay)
}

func (s *Store) sumMetricSince(ctx context.Context, query, guildID, cutoffDay string) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if guildID == "" || cutoffDay == "" {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var total sql.NullInt64
	if err := s.db.QueryRowContext(ctx, rebind(query), guildID, cutoffDay).Scan(&total); err != nil {
		return 0, err
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}

func (s *Store) DatabaseSizeBytes(ctx context.Context) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var size sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT pg_database_size(current_database())`).Scan(&size); err != nil {
		return 0, err
	}
	if size.Valid {
		return size.Int64, nil
	}
	return 0, nil
}
