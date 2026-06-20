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
	"github.com/small-frappuccino/discordcore/pkg/idgen"
)

// QOTDQuestionSelector controls how the next question is picked when
// reserving for a publish. It is decoupled from QOTDOfficialPostRecord's
// PublishOrdinal so the visible thread numbering stays monotonic regardless
// of which strategy ran.
type QOTDQuestionSelector string

const (
	// QOTDQuestionSelectorQueue picks the head of the queue (queue_position
	// ASC, id ASC). This is the historical default.
	QOTDQuestionSelectorQueue QOTDQuestionSelector = "queue"
	// QOTDQuestionSelectorRandom picks a uniformly-random eligible question.
	QOTDQuestionSelectorRandom QOTDQuestionSelector = "random"
)

func (s QOTDQuestionSelector) normalized() QOTDQuestionSelector {
	switch s {
	case QOTDQuestionSelectorRandom:
		return QOTDQuestionSelectorRandom
	default:
		return QOTDQuestionSelectorQueue
	}
}

// orderByClause returns the SQL ORDER BY fragment that implements the
// strategy. It is composed into the reserve queries and is intentionally
// bind-parameter-free so it can be inlined safely.
func (s QOTDQuestionSelector) orderByClause() string {
	if s.normalized() == QOTDQuestionSelectorRandom {
		return "ORDER BY RANDOM()"
	}
	return "ORDER BY queue_position ASC, id ASC"
}

