package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type qotdRowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) CreateQOTDQuestion(ctx context.Context, rec QOTDQuestionRecord) (*QOTDQuestionRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	normalized, err := normalizeQOTDQuestionRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("create qotd question: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	position := normalized.QueuePosition
	if position < 1 {
		position = 0
	}

	row := s.queryRowContext(ctx,
		`INSERT INTO qotd_questions (
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at
		)
		VALUES (
			?,
			?,
			?,
			?,
			CASE
				WHEN ? > 0 THEN ?
				ELSE COALESCE((SELECT MAX(queue_position) + 1 FROM qotd_questions WHERE guild_id = ? AND deck_id = ?), 1)
			END,
			?,
			?,
			?
		)
		RETURNING
			id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			created_at,
			updated_at`,
		normalized.GuildID,
		normalized.DeckID,
		normalized.Body,
		normalized.Status,
		position,
		position,
		normalized.GuildID,
		normalized.DeckID,
		normalized.CreatedBy,
		nullableTime(normalized.ScheduledForDateUTC),
		nullableTime(normalized.UsedAt),
	)
	created, err := scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("create qotd question: %w", err)
	}
	return created, nil
}

func (s *Store) UpdateQOTDQuestion(ctx context.Context, rec QOTDQuestionRecord) (*QOTDQuestionRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if rec.ID <= 0 {
		return nil, fmt.Errorf("update qotd question: id is required")
	}
	normalized, err := normalizeQOTDQuestionRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("update qotd question: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	position := normalized.QueuePosition
	if position < 1 {
		position = 0
	}

	row := s.queryRowContext(ctx,
		`UPDATE qotd_questions
		SET
			deck_id = ?,
			body = ?,
			status = ?,
			queue_position = CASE WHEN ? > 0 THEN ? ELSE queue_position END,
			created_by = ?,
			scheduled_for_date_utc = ?,
			used_at = ?,
			updated_at = NOW()
		WHERE id = ? AND guild_id = ?
		RETURNING
			id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			created_at,
			updated_at`,
		normalized.DeckID,
		normalized.Body,
		normalized.Status,
		position,
		position,
		normalized.CreatedBy,
		nullableTime(normalized.ScheduledForDateUTC),
		nullableTime(normalized.UsedAt),
		normalized.ID,
		normalized.GuildID,
	)
	updated, err := scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("update qotd question: %w", err)
	}
	return updated, nil
}

func (s *Store) DeleteQOTDQuestion(ctx context.Context, guildID string, questionID int64) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || questionID <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := s.execContext(ctx, `DELETE FROM qotd_questions WHERE guild_id = ? AND id = ?`, guildID, questionID)
	if err != nil {
		return fmt.Errorf("delete qotd question: %w", err)
	}
	return nil
}

func (s *Store) DeleteQOTDQuestionsByDecks(ctx context.Context, guildID string, deckIDs []string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil
	}
	normalizedDeckIDs := normalizeQOTDDeckIDs(deckIDs)
	if len(normalizedDeckIDs) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete qotd questions by decks: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, deckID := range normalizedDeckIDs {
		if _, err := txExecContext(ctx, tx,
			`DELETE FROM qotd_questions WHERE guild_id = ? AND deck_id = ?`,
			guildID,
			deckID,
		); err != nil {
			return fmt.Errorf("delete qotd questions by decks: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete qotd questions by decks: %w", err)
	}
	return nil
}

func (s *Store) ListQOTDQuestions(ctx context.Context, guildID, deckID string) ([]QOTDQuestionRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
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
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = ?
		  AND (? = '' OR deck_id = ?)
		ORDER BY queue_position ASC, id ASC`,
		guildID,
		deckID,
		deckID,
	)
	if err != nil {
		return nil, fmt.Errorf("list qotd questions: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDQuestionRecord, 0, 16)
	for rows.Next() {
		record, err := scanQOTDQuestionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list qotd questions: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list qotd questions: %w", err)
	}
	return records, nil
}

func (s *Store) GetQOTDQuestion(ctx context.Context, guildID string, questionID int64) (*QOTDQuestionRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || questionID <= 0 {
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
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = ? AND id = ?`,
		guildID,
		questionID,
	)
	record, err := scanQOTDQuestionRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get qotd question: %w", err)
	}
	return record, nil
}

