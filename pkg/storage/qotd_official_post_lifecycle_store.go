package storage

import (
	"context"
	"fmt"
	"strings"
)

func (s *Store) UpdateQOTDOfficialPostProgress(ctx context.Context, id int64, progress QOTDOfficialPostRecord) (*QOTDOfficialPostRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("update qotd official post progress: id is required")
	}
	progress.QuestionListThreadID = strings.TrimSpace(progress.QuestionListThreadID)
	progress.QuestionListEntryMessageID = strings.TrimSpace(progress.QuestionListEntryMessageID)
	progress.DiscordThreadID = strings.TrimSpace(progress.DiscordThreadID)
	progress.DiscordStarterMessageID = strings.TrimSpace(progress.DiscordStarterMessageID)
	progress.AnswerChannelID = strings.TrimSpace(progress.AnswerChannelID)
	progress.PublishedAt = normalizeQOTDTimePtr(progress.PublishedAt)
	if ctx == nil {
		ctx = context.Background()
	}

	row := s.queryRowContext(ctx,
		`UPDATE qotd_official_posts
		SET
			question_list_thread_id = COALESCE(NULLIF(?, ''), question_list_thread_id),
			question_list_entry_message_id = COALESCE(NULLIF(?, ''), question_list_entry_message_id),
			discord_thread_id = COALESCE(NULLIF(?, ''), discord_thread_id),
			discord_starter_message_id = COALESCE(NULLIF(?, ''), discord_starter_message_id),
			answer_channel_id = COALESCE(NULLIF(?, ''), answer_channel_id),
			published_at = COALESCE(?, published_at),
			updated_at = NOW()
		WHERE id = ?
		RETURNING
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at`,
		progress.QuestionListThreadID,
		progress.QuestionListEntryMessageID,
		zeroEmptyString(progress.DiscordThreadID),
		zeroEmptyString(progress.DiscordStarterMessageID),
		progress.AnswerChannelID,
		nullableTime(progress.PublishedAt),
		id,
	)
	record, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("update qotd official post progress: %w", err)
	}
	return record, nil
}

func (s *Store) ListQOTDOfficialPostsPendingRecovery(ctx context.Context, guildID string) ([]QOTDOfficialPostRecord, error) {
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

	rows, err := s.queryContext(ctx,
		`SELECT
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			consume_automatic_slot,
			publish_date_utc,
			state,
			channel_id,
			question_list_thread_id,
			question_list_entry_message_id,
			discord_thread_id,
			discord_starter_message_id,
			answer_channel_id,
			question_text_snapshot,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at
		FROM qotd_official_posts
		WHERE guild_id = ?
		  AND archived_at IS NULL
		  AND state IN ('provisioning', 'failed')
		ORDER BY updated_at ASC, id ASC`,
		guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("list qotd official posts pending recovery: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDOfficialPostRecord, 0, 8)
	for rows.Next() {
		record, err := scanQOTDOfficialPostRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list qotd official posts pending recovery: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list qotd official posts pending recovery: %w", err)
	}
	return records, nil
}
