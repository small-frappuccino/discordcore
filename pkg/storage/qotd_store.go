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

// CreateQOTDOfficialPostProvisioning inserts a fresh provisioning row and
// allocates a publish ordinal in the same statement. Within a single bot
// process the qotd Service serializes calls per-guild via its lifecycle
// lock, so the SELECT MAX subquery is race-free in practice. Across replicas
// a concurrent provisioning could observe the same MAX and both attempt the
// same ordinal; the unique index idx_qotd_official_posts_publish_ordinal
// then forces one of them to fail with a duplicate-key violation. Callers
// see this as a generic error (not a scheduled-publish conflict, since the
// semantics differ) and a subsequent attempt will succeed against the
// updated MAX.
func (s *Store) CreateQOTDOfficialPostProvisioning(ctx context.Context, rec QOTDOfficialPostRecord) (*QOTDOfficialPostRecord, error) {
	normalized, err := normalizeQOTDOfficialPostRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("create qotd official post: %w", err)
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
			last_reconciled_at
		)
		VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			COALESCE((SELECT MAX(publish_ordinal) FROM qotd_official_posts WHERE guild_id = ? AND deck_id = ?), 0) + 1,
			?, ?, ?, ?, ?, ?
		)
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
		normalized.GuildID,
		normalized.DeckID,
		normalized.DeckNameSnapshot,
		normalized.QuestionID,
		normalized.PublishMode,
		normalized.ConsumeAutomaticSlot,
		normalized.PublishDateUTC,
		normalized.State,
		normalized.ChannelID,
		normalized.QuestionListThreadID,
		normalized.QuestionListEntryMessageID,
		zeroEmptyString(normalized.DiscordThreadID),
		zeroEmptyString(normalized.DiscordStarterMessageID),
		normalized.AnswerChannelID,
		normalized.QuestionTextSnapshot,
		zeroEmptyString(normalized.Nonce),
		// publish_ordinal subquery binds:
		normalized.GuildID,
		normalized.DeckID,
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

func (s *Store) FinalizeQOTDOfficialPost(ctx context.Context, id int64, questionListThreadID, questionListEntryMessageID, discordThreadID, starterMessageID, answerChannelID string, publishedAt time.Time) (_ *QOTDOfficialPostRecord, err error) {
	defer func() { err = errutil.Wrap(err, "finalize qotd official post") }()
	if id <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	questionListThreadID = strings.TrimSpace(questionListThreadID)
	questionListEntryMessageID = strings.TrimSpace(questionListEntryMessageID)
	discordThreadID = strings.TrimSpace(discordThreadID)
	starterMessageID = strings.TrimSpace(starterMessageID)
	answerChannelID = strings.TrimSpace(answerChannelID)
	if starterMessageID == "" {
		return nil, fmt.Errorf("starter message id is required")
	}
	if answerChannelID == "" {
		return nil, fmt.Errorf("answer channel id is required")
	}
	if publishedAt.IsZero() {
		return nil, fmt.Errorf("published_at is required")
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
		return nil, err
	}
	return updated, nil
}

