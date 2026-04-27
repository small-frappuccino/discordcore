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
			display_id,
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
			COALESCE((SELECT MAX(display_id) + 1 FROM qotd_questions WHERE guild_id = ? AND deck_id = ?), 1),
			?,
			?,
			?
		)
		RETURNING
			id,
			display_id,
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("update qotd question: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentDeckID string
	var currentQueuePosition int64
	if err := txQueryRow(tx,
		`SELECT deck_id, queue_position FROM qotd_questions WHERE id = ? AND guild_id = ? FOR UPDATE`,
		normalized.ID,
		normalized.GuildID,
	).Scan(&currentDeckID, &currentQueuePosition); err != nil {
		return nil, fmt.Errorf("update qotd question: %w", err)
	}

	position := normalized.QueuePosition
	if position < 1 {
		position = 0
	}

	movingDeck := currentDeckID != normalized.DeckID
	row := txQueryRow(tx,
		`UPDATE qotd_questions
		SET
			deck_id = ?,
			body = ?,
			status = ?,
			queue_position = CASE
				WHEN ? > 0 THEN ?
				WHEN ? THEN COALESCE((SELECT MAX(queue_position) + 1 FROM qotd_questions WHERE guild_id = ? AND deck_id = ?), 1)
				ELSE queue_position
			END,
			display_id = CASE
				WHEN ? THEN COALESCE((SELECT MAX(display_id) + 1 FROM qotd_questions WHERE guild_id = ? AND deck_id = ?), 1)
				ELSE display_id
			END,
			created_by = ?,
			scheduled_for_date_utc = ?,
			used_at = ?,
			updated_at = NOW()
		WHERE id = ? AND guild_id = ?
		RETURNING
			id,
			display_id,
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
		movingDeck,
		normalized.GuildID,
		normalized.DeckID,
		movingDeck,
		normalized.GuildID,
		normalized.DeckID,
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

	needsReindex := movingDeck || (position > 0 && position != currentQueuePosition)
	if needsReindex {
		if movingDeck {
			if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, normalized.GuildID, currentDeckID); err != nil {
				return nil, fmt.Errorf("update qotd question: %w", err)
			}
		}
		if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, normalized.GuildID, normalized.DeckID); err != nil {
			return nil, fmt.Errorf("update qotd question: %w", err)
		}
		updated, err = getQOTDQuestionTx(ctx, tx, normalized.GuildID, normalized.ID)
		if err != nil {
			return nil, fmt.Errorf("update qotd question: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete qotd question: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var deckID string
	if err := txQueryRow(tx,
		`SELECT deck_id FROM qotd_questions WHERE guild_id = ? AND id = ? FOR UPDATE`,
		guildID,
		questionID,
	).Scan(&deckID); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("delete qotd question: %w", err)
	}

	if _, err := txExecContext(ctx, tx, `DELETE FROM qotd_questions WHERE guild_id = ? AND id = ?`, guildID, questionID); err != nil {
		return fmt.Errorf("delete qotd question: %w", err)
	}
	if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, guildID, deckID); err != nil {
		return fmt.Errorf("delete qotd question: %w", err)
	}
	if err := tx.Commit(); err != nil {
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

func (s *Store) CreateQOTDCollectedQuestions(ctx context.Context, records []QOTDCollectedQuestionRecord) (int, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	normalized, err := normalizeQOTDCollectedQuestionRecords(records)
	if err != nil {
		return 0, fmt.Errorf("create qotd collected questions: %w", err)
	}
	if len(normalized) == 0 {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("create qotd collected questions: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	created := 0
	for _, record := range normalized {
		result, err := txExecContext(ctx, tx,
			`INSERT INTO qotd_collected_questions (
				guild_id,
				source_channel_id,
				source_message_id,
				source_author_id,
				source_author_name_snapshot,
				source_created_at,
				embed_title,
				question_text
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (guild_id, source_message_id) DO NOTHING`,
			record.GuildID,
			record.SourceChannelID,
			record.SourceMessageID,
			zeroEmptyString(record.SourceAuthorID),
			zeroEmptyString(record.SourceAuthorNameSnapshot),
			record.SourceCreatedAt,
			record.EmbedTitle,
			record.QuestionText,
		)
		if err != nil {
			return 0, fmt.Errorf("create qotd collected questions: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("create qotd collected questions: rows affected: %w", err)
		}
		created += int(rowsAffected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("create qotd collected questions: %w", err)
	}
	return created, nil
}

func (s *Store) CountQOTDCollectedQuestions(ctx context.Context, guildID string) (int, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var total int
	if err := s.queryRowContext(
		ctx,
		`SELECT COUNT(*) FROM qotd_collected_questions WHERE guild_id = ?`,
		guildID,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("count qotd collected questions: %w", err)
	}
	return total, nil
}

func (s *Store) ListRecentQOTDCollectedQuestions(ctx context.Context, guildID string, limit int) ([]QOTDCollectedQuestionRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 25
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rows, err := s.queryContext(ctx,
		`SELECT
			id,
			guild_id,
			source_channel_id,
			source_message_id,
			source_author_id,
			source_author_name_snapshot,
			source_created_at,
			embed_title,
			question_text,
			created_at,
			updated_at
		FROM qotd_collected_questions
		WHERE guild_id = ?
		ORDER BY source_created_at DESC, id DESC
		LIMIT ?`,
		guildID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent qotd collected questions: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDCollectedQuestionRecord, 0, limit)
	for rows.Next() {
		record, err := scanQOTDCollectedQuestionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list recent qotd collected questions: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list recent qotd collected questions: %w", err)
	}
	return records, nil
}

func (s *Store) ListAllQOTDCollectedQuestions(ctx context.Context, guildID string) ([]QOTDCollectedQuestionRecord, error) {
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
			source_channel_id,
			source_message_id,
			source_author_id,
			source_author_name_snapshot,
			source_created_at,
			embed_title,
			question_text,
			created_at,
			updated_at
		FROM qotd_collected_questions
		WHERE guild_id = ?
		ORDER BY source_created_at ASC, id ASC`,
		guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("list all qotd collected questions: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDCollectedQuestionRecord, 0, 32)
	for rows.Next() {
		record, err := scanQOTDCollectedQuestionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list all qotd collected questions: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list all qotd collected questions: %w", err)
	}
	return records, nil
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
			display_id,
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
			display_id,
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
	if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, guildID, deckID); err != nil {
		return fmt.Errorf("reorder qotd questions: %w", err)
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
			display_id,
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
			display_id,
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
			display_id,
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
			display_id,
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
		normalized.GuildID,
		normalized.DeckID,
		normalized.DeckNameSnapshot,
		normalized.QuestionID,
		normalized.PublishMode,
		normalized.PublishDateUTC,
		normalized.State,
		normalized.ChannelID,
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

func normalizeQOTDQuestionRecord(rec QOTDQuestionRecord) (QOTDQuestionRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.Body = strings.TrimSpace(rec.Body)
	rec.Status = strings.TrimSpace(rec.Status)
	rec.CreatedBy = strings.TrimSpace(rec.CreatedBy)
	rec.DisplayID = maxInt64(rec.DisplayID, 0)
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
	rec.ChannelID = strings.TrimSpace(rec.ChannelID)
	rec.QuestionListThreadID = strings.TrimSpace(rec.QuestionListThreadID)
	rec.QuestionListEntryMessageID = strings.TrimSpace(rec.QuestionListEntryMessageID)
	rec.DiscordThreadID = strings.TrimSpace(rec.DiscordThreadID)
	rec.DiscordStarterMessageID = strings.TrimSpace(rec.DiscordStarterMessageID)
	rec.AnswerChannelID = strings.TrimSpace(rec.AnswerChannelID)
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
	if rec.ChannelID == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("channel_id is required")
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

func normalizeQOTDThreadArchiveRecord(rec QOTDThreadArchiveRecord) (QOTDThreadArchiveRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.SourceKind = strings.TrimSpace(rec.SourceKind)
	rec.DiscordThreadID = strings.TrimSpace(rec.DiscordThreadID)
	rec.ArchivedAt = normalizeQOTDRequiredTime(rec.ArchivedAt)

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

func normalizeQOTDCollectedQuestionRecords(records []QOTDCollectedQuestionRecord) ([]QOTDCollectedQuestionRecord, error) {
	if len(records) == 0 {
		return nil, nil
	}

	order := make([]string, 0, len(records))
	byMessage := make(map[string]QOTDCollectedQuestionRecord, len(records))
	for _, record := range records {
		record.GuildID = strings.TrimSpace(record.GuildID)
		record.SourceChannelID = strings.TrimSpace(record.SourceChannelID)
		record.SourceMessageID = strings.TrimSpace(record.SourceMessageID)
		record.SourceAuthorID = strings.TrimSpace(record.SourceAuthorID)
		record.SourceAuthorNameSnapshot = strings.TrimSpace(record.SourceAuthorNameSnapshot)
		record.EmbedTitle = strings.TrimSpace(record.EmbedTitle)
		record.QuestionText = strings.Join(strings.Fields(strings.TrimSpace(record.QuestionText)), " ")
		record.SourceCreatedAt = normalizeQOTDRequiredTime(record.SourceCreatedAt)

		switch {
		case record.GuildID == "":
			return nil, fmt.Errorf("guild_id is required")
		case record.SourceChannelID == "":
			return nil, fmt.Errorf("source_channel_id is required")
		case record.SourceMessageID == "":
			return nil, fmt.Errorf("source_message_id is required")
		case record.SourceCreatedAt.IsZero():
			return nil, fmt.Errorf("source_created_at is required")
		case record.QuestionText == "":
			return nil, fmt.Errorf("question_text is required")
		}

		key := record.GuildID + "\x00" + record.SourceMessageID
		if _, exists := byMessage[key]; !exists {
			order = append(order, key)
		}
		byMessage[key] = record
	}

	normalized := make([]QOTDCollectedQuestionRecord, 0, len(order))
	for _, key := range order {
		normalized = append(normalized, byMessage[key])
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
		&record.DisplayID,
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

func getQOTDQuestionTx(ctx context.Context, tx *sql.Tx, guildID string, questionID int64) (*QOTDQuestionRecord, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := txQueryRow(tx,
		`SELECT
			id,
			display_id,
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
		return nil, err
	}
	return record, nil
}

func reindexQOTDQuestionDisplayIDsTx(ctx context.Context, tx *sql.Tx, guildID, deckID string) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return nil
	}

	if _, err := txExecContext(ctx, tx,
		`UPDATE qotd_questions
		SET display_id = -id
		WHERE guild_id = ? AND deck_id = ?`,
		guildID,
		deckID,
	); err != nil {
		return err
	}

	if _, err := txExecContext(ctx, tx,
		`WITH ordered AS (
			SELECT
				id,
				ROW_NUMBER() OVER (ORDER BY queue_position ASC, id ASC)::BIGINT AS next_display_id
			FROM qotd_questions
			WHERE guild_id = ?
			  AND deck_id = ?
		)
		UPDATE qotd_questions AS questions
		SET display_id = ordered.next_display_id
		FROM ordered
		WHERE questions.id = ordered.id`,
		guildID,
		deckID,
	); err != nil {
		return err
	}
	return nil
}

func scanQOTDOfficialPostRecord(scanner qotdRowScanner) (*QOTDOfficialPostRecord, error) {
	var record QOTDOfficialPostRecord
	var questionListThreadID sql.NullString
	var questionListEntryMessageID sql.NullString
	var threadID sql.NullString
	var starterMessageID sql.NullString
	var answerChannelID sql.NullString
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
		&record.ChannelID,
		&questionListThreadID,
		&questionListEntryMessageID,
		&threadID,
		&starterMessageID,
		&answerChannelID,
		&record.QuestionTextSnapshot,
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
	record.QuestionListThreadID = strings.TrimSpace(questionListThreadID.String)
	record.QuestionListEntryMessageID = strings.TrimSpace(questionListEntryMessageID.String)
	record.DiscordThreadID = threadID.String
	record.DiscordStarterMessageID = starterMessageID.String
	record.AnswerChannelID = strings.TrimSpace(answerChannelID.String)
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

func scanQOTDCollectedQuestionRecord(scanner qotdRowScanner) (*QOTDCollectedQuestionRecord, error) {
	var record QOTDCollectedQuestionRecord
	var sourceAuthorID sql.NullString
	var sourceAuthorName sql.NullString
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.SourceChannelID,
		&record.SourceMessageID,
		&sourceAuthorID,
		&sourceAuthorName,
		&record.SourceCreatedAt,
		&record.EmbedTitle,
		&record.QuestionText,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.SourceAuthorID = strings.TrimSpace(sourceAuthorID.String)
	record.SourceAuthorNameSnapshot = strings.TrimSpace(sourceAuthorName.String)
	record.SourceCreatedAt = record.SourceCreatedAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	return &record, nil
}

func scanQOTDThreadArchiveRecord(scanner qotdRowScanner) (*QOTDThreadArchiveRecord, error) {
	var record QOTDThreadArchiveRecord
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.OfficialPostID,
		&record.SourceKind,
		&record.DiscordThreadID,
		&record.ArchivedAt,
		&record.CreatedAt,
	); err != nil {
		return nil, err
	}
	record.ArchivedAt = record.ArchivedAt.UTC()
	return &record, nil
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
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
