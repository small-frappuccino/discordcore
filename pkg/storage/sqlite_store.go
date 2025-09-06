package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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

	const createGuildMeta = `
CREATE TABLE IF NOT EXISTS guild_meta (
  guild_id  TEXT PRIMARY KEY,
  bot_since TIMESTAMP
);`

	stmts := []string{
		createMessages,
		createMemberJoins,
		createAvatarsCurrent,
		createAvatarsHistory,
		createGuildMeta,
	}
	for _, sqlText := range stmts {
		if _, err := db.Exec(sqlText); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}
	return nil
}
