package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type qotdRowScanner interface {
	Scan(dest ...any) error
}

// CreateQOTDQuestion creates qotdquestion.
func (s *Store) CreateQOTDQuestion(ctx context.Context, rec QOTDQuestionRecord) (res *QOTDQuestionRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("create qotd question: %w", err)
		}
	}()
	normalized, err := normalizeQOTDQuestionRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDQuestion: %w", err)
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
			used_at,
			published_once_at
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			CASE
				WHEN $5 > 0 THEN $6
				ELSE COALESCE((SELECT MAX(queue_position) + 1 FROM qotd_questions WHERE guild_id = $7 AND deck_id = $8), 1)
			END,
			COALESCE((SELECT MAX(display_id) + 1 FROM qotd_questions WHERE guild_id = $9 AND deck_id = $10), 1),
			$11,
			$12,
			$13,
			$14
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
			published_once_at,
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
		nullableTime(normalized.PublishedOnceAt),
	)
	created, err := scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDQuestion: %w", err)
	}
	return created, nil
}

// UpdateQOTDQuestion updates qotdquestion.
func (s *Store) UpdateQOTDQuestion(ctx context.Context, rec QOTDQuestionRecord) (_ *QOTDQuestionRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update qotd question: %w", err)
		}
	}()
	if rec.ID <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	normalized, err := normalizeQOTDQuestionRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	var currentDeckID string
	var currentQueuePosition int64
	if err := txQueryRow(tx,
		`SELECT deck_id, queue_position FROM qotd_questions WHERE id = $1 AND guild_id = $2 FOR UPDATE`,
		normalized.ID,
		normalized.GuildID,
	).Scan(&currentDeckID, &currentQueuePosition); err != nil {
		return nil, err
	}

	position := normalized.QueuePosition
	if position < 1 {
		position = 0
	}

	movingDeck := currentDeckID != normalized.DeckID
	row := txQueryRow(tx,
		`UPDATE qotd_questions
		SET
			deck_id = $1,
			body = $2,
			status = $3,
			queue_position = CASE
				WHEN $4 > 0 THEN $5
				WHEN $6 THEN COALESCE((SELECT MAX(queue_position) + 1 FROM qotd_questions WHERE guild_id = $7 AND deck_id = $8), 1)
				ELSE queue_position
			END,
			display_id = CASE
				WHEN $9 THEN COALESCE((SELECT MAX(display_id) + 1 FROM qotd_questions WHERE guild_id = $10 AND deck_id = $11), 1)
				ELSE display_id
			END,
			created_by = $12,
			scheduled_for_date_utc = $13,
			used_at = $14,
			published_once_at = $15,
			updated_at = NOW()
		WHERE id = $16 AND guild_id = $17
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
			published_once_at,
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
		nullableTime(normalized.PublishedOnceAt),
		normalized.ID,
		normalized.GuildID,
	)
	updated, err := scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
	}

	needsReindex := movingDeck || (position > 0 && position != currentQueuePosition)
	if needsReindex {
		if movingDeck {
			if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, normalized.GuildID, currentDeckID); err != nil {
				return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
			}
		}
		if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, normalized.GuildID, normalized.DeckID); err != nil {
			return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
		}
		updated, err = getQOTDQuestionTx(ctx, tx, normalized.GuildID, normalized.ID)
		if err != nil {
			return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDQuestion: %w", err)
	}
	return updated, nil
}

// DeleteQOTDQuestion deletes qotdquestion.
func (s *Store) DeleteQOTDQuestion(ctx context.Context, guildID string, questionID int64) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("delete qotd question: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || questionID <= 0 {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestion: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	var deckID string
	if err := txQueryRow(tx,
		`SELECT deck_id FROM qotd_questions WHERE guild_id = $1 AND id = $2 FOR UPDATE`,
		guildID,
		questionID,
	).Scan(&deckID); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	if _, err := txExecContext(ctx, tx, `DELETE FROM qotd_questions WHERE guild_id = $1 AND id = $2`, guildID, questionID); err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestion: %w", err)
	}
	if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, guildID, deckID); err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestion: %w", err)
	}
	return nil
}

// DeleteQOTDQuestionsByDecks deletes qotdquestions by decks.
func (s *Store) DeleteQOTDQuestionsByDecks(ctx context.Context, guildID string, deckIDs []string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("delete qotd questions by decks: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil
	}
	normalizedDeckIDs := normalizeQOTDDeckIDs(deckIDs)
	if len(normalizedDeckIDs) == 0 {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestionsByDecks: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	for _, deckID := range normalizedDeckIDs {
		if _, err := txExecContext(ctx, tx,
			`DELETE FROM qotd_questions WHERE guild_id = $1 AND deck_id = $2`,
			guildID,
			deckID,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Store.DeleteQOTDQuestionsByDecks: %w", err)
	}
	return nil
}

// ListQOTDQuestions lists qotdquestions.
func (s *Store) ListQOTDQuestions(ctx context.Context, guildID, deckID string) (_ iter.Seq2[QOTDQuestionRecord, error], err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list qotd questions: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, nil
	}

	return func(yield func(QOTDQuestionRecord, error) bool) {
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
				published_once_at,
				created_at,
				updated_at
			FROM qotd_questions
			WHERE guild_id = $1
			  AND ($2 = '' OR deck_id = $3)
			ORDER BY queue_position ASC, id ASC`,
			guildID,
			deckID,
			deckID,
		)
		if err != nil {
			yield(QOTDQuestionRecord{}, fmt.Errorf("Store.ListQOTDQuestions: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			record, err := scanQOTDQuestionRecord(rows)
			if err != nil {
				yield(QOTDQuestionRecord{}, fmt.Errorf("Store.ListQOTDQuestions: %w", err))
				return
			}
			if !yield(*record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(QOTDQuestionRecord{}, fmt.Errorf("Store.ListQOTDQuestions: %w", err))
		}
	}, nil

}

// GetQOTDQuestion gets qotdquestion.
func (s *Store) GetQOTDQuestion(ctx context.Context, guildID string, questionID int64) (*QOTDQuestionRecord, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || questionID <= 0 {
		return nil, nil
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
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = $1 AND id = $2`,
		guildID,
		questionID,
	)
	record, err := scanQOTDQuestionRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDQuestion: %w", err)
	}
	return record, nil
}

// ReorderQOTDQuestions reorders qotdquestions.
func (s *Store) ReorderQOTDQuestions(ctx context.Context, guildID, deckID string, orderedIDs []int64) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("reorder qotd questions: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return fmt.Errorf("guild_id is required")
	}
	if deckID == "" {
		return fmt.Errorf("deck_id is required")
	}
	normalizedIDs, err := normalizeQOTDOrderedIDs(orderedIDs)
	if err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	rows, err := txQueryContext(ctx, tx,
		`SELECT id, queue_position
		FROM qotd_questions
		WHERE guild_id = $1
		  AND deck_id = $2
		ORDER BY queue_position ASC, id ASC
		FOR UPDATE`,
		guildID,
		deckID,
	)
	if err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	defer rows.Close()

	currentIDs := make([]int64, 0, len(normalizedIDs))
	var maxQueuePosition int64
	for rows.Next() {
		var id int64
		var queuePosition int64
		if err := rows.Scan(&id, &queuePosition); err != nil {
			return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
		}
		currentIDs = append(currentIDs, id)
		if queuePosition > maxQueuePosition {
			maxQueuePosition = queuePosition
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	if !sameQOTDIDSet(currentIDs, normalizedIDs) {
		return fmt.Errorf("ordered ids must match the full guild question set")
	}

	// Move rows out of the indexed range first so swaps do not trip the unique queue constraint.
	tempBase := maxQueuePosition + int64(len(normalizedIDs))
	for idx, id := range normalizedIDs {
		if _, err := txExecContext(ctx, tx,
			`UPDATE qotd_questions SET queue_position = $1, updated_at = NOW() WHERE guild_id = $2 AND deck_id = $3 AND id = $4`,
			tempBase+int64(idx)+1,
			guildID,
			deckID,
			id,
		); err != nil {
			return err
		}
	}
	for idx, id := range normalizedIDs {
		if _, err := txExecContext(ctx, tx,
			`UPDATE qotd_questions SET queue_position = $1, updated_at = NOW() WHERE guild_id = $2 AND deck_id = $3 AND id = $4`,
			idx+1,
			guildID,
			deckID,
			id,
		); err != nil {
			return err
		}
	}
	if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, guildID, deckID); err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Store.ReorderQOTDQuestions: %w", err)
	}
	return nil
}

// ReserveNextQOTDQuestion reserves next qotdquestion.
func (s *Store) ReserveNextQOTDQuestion(ctx context.Context, guildID, deckID string, publishDateUTC time.Time, selector QOTDQuestionSelector) (_ *QOTDQuestionRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("reserve qotd question: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, fmt.Errorf("guild_id is required")
	}
	if deckID == "" {
		return nil, fmt.Errorf("deck_id is required")
	}
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if publishDateUTC.IsZero() {
		return nil, fmt.Errorf("publish_date_utc is required")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Store.ReserveNextQOTDQuestion: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

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
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = $1
		  AND deck_id = $2
		  AND status = 'ready'
		  AND scheduled_for_date_utc IS NULL
		  AND published_once_at IS NULL
		`+selector.orderByClause()+`
		FOR UPDATE SKIP LOCKED
		LIMIT 1`,
		guildID,
		deckID,
	)
	record, err := scanQOTDQuestionRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.ReserveNextQOTDQuestion: %w", err)
	}

	row = txQueryRow(tx,
		`UPDATE qotd_questions
		SET
			status = 'reserved',
			scheduled_for_date_utc = $1,
			updated_at = NOW()
		WHERE id = $2
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
			published_once_at,
			created_at,
			updated_at`,
		publishDateUTC,
		record.ID,
	)
	record, err = scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.ReserveNextQOTDQuestion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("Store.ReserveNextQOTDQuestion: %w", err)
	}
	return record, nil
}

