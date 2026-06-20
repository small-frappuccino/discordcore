package storage

import (
	"context"
	"errors"
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

// MessageVersion is one persisted revision of a Discord message.
type MessageVersion struct {
	GuildID     string
	MessageID   string
	ChannelID   string
	AuthorID    string
	Version     int
	EventType   string
	Content     string
	Attachments int
	Embeds      int
	Stickers    int
	CreatedAt   time.Time
}

// UpsertMessage inserts or updates a message record transactionally.
func (s *Store) UpsertMessage(m MessageRecord) error {
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

// GetMessage returns a non-expired message if present.
func (s *Store) GetMessage(ctx context.Context, guildID, messageID string) (*MessageRecord, error) {
	row := s.db.QueryRow(ctx,
		`SELECT guild_id, message_id, channel_id, author_id, author_username, author_avatar, content, cached_at, expires_at
         FROM messages
         WHERE guild_id=$1 AND message_id=$2 AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`,
		guildID, messageID,
	)

	var rec MessageRecord
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

// InsertMessageVersionsMixedBatchContext inserts a batch of message history rows.
func (s *Store) InsertMessageVersionsMixedBatchContext(ctx context.Context, versions []MessageVersion) (err error) {
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

func normalizeMessageVersions(versions []MessageVersion) []MessageVersion {
	if len(versions) == 0 {
		return nil
	}
	normalized := make([]MessageVersion, 0, len(versions))
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

func reserveMessageVersionRangesTx(ctx context.Context, tx pgx.Tx, versions []MessageVersion) ([]MessageVersion, error) {
	if len(versions) == 0 {
		return nil, nil
	}

	assigned := append([]MessageVersion(nil), versions...)
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

func groupMessageVersions(versions []MessageVersion) []messageVersionGroup {
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

func insertMessageHistoryBatchTx(ctx context.Context, tx pgx.Tx, versions []MessageVersion) error {
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

// DailyMessageCountDelta represents a delta for daily message counts.
type DailyMessageCountDelta struct {
	GuildID   string
	ChannelID string
	UserID    string
	Day       time.Time
	Count     int
}

// IncrementDailyMessageCountsContext increments the daily message counts for multiple guilds.
func (s *Store) IncrementDailyMessageCountsContext(ctx context.Context, deltas []DailyMessageCountDelta) error {
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
func (s *Store) InsertMessageVersion(ctx context.Context, v MessageVersion) error {
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
