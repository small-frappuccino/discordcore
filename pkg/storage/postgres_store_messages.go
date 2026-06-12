package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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

	var expires any
	if m.HasExpiry {
		expires = m.ExpiresAt.UTC()
	} else {
		expires = nil
	}
	_, err := s.exec(
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
func (s *Store) UpsertMessagesContext(ctx context.Context, records []MessageRecord) error {

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

	row := s.queryRow(
		`SELECT guild_id, message_id, channel_id, author_id, author_username, author_avatar, content, cached_at, expires_at
         FROM messages
         WHERE guild_id=$1 AND message_id=$2 AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`,
		guildID, messageID,
	)

	var rec MessageRecord
	var expires *time.Time
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
		if err == pgx.ErrNoRows {
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

// DeleteMessage removes a message record (no error if absent).
func (s *Store) DeleteMessage(guildID, messageID string) error {
	_, err := s.exec(`DELETE FROM messages WHERE guild_id=$1 AND message_id=$2`, guildID, messageID)
	return err
}

// DeleteMessagesContext removes a batch of message records.
func (s *Store) DeleteMessagesContext(ctx context.Context, keys []MessageDeleteKey) error {

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
	return 0, nil
}

// CleanupObsoleteMemberRoles removes role records older than retentionDays
func (s *Store) CleanupObsoleteMemberRoles(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	result, err := s.exec(`DELETE FROM roles_current WHERE updated_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("Store.CleanupObsoleteMemberRoles: %w", err)
	}
	return result.RowsAffected(), nil
}

// CleanupObsoleteAvatars removes avatar records older than retentionDays
func (s *Store) CleanupObsoleteAvatars(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 180
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	result, err := s.exec(`DELETE FROM avatars_history WHERE changed_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("Store.CleanupObsoleteAvatars: %w", err)
	}
	return result.RowsAffected(), nil
}

// CleanupAllObsoleteData performs cleanup of all obsolete data with default retention periods
func (s *Store) CleanupAllObsoleteData() error {

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