// ReserveNextReadyQOTDQuestion reserves next ready qotdquestion.
func (s *Store) ReserveNextReadyQOTDQuestion(ctx context.Context, guildID, deckID string, selector QOTDQuestionSelector) (_ *QOTDQuestionRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("reserve ready qotd question: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, fmt.Errorf("guild_id is required")
	}
	if deckID == "" {
		return nil, fmt.Errorf("deck_id is required")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Store.ReserveNextReadyQOTDQuestion: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

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
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = $1
		  AND deck_id = $2
		  AND status = 'ready'
		  AND scheduled_for_date_utc IS NULL
		  AND published_once_at IS NULL
		`+selector.orderByClause()+`
		FOR UPDATE SKIP LOCKED
		LIMIT 1`,
		guildID,
		deckID,
	)
	record, err := scanQOTDQuestionRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.ReserveNextReadyQOTDQuestion: %w", err)
	}

	row = txQueryRow(tx,
		`UPDATE qotd_questions
		SET
			status = 'reserved',
			updated_at = NOW()
		WHERE id = $1
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
			published_once_at,
			created_at,
			updated_at`,
		record.ID,
	)
	record, err = scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.ReserveNextReadyQOTDQuestion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("Store.ReserveNextReadyQOTDQuestion: %w", err)
	}
	return record, nil
}

// ReclaimOrphanReservedQOTDQuestions releases reservations whose
// CreateQOTDOfficialPostProvisioning never landed (process crashed between
// ReserveNextQOTDQuestion and the official-post insert) so they re-enter the
// publish queue. We restrict to scheduled_for_date_utc < todayUTC: today's
// reservations may belong to an in-flight publish that another goroutine is
// currently running. Returns the freed question IDs in queue order so callers
// can log or test the cleanup deterministically.
func (s *Store) ReclaimOrphanReservedQOTDQuestions(ctx context.Context, guildID string, todayUTC time.Time) iter.Seq2[int64, error] {
	return func(yield func(int64, error) bool) {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return
		}
		todayUTC = normalizeQOTDDateUTC(todayUTC)
		if todayUTC.IsZero() {
			yield(0, fmt.Errorf("reclaim orphan qotd reservations: today_utc is required"))
			return
		}

		rows, err := s.queryContext(ctx,
			`UPDATE qotd_questions q
			 SET
				status = 'ready',
				scheduled_for_date_utc = NULL,
				updated_at = NOW()
			 WHERE q.guild_id = $1
			   AND q.status = 'reserved'
			   AND q.scheduled_for_date_utc IS NOT NULL
			   AND q.scheduled_for_date_utc < $2
			   AND NOT EXISTS (
				 SELECT 1
				 FROM qotd_official_posts p
				 WHERE p.guild_id = q.guild_id
				   AND p.question_id = q.id
			   )
			 RETURNING q.id`,
			guildID,
			todayUTC,
		)
		if err != nil {
			yield(0, fmt.Errorf("reclaim orphan qotd reservations: Store.ReclaimOrphanReservedQOTDQuestions: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				yield(0, fmt.Errorf("reclaim orphan qotd reservations: Store.ReclaimOrphanReservedQOTDQuestions: %w", err))
				return
			}
			if !yield(id, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(0, fmt.Errorf("reclaim orphan qotd reservations: Store.ReclaimOrphanReservedQOTDQuestions: %w", err))
		}
	}
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
	rec.PublishedOnceAt = normalizeQOTDTimePtr(rec.PublishedOnceAt)

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
	var publishedOnceAt sql.NullTime
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
		&publishedOnceAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.ScheduledForDateUTC = timePtrFromNull(scheduledFor)
	record.UsedAt = timePtrFromNull(usedAt)
	record.PublishedOnceAt = timePtrFromNull(publishedOnceAt)
	return &record, nil
}

func getQOTDQuestionTx(ctx context.Context, tx pgx.Tx, guildID string, questionID int64) (*QOTDQuestionRecord, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
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
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = $1 AND id = $2`,
		guildID,
		questionID,
	)
	record, err := scanQOTDQuestionRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getQOTDQuestionTx: %w", err)
	}
	return record, nil
}

func reindexQOTDQuestionDisplayIDsTx(ctx context.Context, tx pgx.Tx, guildID, deckID string) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return nil
	}

	if _, err := txExecContext(ctx, tx,
		`UPDATE qotd_questions
		SET display_id = -id
		WHERE guild_id = $1 AND deck_id = $2`,
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
			WHERE guild_id = $1
			  AND deck_id = $2
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

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
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

func zeroEmptyString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: value, Valid: true}
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
