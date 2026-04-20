package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *Store) CreateQOTDOfficialPostProvisioning(ctx context.Context, rec QOTDOfficialPostRecord) (*QOTDOfficialPostRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	normalized, err := normalizeQOTDOfficialPostRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("create qotd official post: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if normalized.State == "" {
		normalized.State = "provisioning"
	}

	row := s.queryRowContext(ctx,
		`INSERT INTO qotd_official_posts (
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			publish_date_utc,
			state,
			forum_channel_id,
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
			last_reconciled_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			publish_date_utc,
			state,
			forum_channel_id,
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
		normalized.GuildID,
		normalized.DeckID,
		normalized.DeckNameSnapshot,
		normalized.QuestionID,
		normalized.PublishMode,
		normalized.PublishDateUTC,
		normalized.State,
		normalized.ForumChannelID,
		normalized.QuestionListThreadID,
		normalized.QuestionListEntryMessageID,
		zeroEmptyString(normalized.DiscordThreadID),
		zeroEmptyString(normalized.DiscordStarterMessageID),
		normalized.AnswerChannelID,
		normalized.QuestionTextSnapshot,
		nullableTime(normalized.PublishedAt),
		normalized.GraceUntil.UTC(),
		normalized.ArchiveAt.UTC(),
		nullableTime(normalized.ClosedAt),
		nullableTime(normalized.ArchivedAt),
		nullableTime(normalized.LastReconciledAt),
	)
	created, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("create qotd official post: %w", err)
	}
	return created, nil
}

func (s *Store) FinalizeQOTDOfficialPost(ctx context.Context, id int64, questionListThreadID, questionListEntryMessageID, discordThreadID, starterMessageID, answerChannelID string, publishedAt time.Time) (*QOTDOfficialPostRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("finalize qotd official post: id is required")
	}
	questionListThreadID = strings.TrimSpace(questionListThreadID)
	questionListEntryMessageID = strings.TrimSpace(questionListEntryMessageID)
	discordThreadID = strings.TrimSpace(discordThreadID)
	starterMessageID = strings.TrimSpace(starterMessageID)
	answerChannelID = strings.TrimSpace(answerChannelID)
	if questionListThreadID == "" {
		return nil, fmt.Errorf("finalize qotd official post: question list thread id is required")
	}
	if questionListEntryMessageID == "" {
		return nil, fmt.Errorf("finalize qotd official post: question list entry message id is required")
	}
	if starterMessageID == "" {
		return nil, fmt.Errorf("finalize qotd official post: starter message id is required")
	}
	if answerChannelID == "" {
		return nil, fmt.Errorf("finalize qotd official post: answer channel id is required")
	}
	if publishedAt.IsZero() {
		return nil, fmt.Errorf("finalize qotd official post: published_at is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := s.queryRowContext(ctx,
		`UPDATE qotd_official_posts
		SET
			question_list_thread_id = ?,
			question_list_entry_message_id = ?,
			discord_thread_id = ?,
			discord_starter_message_id = ?,
			answer_channel_id = ?,
			published_at = ?,
			updated_at = NOW()
		WHERE id = ?
		RETURNING
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			publish_date_utc,
			state,
			forum_channel_id,
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
		questionListThreadID,
		questionListEntryMessageID,
		zeroEmptyString(discordThreadID),
		starterMessageID,
		answerChannelID,
		publishedAt.UTC(),
		id,
	)
	updated, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("finalize qotd official post: %w", err)
	}
	return updated, nil
}

func (s *Store) GetQOTDOfficialPostByID(ctx context.Context, id int64) (*QOTDOfficialPostRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	row := s.queryRowContext(ctx,
		`SELECT
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			publish_date_utc,
			state,
			forum_channel_id,
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
		WHERE id = ?`,
		id,
	)
	record, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get qotd official post by id: %w", err)
	}
	return record, nil
}

func (s *Store) GetQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (*QOTDOfficialPostRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	row := s.queryRowContext(ctx,
		`SELECT
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			publish_date_utc,
			state,
			forum_channel_id,
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
		  AND publish_mode = 'scheduled'
		  AND publish_date_utc = ?`,
		guildID,
		publishDateUTC,
	)
	record, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get qotd official post by date: %w", err)
	}
	return record, nil
}

func (s *Store) GetCurrentAndPreviousQOTDPosts(ctx context.Context, guildID string, now time.Time) ([]QOTDOfficialPostRecord, error) {
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
			publish_date_utc,
			state,
			forum_channel_id,
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
		  AND publish_mode = 'scheduled'
		  AND published_at IS NOT NULL
		  AND archived_at IS NULL
		  AND archive_at > ?
		ORDER BY publish_date_utc DESC
		LIMIT 2`,
		guildID,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("list current and previous qotd posts: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDOfficialPostRecord, 0, 2)
	for rows.Next() {
		record, err := scanQOTDOfficialPostRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list current and previous qotd posts: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list current and previous qotd posts: %w", err)
	}
	return records, nil
}

func (s *Store) ListQOTDOfficialPostsNeedingArchive(ctx context.Context, now time.Time) ([]QOTDOfficialPostRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
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
			publish_date_utc,
			state,
			forum_channel_id,
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
		WHERE published_at IS NOT NULL
		  AND archived_at IS NULL
		  AND archive_at <= ?
		ORDER BY archive_at ASC, id ASC`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("list qotd official posts needing archive: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDOfficialPostRecord, 0, 8)
	for rows.Next() {
		record, err := scanQOTDOfficialPostRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list qotd official posts needing archive: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list qotd official posts needing archive: %w", err)
	}
	return records, nil
}

func (s *Store) UpdateQOTDOfficialPostState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (*QOTDOfficialPostRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("update qotd official post state: id is required")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return nil, fmt.Errorf("update qotd official post state: state is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := s.queryRowContext(ctx,
		`UPDATE qotd_official_posts
		SET
			state = ?,
			closed_at = ?,
			archived_at = ?,
			last_reconciled_at = NOW(),
			updated_at = NOW()
		WHERE id = ?
		RETURNING
			id,
			guild_id,
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			publish_date_utc,
			state,
			forum_channel_id,
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
		state,
		nullableTime(closedAt),
		nullableTime(archivedAt),
		id,
	)
	record, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("update qotd official post state: %w", err)
	}
	return record, nil
}