// QOTDQuestionRecord is a stored QOTD question. ID is the global primary key;
// DisplayID is the stable per-guild identifier shown to users. Status holds a
// qotd.QuestionStatus value, and the *time.Time fields are nil until the
// corresponding lifecycle event (scheduled, used, first published) occurs.
type QOTDQuestionRecord struct {
	ID                  int64
	DisplayID           int64
	GuildID             string
	DeckID              string
	Body                string
	Status              string
	QueuePosition       int64
	CreatedBy           string
	ScheduledForDateUTC *time.Time
	UsedAt              *time.Time
	PublishedOnceAt     *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// QOTDOfficialPostRecord is the durable state of one official QOTD post and the
// source of truth the reconcile loop drives toward Discord. State holds a
// qotd.OfficialPostState value. Two of its fields anchor publish idempotency
// (see the PublishOrdinal and Nonce field comments); changing publish paths
// must preserve both.
type QOTDOfficialPostRecord struct {
	ID               int64
	GuildID          string
	DeckID           string
	DeckNameSnapshot string
	QuestionID       int64
	// PublishOrdinal is the per-(guild,deck) publication sequence number
	// assigned at provisioning. The Discord thread title renders this
	// ("Question #001") so the visible numbering is decoupled from the
	// question's queue position and remains monotonic regardless of which
	// selection strategy (queue order vs random) chose the question.
	PublishOrdinal             int64
	PublishMode                string
	ConsumeAutomaticSlot       bool
	PublishDateUTC             time.Time
	State                      string
	ChannelID                  string
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	DiscordThreadID            string
	DiscordStarterMessageID    string
	AnswerChannelID            string
	QuestionTextSnapshot       string
	// Nonce is sent to Discord with enforce_nonce=true so that retried
	// publishes after a crash (record persisted, Discord call accepted, but
	// the message ID never made it back to our DB) deduplicate server-side
	// instead of producing a second QOTD post in the channel. Empty for
	// legacy records created before the column existed; the publisher falls
	// back to the non-idempotent send path in that case.
	Nonce            string
	PublishedAt      *time.Time
	GraceUntil       time.Time
	ArchiveAt        time.Time
	ClosedAt         *time.Time
	ArchivedAt       *time.Time
	LastReconciledAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// QOTDSurfaceRecord maps a guild deck to its Discord surface: the channel and
// the question-list thread used to render the deck's published questions.
type QOTDSurfaceRecord struct {
	ID                   int64
	GuildID              string
	DeckID               string
	ChannelID            string
	QuestionListThreadID string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// QOTDAnswerMessageRecord is a stored answer posted against an official post.
// State holds a qotd.AnswerRecordState value; ClosedAt and ArchivedAt are nil
// until the answer surface closes or is archived.
type QOTDAnswerMessageRecord struct {
	ID                      int64
	GuildID                 string
	OfficialPostID          int64
	UserID                  string
	State                   string
	AnswerChannelID         string
	DiscordMessageID        string
	CreatedViaInteractionID string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	ClosedAt                *time.Time
	ArchivedAt              *time.Time
}

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

	row := s.db.QueryRow(ctx, `INSERT INTO qotd_questions (
			id,
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
			$5,
			CASE
				WHEN $6 > 0 THEN $7
				ELSE COALESCE((SELECT MAX(queue_position) + 1 FROM qotd_questions WHERE guild_id = $8 AND deck_id = $9), 1)
			END,
			COALESCE((SELECT MAX(display_id) + 1 FROM qotd_questions WHERE guild_id = $10 AND deck_id = $11), 1),
			$12,
			$13,
			$14,
			$15
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
		idgen.GenerateID(),
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
	if err := tx.QueryRow(ctx,
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
	row := tx.QueryRow(ctx,
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
	if err := tx.QueryRow(ctx,
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
		rows, err := s.db.Query(ctx, `SELECT
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

	row := s.db.QueryRow(ctx, `SELECT
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

	row := tx.QueryRow(ctx,
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

	row = tx.QueryRow(ctx,
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

	row := tx.QueryRow(ctx,
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

	row = tx.QueryRow(ctx,
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

		rows, err := s.db.Query(ctx, `UPDATE qotd_questions q
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

	row := tx.QueryRow(ctx,
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

// CreateQOTDOfficialPostProvisioning creates qotdofficial post provisioning.
func (s *Store) CreateQOTDOfficialPostProvisioning(ctx context.Context, rec QOTDOfficialPostRecord) (res *QOTDOfficialPostRecord, err error) {
	normalized, err := normalizeQOTDOfficialPostRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDOfficialPostProvisioning: %w", err)
	}
	if normalized.State == "" {
		normalized.State = "provisioning"
	}

	row := s.db.QueryRow(ctx, `INSERT INTO qotd_official_posts (
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
			last_reconciled_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17,
			COALESCE((SELECT MAX(publish_ordinal) FROM qotd_official_posts WHERE guild_id = $18 AND deck_id = $19), 0) + 1,
			$20, $21, $22, $23, $24, $25
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
		idgen.GenerateID(),
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
		return nil, fmt.Errorf("Store.CreateQOTDOfficialPostProvisioning: %w", err)
	}
	return created, nil
}

// FinalizeQOTDOfficialPostParams carries the Discord-side identifiers and the
// publish timestamp recorded when a QOTD official post finishes provisioning.
type FinalizeQOTDOfficialPostParams struct {
	ID                         int64
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	DiscordThreadID            string
	StarterMessageID           string
	AnswerChannelID            string
	PublishedAt                time.Time
}

// FinalizeQOTDOfficialPost finalizes qotdofficial post.
func (s *Store) FinalizeQOTDOfficialPost(ctx context.Context, params FinalizeQOTDOfficialPostParams) (_ *QOTDOfficialPostRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("finalize qotd official post: %w", err)
		}
	}()
	if params.ID <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	questionListThreadID := strings.TrimSpace(params.QuestionListThreadID)
	questionListEntryMessageID := strings.TrimSpace(params.QuestionListEntryMessageID)
	discordThreadID := strings.TrimSpace(params.DiscordThreadID)
	starterMessageID := strings.TrimSpace(params.StarterMessageID)
	answerChannelID := strings.TrimSpace(params.AnswerChannelID)
	if starterMessageID == "" {
		return nil, fmt.Errorf("starter message id is required")
	}
	if answerChannelID == "" {
		return nil, fmt.Errorf("answer channel id is required")
	}
	if params.PublishedAt.IsZero() {
		return nil, fmt.Errorf("published_at is required")
	}

	row := s.db.QueryRow(ctx, `UPDATE qotd_official_posts
		SET
			question_list_thread_id = $1,
			question_list_entry_message_id = $2,
			discord_thread_id = $3,
			discord_starter_message_id = $4,
			answer_channel_id = $5,
			published_at = $6,
			updated_at = NOW()
		WHERE id = $7
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
		params.PublishedAt.UTC(),
		params.ID,
	)
	updated, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.FinalizeQOTDOfficialPost: %w", err)
	}
	return updated, nil
}

// GetQOTDOfficialPostByID gets qotdofficial post by id.
func (s *Store) GetQOTDOfficialPostByID(ctx context.Context, id int64) (res *QOTDOfficialPostRecord, err error) {
	if id <= 0 {
		return nil, nil
	}
	row := s.db.QueryRow(ctx, `SELECT
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
		WHERE id = $1`,
		id,
	)
	record, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDOfficialPostByID: %w", err)
	}
	return record, nil
}

// GetQOTDOfficialPostByDate gets qotdofficial post by date.
func (s *Store) GetQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (res *QOTDOfficialPostRecord, err error) {
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}
	row := s.db.QueryRow(ctx, `SELECT
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
		WHERE guild_id = $1
		  AND publish_date_utc = $2::DATE
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
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDOfficialPostByDate: %w", err)
	}
	return record, nil
}

// ListQOTDOfficialPostsByDate lists qotdofficial posts by date.
func (s *Store) ListQOTDOfficialPostsByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (_ iter.Seq[QOTDOfficialPostRecord], err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list qotd official posts by date: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return func(yield func(QOTDOfficialPostRecord) bool) {}, nil
	}

	rows, err := s.db.Query(ctx, `SELECT
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
		WHERE guild_id = $1
		  AND publish_date_utc = $2::DATE
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
		return nil, fmt.Errorf("Store.ListQOTDOfficialPostsByDate: %w", err)
	}

	return func(yield func(QOTDOfficialPostRecord) bool) {
		defer rows.Close()
		for rows.Next() {
			record, err := scanQOTDOfficialPostRecord(rows)
			if err != nil {
				return
			}
			if !yield(*record) {
				return
			}
		}
	}, nil
}

// GetAutomaticSlotQOTDOfficialPostByDate gets automatic slot qotdofficial post by date.
func (s *Store) GetAutomaticSlotQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (res *QOTDOfficialPostRecord, err error) {
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}
	row := s.db.QueryRow(ctx, `SELECT
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
		WHERE guild_id = $1
		  AND publish_date_utc = $2::DATE
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
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetAutomaticSlotQOTDOfficialPostByDate: %w", err)
	}
	return record, nil
}

// GetScheduledQOTDOfficialPostByDate gets scheduled qotdofficial post by date.
func (s *Store) GetScheduledQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (res *QOTDOfficialPostRecord, err error) {
	guildID = strings.TrimSpace(guildID)
	publishDateUTC = normalizeQOTDDateUTC(publishDateUTC)
	if guildID == "" || publishDateUTC.IsZero() {
		return nil, nil
	}

	row := s.db.QueryRow(ctx, `SELECT
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
		WHERE guild_id = $1
		  AND publish_mode = 'scheduled'
		  AND publish_date_utc = $2::DATE
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
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetScheduledQOTDOfficialPostByDate: %w", err)
	}
	return record, nil
}

// DeleteQOTDOfficialPostsByDeck deletes qotdofficial posts by deck.
func (s *Store) DeleteQOTDOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (count int, err error) {
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return 0, nil
	}

	result, err := s.db.Exec(ctx,
		`DELETE FROM qotd_official_posts WHERE guild_id = $1 AND deck_id = $2`,
		guildID,
		deckID,
	)
	if err != nil {
		return 0, fmt.Errorf("Store.DeleteQOTDOfficialPostsByDeck: %w", err)
	}
	deleted := result.RowsAffected()
	return int(deleted), nil
}