func (s *Store) GetQOTDOfficialPostByID(ctx context.Context, id int64) (*QOTDOfficialPostRecord, error) {
	if id <= 0 {
		return nil, nil
	}
	row := s.queryRowContext(ctx,
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
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}
	row := s.queryRowContext(ctx,
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
		  AND publish_date_utc = ?
		ORDER BY
		  CASE WHEN archived_at IS NULL THEN 0 ELSE 1 END,
		  CASE
		    WHEN published_at IS NOT NULL
		      AND discord_thread_id IS NOT NULL
		      AND discord_starter_message_id IS NOT NULL
		      AND answer_channel_id IS NOT NULL THEN 0
		    ELSE 1
		  END,
		  COALESCE(published_at, updated_at) DESC,
		  id DESC,
		  updated_at DESC
		LIMIT 1`,
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

func (s *Store) ListQOTDOfficialPostsByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (_ []QOTDOfficialPostRecord, err error) {
	defer func() { err = errutil.Wrap(err, "list qotd official posts by date") }()
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
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
		  AND publish_date_utc = ?
		ORDER BY
		  CASE WHEN archived_at IS NULL THEN 0 ELSE 1 END,
		  CASE
		    WHEN published_at IS NOT NULL
		      AND discord_thread_id IS NOT NULL
		      AND discord_starter_message_id IS NOT NULL
		      AND answer_channel_id IS NOT NULL THEN 0
		    ELSE 1
		  END,
		  CASE WHEN publish_mode = 'scheduled' THEN 0 ELSE 1 END,
		  COALESCE(published_at, updated_at) DESC,
		  id DESC,
		  updated_at DESC`,
		guildID,
		publishDateUTC,
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

func (s *Store) GetAutomaticSlotQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (*QOTDOfficialPostRecord, error) {
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}
	row := s.queryRowContext(ctx,
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
		  AND publish_date_utc = ?
		  AND (publish_mode = 'scheduled' OR consume_automatic_slot = TRUE)
		ORDER BY
		  CASE WHEN archived_at IS NULL THEN 0 ELSE 1 END,
		  CASE
		    WHEN published_at IS NOT NULL
		      AND discord_thread_id IS NOT NULL
		      AND discord_starter_message_id IS NOT NULL
		      AND answer_channel_id IS NOT NULL THEN 0
		    ELSE 1
		  END,
		  CASE WHEN publish_mode = 'scheduled' THEN 0 ELSE 1 END,
		  COALESCE(published_at, updated_at) DESC,
		  id DESC,
		  updated_at DESC
		LIMIT 1`,
		guildID,
		publishDateUTC,
	)
	record, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get automatic-slot qotd official post by date: %w", err)
	}
	return record, nil
}

func (s *Store) GetScheduledQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (*QOTDOfficialPostRecord, error) {
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}

	row := s.queryRowContext(ctx,
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
		  AND publish_mode = 'scheduled'
		  AND publish_date_utc = ?
		ORDER BY
		  CASE WHEN archived_at IS NULL THEN 0 ELSE 1 END,
		  CASE
		    WHEN published_at IS NOT NULL
		      AND discord_thread_id IS NOT NULL
		      AND discord_starter_message_id IS NOT NULL
		      AND answer_channel_id IS NOT NULL THEN 0
		    ELSE 1
		  END,
		  updated_at DESC,
		  id DESC
		LIMIT 1`,
		guildID,
		publishDateUTC,
	)
	record, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get scheduled qotd official post by date: %w", err)
	}
	return record, nil
}

func (s *Store) GetCurrentAndPreviousQOTDPosts(ctx context.Context, guildID string, now time.Time) (_ []QOTDOfficialPostRecord, err error) {
	defer func() { err = errutil.Wrap(err, "list current and previous qotd posts") }()
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
	defer func() { err = errutil.Wrap(err, "list qotd official posts needing archive") }()
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
	defer func() { err = errutil.Wrap(err, "update qotd official post state") }()
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

func (s *Store) DeleteQOTDOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (int, error) {
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return 0, nil
	}

	result, err := s.execContext(ctx,
		`DELETE FROM qotd_official_posts WHERE guild_id = ? AND deck_id = ?`,
		guildID,
		deckID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete qotd official posts by deck: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete qotd official posts by deck: %w", err)
	}
	return int(deleted), nil
}

func (s *Store) DeleteQOTDOfficialPostByID(ctx context.Context, id int64) error {
	if id <= 0 {
		return nil
	}

	if _, err := s.execContext(ctx, `DELETE FROM qotd_official_posts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete qotd official post by id: %w", err)
	}
	return nil
}

func (s *Store) DeleteQOTDUnpublishedOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (int, error) {
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return 0, nil
	}

	result, err := s.execContext(ctx,
		`DELETE FROM qotd_official_posts
		 WHERE guild_id = ?
		   AND deck_id = ?
		   AND published_at IS NULL`,
		guildID,
		deckID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete unpublished qotd official posts by deck: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete unpublished qotd official posts by deck: %w", err)
	}
	return int(deleted), nil
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
	rec.Nonce = strings.TrimSpace(rec.Nonce)
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
	if rec.PublishMode == "scheduled" {
		rec.ConsumeAutomaticSlot = true
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

func scanQOTDOfficialPostRecord(scanner qotdRowScanner) (*QOTDOfficialPostRecord, error) {
	var record QOTDOfficialPostRecord
	var questionListThreadID sql.NullString
	var questionListEntryMessageID sql.NullString
	var threadID sql.NullString
	var starterMessageID sql.NullString
	var answerChannelID sql.NullString
	var consumeAutomaticSlot bool
	var nonce sql.NullString
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
		&consumeAutomaticSlot,
		&record.PublishDateUTC,
		&record.State,
		&record.ChannelID,
		&questionListThreadID,
		&questionListEntryMessageID,
		&threadID,
		&starterMessageID,
		&answerChannelID,
		&record.QuestionTextSnapshot,
		&nonce,
		&record.PublishOrdinal,
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
	record.ConsumeAutomaticSlot = consumeAutomaticSlot
	record.QuestionListThreadID = strings.TrimSpace(questionListThreadID.String)
	record.QuestionListEntryMessageID = strings.TrimSpace(questionListEntryMessageID.String)
	record.DiscordThreadID = threadID.String
	record.DiscordStarterMessageID = starterMessageID.String
	record.AnswerChannelID = strings.TrimSpace(answerChannelID.String)
	record.Nonce = strings.TrimSpace(nonce.String)
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
