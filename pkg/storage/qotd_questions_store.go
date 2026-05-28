package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/errutil"
)

type qotdRowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) CreateQOTDQuestion(ctx context.Context, rec QOTDQuestionRecord) (*QOTDQuestionRecord, error) {
	normalized, err := normalizeQOTDQuestionRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("create qotd question: %w", err)
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
		return nil, fmt.Errorf("create qotd question: %w", err)
	}
	return created, nil
}

func (s *Store) UpdateQOTDQuestion(ctx context.Context, rec QOTDQuestionRecord) (_ *QOTDQuestionRecord, err error) {
	defer func() { err = errutil.Wrap(err, "update qotd question") }()
	if rec.ID <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	normalized, err := normalizeQOTDQuestionRecord(rec)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentDeckID string
	var currentQueuePosition int64
	if err := txQueryRow(tx,
		`SELECT deck_id, queue_position FROM qotd_questions WHERE id = ? AND guild_id = ? FOR UPDATE`,
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
			published_once_at = ?,
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
		return nil, err
	}

	needsReindex := movingDeck || (position > 0 && position != currentQueuePosition)
	if needsReindex {
		if movingDeck {
			if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, normalized.GuildID, currentDeckID); err != nil {
				return nil, err
			}
		}
		if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, normalized.GuildID, normalized.DeckID); err != nil {
			return nil, err
		}
		updated, err = getQOTDQuestionTx(ctx, tx, normalized.GuildID, normalized.ID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Store) DeleteQOTDQuestion(ctx context.Context, guildID string, questionID int64) (err error) {
	defer func() { err = errutil.Wrap(err, "delete qotd question") }()
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || questionID <= 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
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
		return err
	}

	if _, err := txExecContext(ctx, tx, `DELETE FROM qotd_questions WHERE guild_id = ? AND id = ?`, guildID, questionID); err != nil {
		return err
	}
	if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, guildID, deckID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) DeleteQOTDQuestionsByDecks(ctx context.Context, guildID string, deckIDs []string) (err error) {
	defer func() { err = errutil.Wrap(err, "delete qotd questions by decks") }()
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil
	}
	normalizedDeckIDs := normalizeQOTDDeckIDs(deckIDs)
	if len(normalizedDeckIDs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, deckID := range normalizedDeckIDs {
		if _, err := txExecContext(ctx, tx,
			`DELETE FROM qotd_questions WHERE guild_id = ? AND deck_id = ?`,
			guildID,
			deckID,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ListQOTDQuestions(ctx context.Context, guildID, deckID string) (_ []QOTDQuestionRecord, err error) {
	defer func() { err = errutil.Wrap(err, "list qotd questions") }()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, nil
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
			published_once_at,
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
		return nil, err
	}
	defer rows.Close()

	records := make([]QOTDQuestionRecord, 0, 16)
	for rows.Next() {
		record, err := scanQOTDQuestionRecord(rows)
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

func (s *Store) ReorderQOTDQuestions(ctx context.Context, guildID, deckID string, orderedIDs []int64) (err error) {
	defer func() { err = errutil.Wrap(err, "reorder qotd questions") }()
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
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := txQueryContext(ctx, tx,
		`SELECT id, queue_position
		FROM qotd_questions
		WHERE guild_id = ?
		  AND deck_id = ?
		ORDER BY queue_position ASC, id ASC
		FOR UPDATE`,
		guildID,
		deckID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	currentIDs := make([]int64, 0, len(normalizedIDs))
	var maxQueuePosition int64
	for rows.Next() {
		var id int64
		var queuePosition int64
		if err := rows.Scan(&id, &queuePosition); err != nil {
			return err
		}
		currentIDs = append(currentIDs, id)
		if queuePosition > maxQueuePosition {
			maxQueuePosition = queuePosition
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !sameQOTDIDSet(currentIDs, normalizedIDs) {
		return fmt.Errorf("ordered ids must match the full guild question set")
	}

	// Move rows out of the indexed range first so swaps do not trip the unique queue constraint.
	tempBase := maxQueuePosition + int64(len(normalizedIDs))
	for idx, id := range normalizedIDs {
		if _, err := txExecContext(ctx, tx,
			`UPDATE qotd_questions SET queue_position = ?, updated_at = NOW() WHERE guild_id = ? AND deck_id = ? AND id = ?`,
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
			`UPDATE qotd_questions SET queue_position = ?, updated_at = NOW() WHERE guild_id = ? AND deck_id = ? AND id = ?`,
			idx+1,
			guildID,
			deckID,
			id,
		); err != nil {
			return err
		}
	}
	if err := reindexQOTDQuestionDisplayIDsTx(ctx, tx, guildID, deckID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ReserveNextQOTDQuestion(ctx context.Context, guildID, deckID string, publishDateUTC time.Time, selector QOTDQuestionSelector) (_ *QOTDQuestionRecord, err error) {
	defer func() { err = errutil.Wrap(err, "reserve qotd question") }()
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
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
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = ?
		  AND deck_id = ?
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
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
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
			published_once_at,
			created_at,
			updated_at`,
		publishDateUTC,
		record.ID,
	)
	record, err = scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *Store) ReserveNextReadyQOTDQuestion(ctx context.Context, guildID, deckID string, selector QOTDQuestionSelector) (_ *QOTDQuestionRecord, err error) {
	defer func() { err = errutil.Wrap(err, "reserve ready qotd question") }()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" {
		return nil, fmt.Errorf("guild_id is required")
	}
	if deckID == "" {
		return nil, fmt.Errorf("deck_id is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
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
			published_once_at,
			created_at,
			updated_at
		FROM qotd_questions
		WHERE guild_id = ?
		  AND deck_id = ?
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
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
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
			published_once_at,
			created_at,
			updated_at`,
		record.ID,
	)
	record, err = scanQOTDQuestionRecord(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
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
func (s *Store) ReclaimOrphanReservedQOTDQuestions(ctx context.Context, guildID string, todayUTC time.Time) (_ []int64, err error) {
	defer func() { err = errutil.Wrap(err, "reclaim orphan qotd reservations") }()
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, nil
	}
	todayUTC = normalizeQOTDDateUTC(todayUTC)
	if todayUTC.IsZero() {
		return nil, fmt.Errorf("today_utc is required")
	}

	rows, err := s.queryContext(ctx,
		`UPDATE qotd_questions q
		 SET
			status = 'ready',
			scheduled_for_date_utc = NULL,
			updated_at = NOW()
		 WHERE q.guild_id = ?
		   AND q.status = 'reserved'
		   AND q.scheduled_for_date_utc IS NOT NULL
		   AND q.scheduled_for_date_utc < ?
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
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0, 4)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
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

func getQOTDQuestionTx(ctx context.Context, tx *sql.Tx, guildID string, questionID int64) (*QOTDQuestionRecord, error) {
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

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
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