func (s *Store) ReorderQOTDQuestions(ctx context.Context, guildID, deckID string, orderedIDs []int64) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return fmt.Errorf("reorder qotd questions: guild_id is required")
	}
	if deckID == "" {
		return fmt.Errorf("reorder qotd questions: deck_id is required")
	}
	normalizedIDs, err := normalizeQOTDOrderedIDs(orderedIDs)
	if err != nil {
		return fmt.Errorf("reorder qotd questions: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reorder qotd questions: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := txQueryContext(ctx, tx,
		`SELECT id
		FROM qotd_questions
		WHERE guild_id = ?
		  AND deck_id = ?
		ORDER BY queue_position ASC, id ASC
		FOR UPDATE`,
		guildID,
		deckID,
	)
	if err != nil {
		return fmt.Errorf("reorder qotd questions: %w", err)
	}
	defer rows.Close()

	currentIDs := make([]int64, 0, len(normalizedIDs))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("reorder qotd questions: %w", err)
		}
		currentIDs = append(currentIDs, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reorder qotd questions: %w", err)
	}
	if !sameQOTDIDSet(currentIDs, normalizedIDs) {
		return fmt.Errorf("reorder qotd questions: ordered ids must match the full guild question set")
	}

	for idx, id := range normalizedIDs {
		if _, err := txExecContext(ctx, tx,
			`UPDATE qotd_questions SET queue_position = ?, updated_at = NOW() WHERE guild_id = ? AND deck_id = ? AND id = ?`,
			idx+1,
			guildID,
			deckID,
			id,
		); err != nil {
			return fmt.Errorf("reorder qotd questions: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("reorder qotd questions: %w", err)
	}
	return nil
}

func (s *Store) ReserveNextQOTDQuestion(ctx context.Context, guildID, deckID string, publishDateUTC time.Time) (*QOTDQuestionRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, fmt.Errorf("reserve qotd question: guild_id is required")
	}
	if deckID == "" {
		return nil, fmt.Errorf("reserve qotd question: deck_id is required")
	}
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if publishDateUTC.IsZero() {
		return nil, fmt.Errorf("reserve qotd question: publish_date_utc is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reserve qotd question: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := txQueryRow(tx,
		`SELECT
			id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = ?
		  AND deck_id = ?
		  AND status = 'ready'
		  AND scheduled_for_date_utc IS NULL
		ORDER BY queue_position ASC, id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`,
		guildID,
		deckID,
	)
	record, err := scanQOTDQuestionRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("reserve qotd question: %w", err)
	}

	row = txQueryRow(tx,
		`UPDATE qotd_questions
		SET
			status = 'reserved',
			scheduled_for_date_utc = ?,
			updated_at = NOW()
		WHERE id = ?
		RETURNING
			id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			created_at,
			updated_at`,
		publishDateUTC,
		record.ID,
	)
	record, err = scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("reserve qotd question: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("reserve qotd question: %w", err)
	}
	return record, nil
}

func (s *Store) ReserveNextReadyQOTDQuestion(ctx context.Context, guildID, deckID string) (*QOTDQuestionRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, fmt.Errorf("reserve ready qotd question: guild_id is required")
	}
	if deckID == "" {
		return nil, fmt.Errorf("reserve ready qotd question: deck_id is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reserve ready qotd question: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := txQueryRow(tx,
		`SELECT
			id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = ?
		  AND deck_id = ?
		  AND status = 'ready'
		  AND scheduled_for_date_utc IS NULL
		ORDER BY queue_position ASC, id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`,
		guildID,
		deckID,
	)
	record, err := scanQOTDQuestionRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("reserve ready qotd question: %w", err)
	}

	row = txQueryRow(tx,
		`UPDATE qotd_questions
		SET
			status = 'reserved',
			updated_at = NOW()
		WHERE id = ?
		RETURNING
			id,
			guild_id,
			deck_id,
			body,
			status,
			queue_position,
			created_by,
			scheduled_for_date_utc,
			used_at,
			created_at,
			updated_at`,
		record.ID,
	)
	record, err = scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("reserve ready qotd question: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("reserve ready qotd question: %w", err)
	}
	return record, nil
}

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
			response_channel_id_snapshot,
			discord_thread_id,
			discord_starter_message_id,
			question_text_snapshot,
			is_pinned,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			response_channel_id_snapshot,
			discord_thread_id,
			discord_starter_message_id,
			question_text_snapshot,
			is_pinned,
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
		normalized.ResponseChannelID,
		zeroEmptyString(normalized.DiscordThreadID),
		zeroEmptyString(normalized.DiscordStarterMessageID),
		normalized.QuestionTextSnapshot,
		normalized.IsPinned,
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

