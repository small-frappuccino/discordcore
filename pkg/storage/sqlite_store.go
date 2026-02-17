package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps an embedded SQLite database for durable caching of messages,
// avatar hashes (current and history), guild metadata (e.g., bot_since) and member joins.
// It uses modernc.org/sqlite for CGO-less builds.
type Store struct {
	dbPath string
	db     *sql.DB
}

// NewStore creates a new Store pointing to dbPath. Call Init() before using it.
func NewStore(dbPath string) *Store {
	return &Store{dbPath: dbPath}
}

// Init opens the SQLite database, configures pragmas, and ensures the schema exists.
func (s *Store) Init() error {
	if s.db != nil {
		return nil
	}
	if s.dbPath == "" {
		return fmt.Errorf("db path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	// Pragmas for durability and concurrency
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return fmt.Errorf("set WAL: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return fmt.Errorf("enable FKs: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return fmt.Errorf("set busy_timeout: %w", err)
	}
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL;`); err != nil {
		_ = db.Close()
		return fmt.Errorf("set synchronous: %w", err)
	}

	// Schema creation
	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return err
	}

	s.db = db
	return nil
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
	_, err := s.db.Exec(
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

	row := s.db.QueryRow(
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
	_, err := s.db.Exec(`DELETE FROM messages WHERE guild_id=? AND message_id=?`, guildID, messageID)
	return err
}

// CleanupExpiredMessages deletes all expired messages.
func (s *Store) CleanupExpiredMessages() error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	_, err := s.db.Exec(`DELETE FROM messages WHERE expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP`)
	return err
}

// CleanupObsoleteMemberJoins previously removed member join records based on age.
//
// IMPORTANT: we keep member join timestamps as long-lived historical data so that leave embeds
// can compute "time on server" even for members who joined long before this feature existed.
//
// For now, this is a no-op.
// If storage growth becomes a concern later, implement a cleanup policy based on an explicit
// "left_at" / "last_seen" signal rather than `joined_at`.
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
	result, err := s.db.Exec(`DELETE FROM roles_current WHERE updated_at < ?`, cutoff)
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
	result, err := s.db.Exec(`DELETE FROM avatars_history WHERE changed_at < ?`, cutoff)
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
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || userID == "" || joinedAt.IsZero() {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO member_joins (guild_id, user_id, joined_at)
         VALUES (?, ?, ?)
         ON CONFLICT(guild_id, user_id) DO UPDATE SET
           joined_at = CASE
             WHEN excluded.joined_at < member_joins.joined_at THEN excluded.joined_at
             ELSE member_joins.joined_at
           END`,
		guildID, userID, joinedAt.UTC(),
	)
	return err
}

// GetMemberJoin returns the stored join time for a member, if any.
func (s *Store) GetMemberJoin(guildID, userID string) (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.db.QueryRow(`SELECT joined_at FROM member_joins WHERE guild_id=? AND user_id=?`, guildID, userID)
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
	if err := tx.QueryRow(
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
		if _, err := tx.Exec(
			`INSERT INTO avatars_history (guild_id, user_id, old_hash, new_hash, changed_at)
             VALUES (?, ?, ?, ?, ?)`,
			guildID, userID, curHash, newHash, updatedAt,
		); err != nil {
			return false, "", err
		}
	}

	// Upsert current
	if _, err := tx.Exec(
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
	row := s.db.QueryRow(
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
	_, err := s.db.Exec(
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
	row := s.db.QueryRow(`SELECT bot_since FROM guild_meta WHERE guild_id=?`, guildID)
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

// SetHeartbeat records the last-known "bot is running" timestamp.
func (s *Store) SetHeartbeat(t time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO runtime_meta (key, ts) VALUES (?, ?)
         ON CONFLICT(key) DO UPDATE SET ts=excluded.ts`,
		"heartbeat", t.UTC(),
	)
	return err
}

// GetHeartbeat returns the last recorded heartbeat timestamp, if any.
func (s *Store) GetHeartbeat() (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.db.QueryRow(`SELECT ts FROM runtime_meta WHERE key=?`, "heartbeat")
	var ts time.Time
	if err := row.Scan(&ts); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return ts, true, nil
}

// SetLastEvent records the last time a relevant Discord event was processed.
func (s *Store) SetLastEvent(t time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO runtime_meta (key, ts) VALUES (?, ?)
         ON CONFLICT(key) DO UPDATE SET ts=excluded.ts`,
		"last_event", t.UTC(),
	)
	return err
}

// GetLastEvent returns the last recorded event timestamp, if any.
func (s *Store) GetLastEvent() (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.db.QueryRow(`SELECT ts FROM runtime_meta WHERE key=?`, "last_event")
	var ts time.Time
	if err := row.Scan(&ts); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return ts, true, nil
}

// SetMetadata records a timestamp associated with a specific key.
func (s *Store) SetMetadata(key string, t time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO runtime_meta (key, ts) VALUES (?, ?)
         ON CONFLICT(key) DO UPDATE SET ts=excluded.ts`,
		key, t.UTC(),
	)
	return err
}

// GetMetadata retrieves the timestamp for a specific key.
func (s *Store) GetMetadata(key string) (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.db.QueryRow(`SELECT ts FROM runtime_meta WHERE key=?`, key)
	var ts time.Time
	if err := row.Scan(&ts); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return ts, true, nil
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

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(
		`INSERT INTO moderation_cases (guild_id, last_case_number)
         VALUES (?, 0)
         ON CONFLICT(guild_id) DO NOTHING`,
		guildID,
	); err != nil {
		return 0, err
	}

	if _, err := tx.Exec(
		`UPDATE moderation_cases
         SET last_case_number = last_case_number + 1
         WHERE guild_id = ?`,
		guildID,
	); err != nil {
		return 0, err
	}

	var next int64
	if err := tx.QueryRow(
		`SELECT last_case_number FROM moderation_cases WHERE guild_id = ?`,
		guildID,
	).Scan(&next); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
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
	_, err := s.db.Exec(
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
	row := s.db.QueryRow(`SELECT owner_id FROM guild_meta WHERE guild_id=?`, guildID)
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
	if _, err := tx.Exec(`DELETE FROM roles_current WHERE guild_id=? AND user_id=?`, guildID, userID); err != nil {
		return err
	}
	// Insert new set
	for _, rid := range roles {
		if rid == "" {
			continue
		}
		if _, err := tx.Exec(
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
	rows, err := s.db.Query(`SELECT role_id FROM roles_current WHERE guild_id=? AND user_id=?`, guildID, userID)
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

// ensureSchema creates required tables and indexes if they don't exist.
func ensureSchema(db *sql.DB) error {
	const createMessages = `
CREATE TABLE IF NOT EXISTS messages (
  guild_id        TEXT NOT NULL,
  message_id      TEXT NOT NULL,
  channel_id      TEXT NOT NULL,
  author_id       TEXT NOT NULL,
  author_username TEXT,
  author_avatar   TEXT,
  content         TEXT,
  cached_at       TIMESTAMP NOT NULL,
  expires_at      TIMESTAMP,
  PRIMARY KEY (guild_id, message_id)
);
CREATE INDEX IF NOT EXISTS idx_messages_expires ON messages(expires_at);`

	const createMemberJoins = `
CREATE TABLE IF NOT EXISTS member_joins (
  guild_id   TEXT NOT NULL,
  user_id    TEXT NOT NULL,
  joined_at  TIMESTAMP NOT NULL,
  PRIMARY KEY (guild_id, user_id)
);`

	const createAvatarsCurrent = `
CREATE TABLE IF NOT EXISTS avatars_current (
  guild_id    TEXT NOT NULL,
  user_id     TEXT NOT NULL,
  avatar_hash TEXT NOT NULL,
  updated_at  TIMESTAMP NOT NULL,
  PRIMARY KEY (guild_id, user_id)
);`

	const createAvatarsHistory = `
CREATE TABLE IF NOT EXISTS avatars_history (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  guild_id   TEXT NOT NULL,
  user_id    TEXT NOT NULL,
  old_hash   TEXT,
  new_hash   TEXT,
  changed_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_avatars_hist_gid_uid ON avatars_history(guild_id, user_id);
CREATE INDEX IF NOT EXISTS idx_avatars_hist_changed ON avatars_history(changed_at);`

	const createMessagesHistory = `
CREATE TABLE IF NOT EXISTS messages_history (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  guild_id      TEXT NOT NULL,
  message_id    TEXT NOT NULL,
  channel_id    TEXT NOT NULL,
  author_id     TEXT NOT NULL,
  version       INTEGER NOT NULL,
  event_type    TEXT NOT NULL,               -- 'create' | 'edit' | 'delete'
  content       TEXT,
  attachments   INTEGER DEFAULT 0,
  embeds_count  INTEGER DEFAULT 0,
  stickers      INTEGER DEFAULT 0,
  created_at    TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_msg_hist_gid_mid ON messages_history(guild_id, message_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_hist_gid_mid_ver ON messages_history(guild_id, message_id, version);`

	const createGuildMeta = `
CREATE TABLE IF NOT EXISTS guild_meta (
  guild_id  TEXT PRIMARY KEY,
  bot_since TIMESTAMP,
  owner_id  TEXT
);`

	const createRuntimeMeta = `
CREATE TABLE IF NOT EXISTS runtime_meta (
  key TEXT PRIMARY KEY,
  ts  TIMESTAMP NOT NULL
);`

	const createModerationCases = `
CREATE TABLE IF NOT EXISTS moderation_cases (
  guild_id         TEXT PRIMARY KEY,
  last_case_number INTEGER NOT NULL DEFAULT 0
 );`

	const createRolesCurrent = `
CREATE TABLE IF NOT EXISTS roles_current (
  guild_id   TEXT NOT NULL,
  user_id    TEXT NOT NULL,
  role_id    TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (guild_id, user_id, role_id)
);
CREATE INDEX IF NOT EXISTS idx_roles_current_member ON roles_current(guild_id, user_id);`

	const createPersistentCache = `
CREATE TABLE IF NOT EXISTS persistent_cache (
  cache_key  TEXT PRIMARY KEY,
  cache_type TEXT NOT NULL,
  data       TEXT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  cached_at  TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_persistent_cache_type ON persistent_cache(cache_type);
CREATE INDEX IF NOT EXISTS idx_persistent_cache_expires ON persistent_cache(expires_at);`

	const createDailyMessageMetrics = `
CREATE TABLE IF NOT EXISTS daily_message_metrics (
  guild_id   TEXT NOT NULL,
  channel_id TEXT NOT NULL,
  user_id    TEXT NOT NULL,
  day        DATE NOT NULL,
  count      INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (guild_id, channel_id, user_id, day)
);
CREATE INDEX IF NOT EXISTS idx_daily_msg_by_guild_day ON daily_message_metrics(guild_id, day);
CREATE INDEX IF NOT EXISTS idx_daily_msg_by_channel_day ON daily_message_metrics(channel_id, day);`

	const createDailyReactionMetrics = `
CREATE TABLE IF NOT EXISTS daily_reaction_metrics (
  guild_id   TEXT NOT NULL,
  channel_id TEXT NOT NULL,
  user_id    TEXT NOT NULL,              -- reactor user id
  day        DATE NOT NULL,
  count      INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (guild_id, channel_id, user_id, day)
);
CREATE INDEX IF NOT EXISTS idx_daily_react_by_guild_day ON daily_reaction_metrics(guild_id, day);
CREATE INDEX IF NOT EXISTS idx_daily_react_by_channel_day ON daily_reaction_metrics(channel_id, day);`

	const createDailyMemberJoinsMetrics = `
CREATE TABLE IF NOT EXISTS daily_member_joins (
  guild_id TEXT NOT NULL,
  user_id  TEXT NOT NULL,
  day      DATE NOT NULL,
  count    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (guild_id, user_id, day)
);
CREATE INDEX IF NOT EXISTS idx_daily_joins_by_guild_day ON daily_member_joins(guild_id, day);`

	const createDailyMemberLeavesMetrics = `
CREATE TABLE IF NOT EXISTS daily_member_leaves (
  guild_id TEXT NOT NULL,
  user_id  TEXT NOT NULL,
  day      DATE NOT NULL,
  count    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (guild_id, user_id, day)
);
CREATE INDEX IF NOT EXISTS idx_daily_leaves_by_guild_day ON daily_member_leaves(guild_id, day);`

	stmts := []string{
		createMessages,
		createMessagesHistory,
		createMemberJoins,
		createAvatarsCurrent,
		createAvatarsHistory,
		createGuildMeta,
		createRuntimeMeta,
		createModerationCases,
		createRolesCurrent,
		createPersistentCache,
		createDailyMessageMetrics,
		createDailyReactionMetrics,
		createDailyMemberJoinsMetrics,
		createDailyMemberLeavesMetrics,
	}
	for _, sqlText := range stmts {
		if _, err := db.Exec(sqlText); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
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
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	// basic validation
	if v.GuildID == "" || v.MessageID == "" || v.ChannelID == "" || v.AuthorID == "" || v.EventType == "" {
		return nil
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Compute next version if not provided
	if v.Version <= 0 {
		var cur sql.NullInt64
		if err := tx.QueryRow(
			`SELECT COALESCE(MAX(version),0) FROM messages_history WHERE guild_id=? AND message_id=?`,
			v.GuildID, v.MessageID,
		).Scan(&cur); err != nil {
			return err
		}
		v.Version = int(cur.Int64) + 1
	}

	// Insert history row
	if _, err := tx.Exec(
		`INSERT INTO messages_history
         (guild_id, message_id, channel_id, author_id, version, event_type, content, attachments, embeds_count, stickers, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.GuildID, v.MessageID, v.ChannelID, v.AuthorID, v.Version, v.EventType, v.Content, v.Attachments, v.Embeds, v.Stickers, v.CreatedAt,
	); err != nil {
		return err
	}

	return tx.Commit()
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
	_, err := s.db.Exec(
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
	row := s.db.QueryRow(
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
	rows, err := s.db.Query(
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
	_, err := s.db.Exec(`DELETE FROM persistent_cache WHERE cache_key=?`, key)
	return err
}

// CleanupExpiredCacheEntries removes all expired cache entries
func (s *Store) CleanupExpiredCacheEntries() error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	_, err := s.db.Exec(`DELETE FROM persistent_cache WHERE expires_at <= ?`, time.Now().UTC())
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
	_, err := s.db.Exec(`DELETE FROM persistent_cache WHERE cache_key LIKE ?`, prefix+"%")
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
	_, err := s.db.Exec(`DELETE FROM persistent_cache WHERE cache_type=? AND cache_key LIKE ?`, cacheType, keyPrefix+"%")
	return err
}

// GetAllMemberJoins retrieves all member join records for a guild
func (s *Store) GetAllMemberJoins(guildID string) (map[string]time.Time, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.db.Query(`SELECT user_id, joined_at FROM member_joins WHERE guild_id=?`, guildID)
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
	rows, err := s.db.Query(`SELECT user_id, role_id FROM roles_current WHERE guild_id=?`, guildID)
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

// TouchMemberJoin updates the joined_at timestamp for a member (keeps it fresh)
func (s *Store) TouchMemberJoin(guildID, userID string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || userID == "" {
		return nil
	}
	_, err := s.db.Exec(
		`UPDATE member_joins SET joined_at=? WHERE guild_id=? AND user_id=?`,
		time.Now().UTC(), guildID, userID,
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
	_, err := s.db.Exec(
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
	rows, err := s.db.Query(
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
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || channelID == "" || userID == "" {
		return nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	_, err := s.db.Exec(
		`INSERT INTO daily_message_metrics (guild_id, channel_id, user_id, day, count)
         VALUES (?, ?, ?, ?, 1)
         ON CONFLICT(guild_id, channel_id, user_id, day) DO UPDATE SET
           count = count + 1`,
		guildID, channelID, userID, day,
	)
	return err
}

// IncrementDailyReactionCount increments the per-day reaction count for a user in a channel.
func (s *Store) IncrementDailyReactionCount(guildID, channelID, userID string, at time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || channelID == "" || userID == "" {
		return nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	_, err := s.db.Exec(
		`INSERT INTO daily_reaction_metrics (guild_id, channel_id, user_id, day, count)
         VALUES (?, ?, ?, ?, 1)
         ON CONFLICT(guild_id, channel_id, user_id, day) DO UPDATE SET
           count = count + 1`,
		guildID, channelID, userID, day,
	)
	return err
}

// IncrementDailyMemberJoin increments the per-day member join counter (per user).
func (s *Store) IncrementDailyMemberJoin(guildID, userID string, at time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || userID == "" {
		return nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	_, err := s.db.Exec(
		`INSERT INTO daily_member_joins (guild_id, user_id, day, count)
         VALUES (?, ?, ?, 1)
         ON CONFLICT(guild_id, user_id, day) DO UPDATE SET
           count = count + 1`,
		guildID, userID, day,
	)
	return err
}

// IncrementDailyMemberLeave increments the per-day member leave counter (per user).
func (s *Store) IncrementDailyMemberLeave(guildID, userID string, at time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" || userID == "" {
		return nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	_, err := s.db.Exec(
		`INSERT INTO daily_member_leaves (guild_id, user_id, day, count)
         VALUES (?, ?, ?, 1)
         ON CONFLICT(guild_id, user_id, day) DO UPDATE SET
           count = count + 1`,
		guildID, userID, day,
	)
	return err
}
