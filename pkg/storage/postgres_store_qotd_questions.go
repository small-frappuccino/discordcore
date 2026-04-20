package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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
