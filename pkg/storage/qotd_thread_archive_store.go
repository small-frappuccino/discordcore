package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Store) GetCurrentAndPreviousQOTDPosts(ctx context.Context, guildID string, now time.Time) (_ []QOTDOfficialPostRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list current and previous qotd posts: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
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
			nonce,
			publish_ordinal,
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
		  AND published_at IS NOT NULL
		  AND archived_at IS NULL
		  AND archive_at > ?
		ORDER BY publish_date_utc DESC, published_at DESC, id DESC
		LIMIT 2`,
		guildID,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]QOTDOfficialPostRecord, 0, 2)
	for rows.Next() {
		record, err := scanQOTDOfficialPostRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) ListQOTDOfficialPostsNeedingArchive(ctx context.Context, now time.Time) (_ []QOTDOfficialPostRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list qotd official posts needing archive: %w", err)
		}
	}()
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
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
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at
		FROM qotd_official_posts
		WHERE published_at IS NOT NULL
		  AND archived_at IS NULL
		  AND archive_at <= ?
		ORDER BY archive_at ASC, id ASC`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]QOTDOfficialPostRecord, 0, 8)
	for rows.Next() {
		record, err := scanQOTDOfficialPostRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) UpdateQOTDOfficialPostState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (_ *QOTDOfficialPostRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update qotd official post state: %w", err)
		}
	}()
	if id <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return nil, fmt.Errorf("state is required")
	}

	row := s.queryRowContext(ctx,
		`UPDATE qotd_official_posts
		SET
			state = $1,
			closed_at = $2,
			archived_at = $3,
			last_reconciled_at = NOW(),
			updated_at = NOW()
		WHERE id = $4
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
			nonce,
			publish_ordinal,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at`,
		state,
		nullableTime(closedAt),
		nullableTime(archivedAt),
		id,
	)
	record, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		return nil, err
	}
	return record, nil
}
