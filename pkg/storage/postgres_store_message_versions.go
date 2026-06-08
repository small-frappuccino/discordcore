package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Message Versioning (history)

// MessageVersion is one persisted revision of a Discord message. Version is the
// monotonic per-message revision counter; EventType records what produced the
// revision (create, edit, delete). The attachment/embed/sticker fields store
// counts, not the payloads themselves.
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

// InsertMessageVersionsBatchContext inserts a batch of message history rows and keeps version counters in sync.
func (s *Store) InsertMessageVersionsBatchContext(ctx context.Context, versions []MessageVersion) error {
	return s.InsertMessageVersionsMixedBatchContext(ctx, versions)
}

// InsertMessageVersionsMixedBatchContext inserts a batch of message history rows, assigning versions for rows with Version <= 0.
func (s *Store) InsertMessageVersionsMixedBatchContext(ctx context.Context, versions []MessageVersion) (err error) {

	normalized := normalizeMessageVersions(versions)
	if len(normalized) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin message versions tx: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(); rerr != nil && !errors.Is(rerr, sql.ErrTxDone) {
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
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit message versions tx: %w", err)
	}
	return nil
}

func normalizeMessageVersions(versions []MessageVersion) []MessageVersion {
	if len(versions) == 0 {
		return nil
	}
	normalized := make([]MessageVersion, 0, len(versions))
	for _, version := range versions {
		if strings.TrimSpace(version.GuildID) == "" ||
			strings.TrimSpace(version.MessageID) == "" ||
			strings.TrimSpace(version.ChannelID) == "" ||
			strings.TrimSpace(version.AuthorID) == "" ||
			strings.TrimSpace(version.EventType) == "" {
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

// InsertMessageVersion inserts a new version row for a message.
func (s *Store) InsertMessageVersion(v MessageVersion) error {
	return s.InsertMessageVersionContext(context.Background(), v)
}

// InsertMessageVersionContext inserts a new version row for a message with context support.
func (s *Store) InsertMessageVersionContext(ctx context.Context, v MessageVersion) error {
	return s.InsertMessageVersionsMixedBatchContext(ctx, []MessageVersion{v})
}

type messageVersionGroup struct {
	GuildID   string
	MessageID string
	Indexes   []int
}

func reserveMessageVersionRangesTx(ctx context.Context, tx *sql.Tx, versions []MessageVersion) ([]MessageVersion, error) {
	if len(versions) == 0 {
		return nil, nil
	}

	assigned := append([]MessageVersion(nil), versions...)
	for _, group := range groupMessageVersions(assigned) {
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
			group = &messageVersionGroup{
				GuildID:   strings.TrimSpace(version.GuildID),
				MessageID: strings.TrimSpace(version.MessageID),
				Indexes:   make([]int, 0, 4),
			}
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

func lockMessageVersionCounterTx(ctx context.Context, tx *sql.Tx, guildID, messageID string) (int, error) {
	if guildID == "" || messageID == "" {
		return 0, nil
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO message_version_counters (guild_id, message_id, last_version)
         VALUES ($1, $2, COALESCE((SELECT MAX(version) FROM messages_history WHERE guild_id=$3 AND message_id=$4), 0))
         ON CONFLICT (guild_id, message_id) DO NOTHING`,
		guildID, messageID, guildID, messageID); err != nil {
		return 0, fmt.Errorf("ensure message version counter: %w", err)
	}

	var lastVersion int64
	if err := tx.QueryRowContext(ctx,
		`SELECT last_version FROM message_version_counters WHERE guild_id=$1 AND message_id=$2 FOR UPDATE`,
		guildID, messageID).Scan(&lastVersion); err != nil {
		return 0, fmt.Errorf("lock message version counter: %w", err)
	}
	return int(lastVersion), nil
}

func updateMessageVersionCounterTx(ctx context.Context, tx *sql.Tx, guildID, messageID string, lastVersion int) error {
	if guildID == "" || messageID == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE message_version_counters SET last_version=$1 WHERE guild_id=$2 AND message_id=$3`,
		lastVersion, guildID, messageID); err != nil {
		return fmt.Errorf("update message version counter: %w", err)
	}
	return nil
}

func insertMessageHistoryBatchTx(ctx context.Context, tx *sql.Tx, versions []MessageVersion) error {
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

	_, err := tx.ExecContext(ctx,
		`INSERT INTO messages_history
         (guild_id, message_id, channel_id, author_id, version, event_type, content, attachments, embeds_count, stickers, created_at)
         SELECT * FROM UNNEST($1::text[], $2::text[], $3::text[], $4::text[], $5::int[], $6::text[], $7::text[], $8::int[], $9::int[], $10::int[], $11::timestamptz[])
         ON CONFLICT(guild_id, message_id, version) DO NOTHING`,
		guildIDs, messageIDs, channelIDs, authorIDs, versionNums, eventTypes, contents, attachments, embedsCounts, stickers, createdAts,
	)
	return err
}