func (s *Store) FinalizeQOTDOfficialPost(ctx context.Context, id int64, discordThreadID, starterMessageID string, publishedAt time.Time) (*QOTDOfficialPostRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("finalize qotd official post: id is required")
	}
	discordThreadID = strings.TrimSpace(discordThreadID)
	starterMessageID = strings.TrimSpace(starterMessageID)
	if starterMessageID == "" {
		return nil, fmt.Errorf("finalize qotd official post: starter message id is required")
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
			discord_thread_id = ?,
			discord_starter_message_id = ?,
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
			response_channel_id_snapshot,
			discord_thread_id,
			discord_starter_message_id,
			question_text_snapshot,
			is_pinned,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at`,
		zeroEmptyString(discordThreadID),
		starterMessageID,
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
			response_channel_id_snapshot,
			discord_thread_id,
			discord_starter_message_id,
			question_text_snapshot,
			is_pinned,
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

func (s *Store) DeleteQOTDOfficialPost(ctx context.Context, id int64) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := s.execContext(ctx, `DELETE FROM qotd_official_posts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete qotd official post: %w", err)
	}
	return nil
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
			response_channel_id_snapshot,
			discord_thread_id,
			discord_starter_message_id,
			question_text_snapshot,
			is_pinned,
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

func (s *Store) GetQOTDOfficialPostByThreadID(ctx context.Context, discordThreadID string) (*QOTDOfficialPostRecord, error) {
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
			deck_id,
			deck_name_snapshot,
			question_id,
			publish_mode,
			publish_date_utc,
			state,
			forum_channel_id,
			response_channel_id_snapshot,
			discord_thread_id,
			discord_starter_message_id,
			question_text_snapshot,
			is_pinned,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at
		FROM qotd_official_posts
		WHERE discord_thread_id = ?`,
		discordThreadID,
	)
	record, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get qotd official post by thread: %w", err)
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
			response_channel_id_snapshot,
			discord_thread_id,
			discord_starter_message_id,
			question_text_snapshot,
			is_pinned,
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
			response_channel_id_snapshot,
			discord_thread_id,
			discord_starter_message_id,
			question_text_snapshot,
			is_pinned,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at
		FROM qotd_official_posts
		WHERE archived_at IS NULL
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

func (s *Store) UpdateQOTDOfficialPostState(ctx context.Context, id int64, state string, pinned bool, closedAt, archivedAt *time.Time) (*QOTDOfficialPostRecord, error) {
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
			is_pinned = ?,
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
			response_channel_id_snapshot,
			discord_thread_id,
			discord_starter_message_id,
			question_text_snapshot,
			is_pinned,
			published_at,
			grace_until,
			archive_at,
			closed_at,
			archived_at,
			last_reconciled_at,
			created_at,
			updated_at`,
		state,
		pinned,
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

func (s *Store) CreateQOTDReplyThreadProvisioning(ctx context.Context, rec QOTDReplyThreadRecord) (*QOTDReplyThreadRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	normalized, err := normalizeQOTDReplyThreadRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("create qotd reply thread: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if normalized.State == "" {
		normalized.State = "provisioning"
	}

	row := s.queryRowContext(ctx,
		`INSERT INTO qotd_reply_threads (
			guild_id,
			official_post_id,
			user_id,
			state,
			forum_channel_id,
			discord_thread_id,
			discord_starter_message_id,
			created_via_interaction_id,
			provisioning_nonce,
			closed_at,
			archived_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			forum_channel_id,
			discord_thread_id,
			discord_starter_message_id,
			created_via_interaction_id,
			provisioning_nonce,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		normalized.GuildID,
		normalized.OfficialPostID,
		normalized.UserID,
		normalized.State,
		normalized.ForumChannelID,
		zeroEmptyString(normalized.DiscordThreadID),
		zeroEmptyString(normalized.DiscordStarterMessageID),
		zeroEmptyString(normalized.CreatedViaInteractionID),
		zeroEmptyString(normalized.ProvisioningNonce),
		nullableTime(normalized.ClosedAt),
		nullableTime(normalized.ArchivedAt),
	)
	created, err := scanQOTDReplyThreadRecord(row)
	if err != nil {
		return nil, fmt.Errorf("create qotd reply thread: %w", err)
	}
	return created, nil
}

func (s *Store) FinalizeQOTDReplyThread(ctx context.Context, id int64, discordThreadID, starterMessageID string) (*QOTDReplyThreadRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("finalize qotd reply thread: id is required")
	}
	discordThreadID = strings.TrimSpace(discordThreadID)
	starterMessageID = strings.TrimSpace(starterMessageID)
	if starterMessageID == "" {
		return nil, fmt.Errorf("finalize qotd reply thread: starter message id is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := s.queryRowContext(ctx,
		`UPDATE qotd_reply_threads
		SET
			discord_thread_id = ?,
			discord_starter_message_id = ?,
			updated_at = NOW()
		WHERE id = ?
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			forum_channel_id,
			discord_thread_id,
			discord_starter_message_id,
			created_via_interaction_id,
			provisioning_nonce,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		zeroEmptyString(discordThreadID),
		starterMessageID,
		id,
	)
	record, err := scanQOTDReplyThreadRecord(row)
	if err != nil {
		return nil, fmt.Errorf("finalize qotd reply thread: %w", err)
	}
	return record, nil
}

func (s *Store) GetQOTDReplyThreadByOfficialPostAndUser(ctx context.Context, officialPostID int64, userID string) (*QOTDReplyThreadRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	userID = strings.TrimSpace(userID)
	if officialPostID <= 0 || userID == "" {
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
			user_id,
			state,
			forum_channel_id,
			discord_thread_id,
			discord_starter_message_id,
			created_via_interaction_id,
			provisioning_nonce,
			created_at,
			updated_at,
			closed_at,
			archived_at
		FROM qotd_reply_threads
		WHERE official_post_id = ? AND user_id = ?`,
		officialPostID,
		userID,
	)
	record, err := scanQOTDReplyThreadRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get qotd reply thread by official post and user: %w", err)
	}
	return record, nil
}

func (s *Store) ListQOTDReplyThreadsByOfficialPost(ctx context.Context, officialPostID int64) ([]QOTDReplyThreadRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if officialPostID <= 0 {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rows, err := s.queryContext(ctx,
		`SELECT
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			forum_channel_id,
			discord_thread_id,
			discord_starter_message_id,
			created_via_interaction_id,
			provisioning_nonce,
			created_at,
			updated_at,
			closed_at,
			archived_at
		FROM qotd_reply_threads
		WHERE official_post_id = ?
		ORDER BY id ASC`,
		officialPostID,
	)
	if err != nil {
		return nil, fmt.Errorf("list qotd reply threads by official post: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDReplyThreadRecord, 0, 16)
	for rows.Next() {
		record, err := scanQOTDReplyThreadRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list qotd reply threads by official post: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list qotd reply threads by official post: %w", err)
	}
	return records, nil
}

func (s *Store) UpdateQOTDReplyThreadState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (*QOTDReplyThreadRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("update qotd reply thread state: id is required")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return nil, fmt.Errorf("update qotd reply thread state: state is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	row := s.queryRowContext(ctx,
		`UPDATE qotd_reply_threads
		SET
			state = ?,
			closed_at = ?,
			archived_at = ?,
			updated_at = NOW()
		WHERE id = ?
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			forum_channel_id,
			discord_thread_id,
			discord_starter_message_id,
			created_via_interaction_id,
			provisioning_nonce,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		state,
		nullableTime(closedAt),
		nullableTime(archivedAt),
		id,
	)
	record, err := scanQOTDReplyThreadRecord(row)
	if err != nil {
		return nil, fmt.Errorf("update qotd reply thread state: %w", err)
	}
	return record, nil
}

func (s *Store) PrepareQOTDReplyThreadProvisioning(ctx context.Context, id int64, forumChannelID, interactionID, provisioningNonce string) (*QOTDReplyThreadRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("prepare qotd reply thread provisioning: id is required")
	}
	forumChannelID = strings.TrimSpace(forumChannelID)
	interactionID = strings.TrimSpace(interactionID)
	provisioningNonce = strings.TrimSpace(provisioningNonce)
	if forumChannelID == "" {
		return nil, fmt.Errorf("prepare qotd reply thread provisioning: forum_channel_id is required")
	}
	if provisioningNonce == "" {
		return nil, fmt.Errorf("prepare qotd reply thread provisioning: provisioning_nonce is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := s.queryRowContext(ctx,
		`UPDATE qotd_reply_threads
		SET
			state = 'provisioning',
			forum_channel_id = ?,
			discord_thread_id = NULL,
			discord_starter_message_id = NULL,
			created_via_interaction_id = ?,
			provisioning_nonce = ?,
			closed_at = NULL,
			archived_at = NULL,
			updated_at = NOW()
		WHERE id = ?
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			forum_channel_id,
			discord_thread_id,
			discord_starter_message_id,
			created_via_interaction_id,
			provisioning_nonce,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		forumChannelID,
		zeroEmptyString(interactionID),
		provisioningNonce,
		id,
	)
	record, err := scanQOTDReplyThreadRecord(row)
	if err != nil {
		return nil, fmt.Errorf("prepare qotd reply thread provisioning: %w", err)
	}
	return record, nil
}

func (s *Store) ListQOTDReplyThreadsPendingRecovery(ctx context.Context, guildID string) ([]QOTDReplyThreadRecord, error) {
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
			official_post_id,
			user_id,
			state,
			forum_channel_id,
			discord_thread_id,
			discord_starter_message_id,
			created_via_interaction_id,
			provisioning_nonce,
			created_at,
			updated_at,
			closed_at,
			archived_at
		FROM qotd_reply_threads
		WHERE guild_id = ?
		  AND state = 'provisioning'
		  AND discord_thread_id IS NULL
		ORDER BY updated_at ASC, id ASC`,
		guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("list qotd reply threads pending recovery: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDReplyThreadRecord, 0, 8)
	for rows.Next() {
		record, err := scanQOTDReplyThreadRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list qotd reply threads pending recovery: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list qotd reply threads pending recovery: %w", err)
	}
	return records, nil
}

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
			reply_thread_id,
			source_kind,
			discord_thread_id,
			archived_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING
			id,
			guild_id,
			official_post_id,
			reply_thread_id,
			source_kind,
			discord_thread_id,
			archived_at,
			created_at`,
		normalized.GuildID,
		normalized.OfficialPostID,
		nullableInt64(normalized.ReplyThreadID),
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
			reply_thread_id,
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

func normalizeQOTDQuestionRecord(rec QOTDQuestionRecord) (QOTDQuestionRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.Body = strings.TrimSpace(rec.Body)
	rec.Status = strings.TrimSpace(rec.Status)
	rec.CreatedBy = strings.TrimSpace(rec.CreatedBy)
	rec.QueuePosition = maxInt64(rec.QueuePosition, 0)
	rec.ScheduledForDateUTC = normalizeQOTDDatePtr(rec.ScheduledForDateUTC)
	rec.UsedAt = normalizeQOTDTimePtr(rec.UsedAt)

	if rec.GuildID == "" {
		return QOTDQuestionRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.DeckID == "" {
		return QOTDQuestionRecord{}, fmt.Errorf("deck_id is required")
	}
	if rec.Body == "" {
		return QOTDQuestionRecord{}, fmt.Errorf("body is required")
	}
	if rec.Status == "" {
		rec.Status = "draft"
	}
	return rec, nil
}

func normalizeQOTDOfficialPostRecord(rec QOTDOfficialPostRecord) (QOTDOfficialPostRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.DeckNameSnapshot = strings.TrimSpace(rec.DeckNameSnapshot)
	rec.PublishMode = strings.TrimSpace(rec.PublishMode)
	rec.State = strings.TrimSpace(rec.State)
	rec.ForumChannelID = strings.TrimSpace(rec.ForumChannelID)
	rec.ResponseChannelID = strings.TrimSpace(rec.ResponseChannelID)
	rec.DiscordThreadID = strings.TrimSpace(rec.DiscordThreadID)
	rec.DiscordStarterMessageID = strings.TrimSpace(rec.DiscordStarterMessageID)
	rec.QuestionTextSnapshot = strings.TrimSpace(rec.QuestionTextSnapshot)
	rec.PublishDateUTC = normalizeQOTDDateUTC(rec.PublishDateUTC)
	rec.PublishedAt = normalizeQOTDTimePtr(rec.PublishedAt)
	rec.ClosedAt = normalizeQOTDTimePtr(rec.ClosedAt)
	rec.ArchivedAt = normalizeQOTDTimePtr(rec.ArchivedAt)
	rec.LastReconciledAt = normalizeQOTDTimePtr(rec.LastReconciledAt)
	rec.GraceUntil = normalizeQOTDRequiredTime(rec.GraceUntil)
	rec.ArchiveAt = normalizeQOTDRequiredTime(rec.ArchiveAt)

	if rec.PublishMode == "" {
		rec.PublishMode = "scheduled"
	}

	if rec.GuildID == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.DeckID == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("deck_id is required")
	}
	if rec.DeckNameSnapshot == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("deck_name_snapshot is required")
	}
	if rec.QuestionID <= 0 {
		return QOTDOfficialPostRecord{}, fmt.Errorf("question_id is required")
	}
	if rec.PublishMode == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("publish_mode is required")
	}
	if rec.PublishDateUTC.IsZero() {
		return QOTDOfficialPostRecord{}, fmt.Errorf("publish_date_utc is required")
	}
	if rec.ForumChannelID == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("forum_channel_id is required")
	}
	if rec.ResponseChannelID == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("response_channel_id is required")
	}
	if rec.QuestionTextSnapshot == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("question_text_snapshot is required")
	}
	if rec.GraceUntil.IsZero() {
		return QOTDOfficialPostRecord{}, fmt.Errorf("grace_until is required")
	}
	if rec.ArchiveAt.IsZero() {
		return QOTDOfficialPostRecord{}, fmt.Errorf("archive_at is required")
	}
	return rec, nil
}

func normalizeQOTDReplyThreadRecord(rec QOTDReplyThreadRecord) (QOTDReplyThreadRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.UserID = strings.TrimSpace(rec.UserID)
	rec.State = strings.TrimSpace(rec.State)
	rec.ForumChannelID = strings.TrimSpace(rec.ForumChannelID)
	rec.DiscordThreadID = strings.TrimSpace(rec.DiscordThreadID)
	rec.DiscordStarterMessageID = strings.TrimSpace(rec.DiscordStarterMessageID)
	rec.CreatedViaInteractionID = strings.TrimSpace(rec.CreatedViaInteractionID)
	rec.ProvisioningNonce = strings.TrimSpace(rec.ProvisioningNonce)
	rec.ClosedAt = normalizeQOTDTimePtr(rec.ClosedAt)
	rec.ArchivedAt = normalizeQOTDTimePtr(rec.ArchivedAt)

	if rec.GuildID == "" {
		return QOTDReplyThreadRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.OfficialPostID <= 0 {
		return QOTDReplyThreadRecord{}, fmt.Errorf("official_post_id is required")
	}
	if rec.UserID == "" {
		return QOTDReplyThreadRecord{}, fmt.Errorf("user_id is required")
	}
	if rec.ForumChannelID == "" {
		return QOTDReplyThreadRecord{}, fmt.Errorf("forum_channel_id is required")
	}
	return rec, nil
}

func normalizeQOTDThreadArchiveRecord(rec QOTDThreadArchiveRecord) (QOTDThreadArchiveRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.SourceKind = strings.TrimSpace(rec.SourceKind)
	rec.DiscordThreadID = strings.TrimSpace(rec.DiscordThreadID)
	rec.ArchivedAt = normalizeQOTDRequiredTime(rec.ArchivedAt)
	if rec.ReplyThreadID != nil && *rec.ReplyThreadID <= 0 {
		rec.ReplyThreadID = nil
	}

	if rec.GuildID == "" {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.OfficialPostID <= 0 {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("official_post_id is required")
	}
	if rec.SourceKind == "" {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("source_kind is required")
	}
	if rec.DiscordThreadID == "" {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("discord_thread_id is required")
	}
	if rec.ArchivedAt.IsZero() {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("archived_at is required")
	}
	return rec, nil
}

func normalizeQOTDMessageArchives(threadArchiveID int64, msgs []QOTDMessageArchiveRecord) ([]QOTDMessageArchiveRecord, error) {
	if len(msgs) == 0 {
		return nil, nil
	}

	order := make([]string, 0, len(msgs))
	byMessage := make(map[string]QOTDMessageArchiveRecord, len(msgs))
	for _, msg := range msgs {
		msg.ThreadArchiveID = threadArchiveID
		msg.DiscordMessageID = strings.TrimSpace(msg.DiscordMessageID)
		msg.AuthorID = strings.TrimSpace(msg.AuthorID)
		msg.AuthorNameSnapshot = strings.TrimSpace(msg.AuthorNameSnapshot)
		msg.Content = strings.TrimSpace(msg.Content)
		msg.EmbedsJSON = cloneQOTDJSONRawMessage(msg.EmbedsJSON)
		msg.AttachmentsJSON = cloneQOTDJSONRawMessage(msg.AttachmentsJSON)
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = time.Now().UTC()
		} else {
			msg.CreatedAt = msg.CreatedAt.UTC()
		}

		if msg.DiscordMessageID == "" {
			return nil, fmt.Errorf("discord_message_id is required")
		}
		if _, ok := byMessage[msg.DiscordMessageID]; !ok {
			order = append(order, msg.DiscordMessageID)
		}
		byMessage[msg.DiscordMessageID] = msg
	}

	normalized := make([]QOTDMessageArchiveRecord, 0, len(order))
	for _, messageID := range order {
		normalized = append(normalized, byMessage[messageID])
	}
	return normalized, nil
}

func normalizeQOTDOrderedIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("ordered ids are required")
	}
	seen := make(map[int64]struct{}, len(ids))
	normalized := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("ordered ids must be positive")
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("ordered ids must be unique")
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func normalizeQOTDDeckIDs(deckIDs []string) []string {
	if len(deckIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(deckIDs))
	normalized := make([]string, 0, len(deckIDs))
	for _, deckID := range deckIDs {
		deckID = strings.TrimSpace(deckID)
		if deckID == "" {
			continue
		}
		if _, ok := seen[deckID]; ok {
			continue
		}
		seen[deckID] = struct{}{}
		normalized = append(normalized, deckID)
	}
	return normalized
}

func sameQOTDIDSet(current, ordered []int64) bool {
	if len(current) != len(ordered) {
		return false
	}
	left := append([]int64(nil), current...)
	right := append([]int64(nil), ordered...)
	sort.Slice(left, func(i, j int) bool { return left[i] < left[j] })
	sort.Slice(right, func(i, j int) bool { return right[i] < right[j] })
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func scanQOTDQuestionRecord(scanner qotdRowScanner) (*QOTDQuestionRecord, error) {
	var record QOTDQuestionRecord
	var scheduledFor sql.NullTime
	var usedAt sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.DeckID,
		&record.Body,
		&record.Status,
		&record.QueuePosition,
		&record.CreatedBy,
		&scheduledFor,
		&usedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.ScheduledForDateUTC = timePtrFromNull(scheduledFor)
	record.UsedAt = timePtrFromNull(usedAt)
	return &record, nil
}

func scanQOTDOfficialPostRecord(scanner qotdRowScanner) (*QOTDOfficialPostRecord, error) {
	var record QOTDOfficialPostRecord
	var threadID sql.NullString
	var starterMessageID sql.NullString
	var publishedAt sql.NullTime
	var closedAt sql.NullTime
	var archivedAt sql.NullTime
	var reconciledAt sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.DeckID,
		&record.DeckNameSnapshot,
		&record.QuestionID,
		&record.PublishMode,
		&record.PublishDateUTC,
		&record.State,
		&record.ForumChannelID,
		&record.ResponseChannelID,
		&threadID,
		&starterMessageID,
		&record.QuestionTextSnapshot,
		&record.IsPinned,
		&publishedAt,
		&record.GraceUntil,
		&record.ArchiveAt,
		&closedAt,
		&archivedAt,
		&reconciledAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.DiscordThreadID = threadID.String
	record.DiscordStarterMessageID = starterMessageID.String
	record.PublishedAt = timePtrFromNull(publishedAt)
	record.ClosedAt = timePtrFromNull(closedAt)
	record.ArchivedAt = timePtrFromNull(archivedAt)
	record.LastReconciledAt = timePtrFromNull(reconciledAt)
	record.PublishMode = strings.TrimSpace(record.PublishMode)
	record.PublishDateUTC = normalizeQOTDDateUTC(record.PublishDateUTC)
	record.GraceUntil = record.GraceUntil.UTC()
	record.ArchiveAt = record.ArchiveAt.UTC()
	return &record, nil
}

func scanQOTDReplyThreadRecord(scanner qotdRowScanner) (*QOTDReplyThreadRecord, error) {
	var record QOTDReplyThreadRecord
	var threadID sql.NullString
	var starterMessageID sql.NullString
	var interactionID sql.NullString
	var provisioningNonce sql.NullString
	var closedAt sql.NullTime
	var archivedAt sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.OfficialPostID,
		&record.UserID,
		&record.State,
		&record.ForumChannelID,
		&threadID,
		&starterMessageID,
		&interactionID,
		&provisioningNonce,
		&record.CreatedAt,
		&record.UpdatedAt,
		&closedAt,
		&archivedAt,
	); err != nil {
		return nil, err
	}
	record.DiscordThreadID = threadID.String
	record.DiscordStarterMessageID = starterMessageID.String
	record.CreatedViaInteractionID = interactionID.String
	record.ProvisioningNonce = provisioningNonce.String
	record.ClosedAt = timePtrFromNull(closedAt)
	record.ArchivedAt = timePtrFromNull(archivedAt)
	return &record, nil
}

func scanQOTDThreadArchiveRecord(scanner qotdRowScanner) (*QOTDThreadArchiveRecord, error) {
	var record QOTDThreadArchiveRecord
	var replyThreadID sql.NullInt64
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.OfficialPostID,
		&replyThreadID,
		&record.SourceKind,
		&record.DiscordThreadID,
		&record.ArchivedAt,
		&record.CreatedAt,
	); err != nil {
		return nil, err
	}
	record.ReplyThreadID = int64PtrFromNull(replyThreadID)
	record.ArchivedAt = record.ArchivedAt.UTC()
	return &record, nil
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullableInt64(value *int64) any {
	if value == nil || *value <= 0 {
		return nil
	}
	return *value
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func cloneQOTDJSONRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

func timePtrFromNull(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	normalized := value.Time.UTC()
	return &normalized
}

func int64PtrFromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	normalized := value.Int64
	return &normalized
}

func normalizeQOTDDateUTC(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func normalizeQOTDRequiredTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}

func normalizeQOTDDatePtr(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	normalized := normalizeQOTDDateUTC(*value)
	return &normalized
}

func normalizeQOTDTimePtr(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func zeroEmptyString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