// DeleteQOTDOfficialPostByID deletes qotdofficial post by id.
func (s *Store) DeleteQOTDOfficialPostByID(ctx context.Context, id int64) (err error) {
	if id <= 0 {
		return nil
	}

	if _, err := s.db.Exec(ctx, `DELETE FROM qotd_official_posts WHERE id = $1`, id); err != nil {
		return fmt.Errorf("Store.DeleteQOTDOfficialPostByID: %w", err)
	}
	return nil
}

// DeleteQOTDUnpublishedOfficialPostsByDeck deletes qotdunpublished official posts by deck.
func (s *Store) DeleteQOTDUnpublishedOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (count int, err error) {
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return 0, nil
	}

	result, err := s.db.Exec(ctx,
		`DELETE FROM qotd_official_posts
		 WHERE guild_id = $1
		   AND deck_id = $2
		   AND published_at IS NULL`,
		guildID,
		deckID,
	)
	if err != nil {
		return 0, fmt.Errorf("Store.DeleteQOTDUnpublishedOfficialPostsByDeck: %w", err)
	}
	deleted := result.RowsAffected()
	return int(deleted), nil
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

// UpdateQOTDOfficialPostProgress updates qotdofficial post progress.
func (s *Store) UpdateQOTDOfficialPostProgress(ctx context.Context, id int64, progress QOTDOfficialPostRecord) (*QOTDOfficialPostRecord, error) {
	if id <= 0 {
		return nil, fmt.Errorf("update qotd official post progress: id is required")
	}
	progress.QuestionListThreadID = strings.TrimSpace(progress.QuestionListThreadID)
	progress.QuestionListEntryMessageID = strings.TrimSpace(progress.QuestionListEntryMessageID)
	progress.DiscordThreadID = strings.TrimSpace(progress.DiscordThreadID)
	progress.DiscordStarterMessageID = strings.TrimSpace(progress.DiscordStarterMessageID)
	progress.AnswerChannelID = strings.TrimSpace(progress.AnswerChannelID)
	progress.PublishedAt = normalizeQOTDTimePtr(progress.PublishedAt)

	row := s.db.QueryRow(ctx, `UPDATE qotd_official_posts
		SET
			question_list_thread_id = COALESCE(NULLIF($1, ''), question_list_thread_id),
			question_list_entry_message_id = COALESCE(NULLIF($2, ''), question_list_entry_message_id),
			discord_thread_id = COALESCE(NULLIF($3, ''), discord_thread_id),
			discord_starter_message_id = COALESCE(NULLIF($4, ''), discord_starter_message_id),
			answer_channel_id = COALESCE(NULLIF($5, ''), answer_channel_id),
			published_at = COALESCE($6, published_at),
			updated_at = NOW()
		WHERE id = $7
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
		return nil, fmt.Errorf("Store.UpdateQOTDOfficialPostProgress: %w", err)
	}
	return record, nil
}

// ListQOTDOfficialPostsPendingRecovery lists qotdofficial posts pending recovery.
func (s *Store) ListQOTDOfficialPostsPendingRecovery(ctx context.Context, guildID string) iter.Seq2[QOTDOfficialPostRecord, error] {
	return func(yield func(QOTDOfficialPostRecord, error) bool) {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return
		}

		rows, err := s.db.Query(ctx, `SELECT
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
			WHERE guild_id = $1
			  AND archived_at IS NULL
			  AND state IN ('provisioning', 'failed')
			ORDER BY updated_at ASC, id ASC`,
			guildID,
		)
		if err != nil {
			yield(QOTDOfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsPendingRecovery: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			record, err := scanQOTDOfficialPostRecord(rows)
			if err != nil {
				yield(QOTDOfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsPendingRecovery: %w", err))
				return
			}
			if !yield(*record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(QOTDOfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsPendingRecovery: %w", err))
		}
	}
}

// GetQOTDSurfaceByDeck gets qotdsurface by deck.
func (s *Store) GetQOTDSurfaceByDeck(ctx context.Context, guildID, deckID string) (res *QOTDSurfaceRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get qotd surface by deck: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return nil, nil
	}

	row := s.db.QueryRow(ctx, `SELECT
			id,
			guild_id,
			deck_id,
			channel_id,
			question_list_thread_id,
			created_at,
			updated_at
		FROM qotd_forum_surfaces
		WHERE guild_id = $1 AND deck_id = $2`,
		guildID,
		deckID,
	)
	record, err := scanQOTDSurfaceRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDSurfaceByDeck: %w", err)
	}
	return record, nil
}

// UpsertQOTDSurface upserts qotdsurface.
func (s *Store) UpsertQOTDSurface(ctx context.Context, rec QOTDSurfaceRecord) (res *QOTDSurfaceRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("upsert qotd surface: %w", err)
		}
	}()
	normalized, err := normalizeQOTDSurfaceRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.UpsertQOTDSurface: %w", err)
	}

	row := s.db.QueryRow(ctx, `INSERT INTO qotd_forum_surfaces (
			id,
			guild_id,
			deck_id,
			channel_id,
			question_list_thread_id
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (guild_id, deck_id) DO UPDATE
		SET
			channel_id = EXCLUDED.channel_id,
			question_list_thread_id = EXCLUDED.question_list_thread_id,
			updated_at = NOW()
		RETURNING
			id,
			guild_id,
			deck_id,
			channel_id,
			question_list_thread_id,
			created_at,
			updated_at`,
		idgen.GenerateID(),
		normalized.GuildID,
		normalized.DeckID,
		normalized.ChannelID,
		zeroEmptyString(normalized.QuestionListThreadID),
	)
	updated, err := scanQOTDSurfaceRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.UpsertQOTDSurface: %w", err)
	}
	return updated, nil
}

// DeleteQOTDSurfaceByDeck deletes qotdsurface by deck.
func (s *Store) DeleteQOTDSurfaceByDeck(ctx context.Context, guildID, deckID string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("delete qotd surface by deck: %w", err)
		}
	}()
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return nil
	}

	if _, err := s.db.Exec(ctx,
		`DELETE FROM qotd_forum_surfaces WHERE guild_id = $1 AND deck_id = $2`,
		guildID,
		deckID,
	); err != nil {
		return err
	}
	return nil
}

// CreateQOTDAnswerMessage creates qotdanswer message.
func (s *Store) CreateQOTDAnswerMessage(ctx context.Context, rec QOTDAnswerMessageRecord) (res *QOTDAnswerMessageRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("create qotd answer message: %w", err)
		}
	}()
	normalized, err := normalizeQOTDAnswerMessageRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDAnswerMessage: %w", err)
	}

	row := s.db.QueryRow(ctx, `INSERT INTO qotd_answer_messages (
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			closed_at,
			archived_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		idgen.GenerateID(),
		normalized.GuildID,
		normalized.OfficialPostID,
		normalized.UserID,
		normalized.State,
		normalized.AnswerChannelID,
		zeroEmptyString(normalized.DiscordMessageID),
		zeroEmptyString(normalized.CreatedViaInteractionID),
		nullableTime(normalized.ClosedAt),
		nullableTime(normalized.ArchivedAt),
	)
	created, err := scanQOTDAnswerMessageRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDAnswerMessage: %w", err)
	}
	return created, nil
}

// FinalizeQOTDAnswerMessage finalizes qotdanswer message.
func (s *Store) FinalizeQOTDAnswerMessage(ctx context.Context, id int64, discordMessageID string) (_ *QOTDAnswerMessageRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("finalize qotd answer message: %w", err)
		}
	}()
	if id <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	discordMessageID = strings.TrimSpace(discordMessageID)
	if discordMessageID == "" {
		return nil, fmt.Errorf("discord message id is required")
	}

	row := s.db.QueryRow(ctx, `UPDATE qotd_answer_messages
		SET
			discord_message_id = $1,
			updated_at = NOW()
		WHERE id = $2
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		discordMessageID,
		id,
	)
	record, err := scanQOTDAnswerMessageRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.FinalizeQOTDAnswerMessage: %w", err)
	}
	return record, nil
}

// GetQOTDAnswerMessageByOfficialPostAndUser gets qotdanswer message by official post and user.
func (s *Store) GetQOTDAnswerMessageByOfficialPostAndUser(ctx context.Context, officialPostID int64, userID string) (res *QOTDAnswerMessageRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get qotd answer message by official post and user: %w", err)
		}
	}()
	userID = strings.TrimSpace(userID)
	if officialPostID <= 0 || userID == "" {
		return nil, nil
	}

	row := s.db.QueryRow(ctx, `SELECT
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			created_at,
			updated_at,
			closed_at,
			archived_at
		FROM qotd_answer_messages
		WHERE official_post_id = $1 AND user_id = $2`,
		officialPostID,
		userID,
	)
	record, err := scanQOTDAnswerMessageRecord(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDAnswerMessageByOfficialPostAndUser: %w", err)
	}
	return record, nil
}

// ListQOTDAnswerMessagesByOfficialPost lists qotdanswer messages by official post.
func (s *Store) ListQOTDAnswerMessagesByOfficialPost(ctx context.Context, officialPostID int64) (_ iter.Seq2[QOTDAnswerMessageRecord, error], err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list qotd answer messages by official post: %w", err)
		}
	}()
	if officialPostID <= 0 {
		return nil, nil
	}

	return func(yield func(QOTDAnswerMessageRecord, error) bool) {
		rows, err := s.db.Query(ctx, `SELECT
				id,
				guild_id,
				official_post_id,
				user_id,
				state,
				answer_channel_id,
				discord_message_id,
				created_via_interaction_id,
				created_at,
				updated_at,
				closed_at,
				archived_at
			FROM qotd_answer_messages
			WHERE official_post_id = $1
			ORDER BY id ASC`,
			officialPostID,
		)
		if err != nil {
			yield(QOTDAnswerMessageRecord{}, fmt.Errorf("Store.ListQOTDAnswerMessagesByOfficialPost: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			record, err := scanQOTDAnswerMessageRecord(rows)
			if err != nil {
				yield(QOTDAnswerMessageRecord{}, fmt.Errorf("Store.ListQOTDAnswerMessagesByOfficialPost: %w", err))
				return
			}
			if !yield(*record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(QOTDAnswerMessageRecord{}, fmt.Errorf("Store.ListQOTDAnswerMessagesByOfficialPost: %w", err))
		}
	}, nil

}

// UpdateQOTDAnswerMessageState updates qotdanswer message state.
func (s *Store) UpdateQOTDAnswerMessageState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (_ *QOTDAnswerMessageRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update qotd answer message state: %w", err)
		}
	}()
	if id <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return nil, fmt.Errorf("state is required")
	}

	row := s.db.QueryRow(ctx, `UPDATE qotd_answer_messages
		SET
			state = $1,
			closed_at = $2,
			archived_at = $3,
			updated_at = NOW()
		WHERE id = $4
		RETURNING
			id,
			guild_id,
			official_post_id,
			user_id,
			state,
			answer_channel_id,
			discord_message_id,
			created_via_interaction_id,
			created_at,
			updated_at,
			closed_at,
			archived_at`,
		state,
		nullableTime(closedAt),
		nullableTime(archivedAt),
		id,
	)
	record, err := scanQOTDAnswerMessageRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.UpdateQOTDAnswerMessageState: %w", err)
	}
	return record, nil
}

func normalizeQOTDSurfaceRecord(rec QOTDSurfaceRecord) (QOTDSurfaceRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.ChannelID = strings.TrimSpace(rec.ChannelID)
	rec.QuestionListThreadID = strings.TrimSpace(rec.QuestionListThreadID)

	if rec.GuildID == "" {
		return QOTDSurfaceRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.DeckID == "" {
		return QOTDSurfaceRecord{}, fmt.Errorf("deck_id is required")
	}
	if rec.ChannelID == "" {
		return QOTDSurfaceRecord{}, fmt.Errorf("channel_id is required")
	}
	return rec, nil
}

func normalizeQOTDAnswerMessageRecord(rec QOTDAnswerMessageRecord) (QOTDAnswerMessageRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.UserID = strings.TrimSpace(rec.UserID)
	rec.State = strings.TrimSpace(rec.State)
	rec.AnswerChannelID = strings.TrimSpace(rec.AnswerChannelID)
	rec.DiscordMessageID = strings.TrimSpace(rec.DiscordMessageID)
	rec.CreatedViaInteractionID = strings.TrimSpace(rec.CreatedViaInteractionID)
	rec.ClosedAt = normalizeQOTDTimePtr(rec.ClosedAt)
	rec.ArchivedAt = normalizeQOTDTimePtr(rec.ArchivedAt)

	if rec.State == "" {
		rec.State = "active"
	}
	if rec.GuildID == "" {
		return QOTDAnswerMessageRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.OfficialPostID <= 0 {
		return QOTDAnswerMessageRecord{}, fmt.Errorf("official_post_id is required")
	}
	if rec.UserID == "" {
		return QOTDAnswerMessageRecord{}, fmt.Errorf("user_id is required")
	}
	if rec.AnswerChannelID == "" {
		return QOTDAnswerMessageRecord{}, fmt.Errorf("answer_channel_id is required")
	}
	return rec, nil
}

func scanQOTDSurfaceRecord(scanner qotdRowScanner) (*QOTDSurfaceRecord, error) {
	var record QOTDSurfaceRecord
	var questionListThreadID sql.NullString
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.DeckID,
		&record.ChannelID,
		&questionListThreadID,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.QuestionListThreadID = strings.TrimSpace(questionListThreadID.String)
	return &record, nil
}

func scanQOTDAnswerMessageRecord(scanner qotdRowScanner) (*QOTDAnswerMessageRecord, error) {
	var record QOTDAnswerMessageRecord
	var discordMessageID sql.NullString
	var interactionID sql.NullString
	var closedAt sql.NullTime
	var archivedAt sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.OfficialPostID,
		&record.UserID,
		&record.State,
		&record.AnswerChannelID,
		&discordMessageID,
		&interactionID,
		&record.CreatedAt,
		&record.UpdatedAt,
		&closedAt,
		&archivedAt,
	); err != nil {
		return nil, err
	}
	record.DiscordMessageID = strings.TrimSpace(discordMessageID.String)
	record.CreatedViaInteractionID = strings.TrimSpace(interactionID.String)
	record.ClosedAt = timePtrFromNull(closedAt)
	record.ArchivedAt = timePtrFromNull(archivedAt)
	return &record, nil
}

// GetCurrentAndPreviousQOTDPosts gets current and previous qotdposts.
func (s *Store) GetCurrentAndPreviousQOTDPosts(ctx context.Context, guildID string, now time.Time) iter.Seq2[QOTDOfficialPostRecord, error] {
	return func(yield func(QOTDOfficialPostRecord, error) bool) {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return
		}
		if now.IsZero() {
			now = time.Now().UTC()
		} else {
			now = now.UTC()
		}

		rows, err := s.db.Query(ctx, `SELECT
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
			WHERE guild_id = $1
			  AND published_at IS NOT NULL
			  AND archived_at IS NULL
			  AND archive_at > $2
			ORDER BY publish_date_utc DESC, published_at DESC, id DESC
			LIMIT 2`,
			guildID,
			now,
		)
		if err != nil {
			yield(QOTDOfficialPostRecord{}, fmt.Errorf("Store.GetCurrentAndPreviousQOTDPosts: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			record, err := scanQOTDOfficialPostRecord(rows)
			if err != nil {
				yield(QOTDOfficialPostRecord{}, fmt.Errorf("Store.GetCurrentAndPreviousQOTDPosts: %w", err))
				return
			}
			if !yield(*record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(QOTDOfficialPostRecord{}, fmt.Errorf("Store.GetCurrentAndPreviousQOTDPosts: %w", err))
		}
	}
}

// ListQOTDOfficialPostsNeedingArchive lists qotdofficial posts needing archive.
func (s *Store) ListQOTDOfficialPostsNeedingArchive(ctx context.Context, now time.Time) iter.Seq2[QOTDOfficialPostRecord, error] {
	return func(yield func(QOTDOfficialPostRecord, error) bool) {
		if now.IsZero() {
			now = time.Now().UTC()
		} else {
			now = now.UTC()
		}

		rows, err := s.db.Query(ctx, `SELECT
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
			  AND archive_at <= $1
			ORDER BY archive_at ASC, id ASC`,
			now,
		)
		if err != nil {
			yield(QOTDOfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsNeedingArchive: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			record, err := scanQOTDOfficialPostRecord(rows)
			if err != nil {
				yield(QOTDOfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsNeedingArchive: %w", err))
				return
			}
			if !yield(*record, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(QOTDOfficialPostRecord{}, fmt.Errorf("Store.ListQOTDOfficialPostsNeedingArchive: %w", err))
		}
	}
}

// UpdateQOTDOfficialPostState updates qotdofficial post state.
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

	row := s.db.QueryRow(ctx, `UPDATE qotd_official_posts
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
		return nil, fmt.Errorf("Store.UpdateQOTDOfficialPostState: %w", err)
	}
	return record, nil
}
