package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s *Store) CreateQOTDThreadArchive(ctx context.Context, rec QOTDThreadArchiveRecord) (*QOTDThreadArchiveRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	normalized, err := normalizeQOTDThreadArchiveRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("create qotd thread archive: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	row := s.queryRowContext(ctx,
		`INSERT INTO qotd_thread_archives (
			guild_id,
			official_post_id,
			source_kind,
			discord_thread_id,
			archived_at
		)
		VALUES (?, ?, ?, ?, ?)
		RETURNING
			id,
			guild_id,
			official_post_id,
			source_kind,
			discord_thread_id,
			archived_at,
			created_at`,
		normalized.GuildID,
		normalized.OfficialPostID,
		normalized.SourceKind,
		normalized.DiscordThreadID,
		normalized.ArchivedAt.UTC(),
	)
	created, err := scanQOTDThreadArchiveRecord(row)
	if err != nil {
		return nil, fmt.Errorf("create qotd thread archive: %w", err)
	}
	return created, nil
}

func (s *Store) AppendQOTDArchivedMessages(ctx context.Context, threadArchiveID int64, msgs []QOTDMessageArchiveRecord) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if threadArchiveID <= 0 {
		return fmt.Errorf("append qotd archived messages: thread_archive_id is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	normalized, err := normalizeQOTDMessageArchives(threadArchiveID, msgs)
	if err != nil {
		return fmt.Errorf("append qotd archived messages: %w", err)
	}
	if len(normalized) == 0 {
		return nil
	}

	return execValuesContext(ctx, s.db,
		`INSERT INTO qotd_message_archives (
			thread_archive_id,
			discord_message_id,
			author_id,
			author_name_snapshot,
			author_is_bot,
			content,
			embeds_json,
			attachments_json,
			created_at
		) VALUES `,
		` ON CONFLICT(thread_archive_id, discord_message_id) DO NOTHING`,
		len(normalized),
		9,
		func(args []any, rowIndex int) []any {
			record := normalized[rowIndex]
			return append(args,
				record.ThreadArchiveID,
				record.DiscordMessageID,
				zeroEmptyString(record.AuthorID),
				record.AuthorNameSnapshot,
				record.AuthorIsBot,
				record.Content,
				nullableJSON(record.EmbedsJSON),
				nullableJSON(record.AttachmentsJSON),
				record.CreatedAt.UTC(),
			)
		},
	)
}

func (s *Store) HasQOTDArchiveForThread(ctx context.Context, discordThreadID string) (bool, error) {
	if s.db == nil {
		return false, fmt.Errorf("store not initialized")
	}
	discordThreadID = strings.TrimSpace(discordThreadID)
	if discordThreadID == "" {
		return false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var exists bool
	if err := s.queryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM qotd_thread_archives WHERE discord_thread_id = ?)`,
		discordThreadID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check qotd archive for thread: %w", err)
	}
	return exists, nil
}

func (s *Store) GetQOTDThreadArchiveByThreadID(ctx context.Context, discordThreadID string) (*QOTDThreadArchiveRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	discordThreadID = strings.TrimSpace(discordThreadID)
	if discordThreadID == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := s.queryRowContext(ctx,
		`SELECT
			id,
			guild_id,
			official_post_id,
			source_kind,
			discord_thread_id,
			archived_at,
			created_at
		FROM qotd_thread_archives
		WHERE discord_thread_id = ?`,
		discordThreadID,
	)
	record, err := scanQOTDThreadArchiveRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get qotd thread archive by thread: %w", err)
	}
	return record, nil
}
