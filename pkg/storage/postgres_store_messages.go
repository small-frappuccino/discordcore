package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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

// MessageDeleteKey identifies one cached message row to delete.
type MessageDeleteKey struct {
	GuildID   string
	MessageID string
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

// UpsertMessagesContext upserts a batch of cached messages.
func (s *Store) UpsertMessagesContext(ctx context.Context, records []MessageRecord) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	normalized := normalizeMessageRecords(records)
	if len(normalized) == 0 {
		return nil
	}

	return execValuesContext(ctx, s.db,
		`INSERT INTO messages (guild_id, message_id, channel_id, author_id, author_username, author_avatar, content, cached_at, expires_at) VALUES `,
		` ON CONFLICT(guild_id, message_id) DO UPDATE SET
           channel_id=excluded.channel_id,
           author_id=excluded.author_id,
           author_username=excluded.author_username,
           author_avatar=excluded.author_avatar,
           content=excluded.content,
           cached_at=excluded.cached_at,
           expires_at=excluded.expires_at`,
		len(normalized),
		9,
		func(args []any, rowIndex int) []any {
			record := normalized[rowIndex]
			var expires any
			if record.HasExpiry {
				expires = record.ExpiresAt.UTC()
			}
			return append(args,
				record.GuildID,
				record.MessageID,
				record.ChannelID,
				record.AuthorID,
				record.AuthorUsername,
				record.AuthorAvatar,
				record.Content,
				record.CachedAt.UTC(),
				expires,
			)
		},
	)
}

func normalizeMessageRecords(records []MessageRecord) []MessageRecord {
	if len(records) == 0 {
		return nil
	}
	order := make([]string, 0, len(records))
	byKey := make(map[string]MessageRecord, len(records))
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

	normalized := make([]MessageRecord, 0, len(order))
	for _, key := range order {
		normalized = append(normalized, byKey[key])
	}
	return normalized
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

// DeleteMessagesContext removes a batch of message records.
func (s *Store) DeleteMessagesContext(ctx context.Context, keys []MessageDeleteKey) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	normalized := normalizeMessageDeleteKeys(keys)
	if len(normalized) == 0 {
		return nil
	}

	return execValuesContext(ctx, s.db,
		`DELETE FROM messages USING (VALUES `,
		`) AS doomed(guild_id, message_id) WHERE messages.guild_id = doomed.guild_id AND messages.message_id = doomed.message_id`,
		len(normalized),
		2,
		func(args []any, rowIndex int) []any {
			key := normalized[rowIndex]
			return append(args, key.GuildID, key.MessageID)
		},
	)
}

func normalizeMessageDeleteKeys(keys []MessageDeleteKey) []MessageDeleteKey {
	if len(keys) == 0 {
		return nil
	}
	order := make([]string, 0, len(keys))
	byKey := make(map[string]MessageDeleteKey, len(keys))
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

	normalized := make([]MessageDeleteKey, 0, len(order))
	for _, composite := range order {
		normalized = append(normalized, byKey[composite])
	}
	return normalized
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
		retentionDays = 30
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
		retentionDays = 180
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

	if err := s.CleanupExpiredMessages(); err != nil {
		return fmt.Errorf("cleanup messages: %w", err)
	}
	if err := s.CleanupExpiredCacheEntries(); err != nil {
		return fmt.Errorf("cleanup cache: %w", err)
	}
	if _, err := s.CleanupObsoleteMemberJoins(90); err != nil {
		return fmt.Errorf("cleanup member joins: %w", err)
	}
	if _, err := s.CleanupObsoleteMemberRoles(30); err != nil {
		return fmt.Errorf("cleanup member roles: %w", err)
	}
	if _, err := s.CleanupObsoleteAvatars(180); err != nil {
		return fmt.Errorf("cleanup avatars: %w", err)
	}

	return nil
}
