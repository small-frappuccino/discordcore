package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *Store) CreateQOTDOfficialPostProvisioning(ctx context.Context, rec QOTDOfficialPostRecord) (res *QOTDOfficialPostRecord, err error) {
	normalized, err := normalizeQOTDOfficialPostRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("Store.CreateQOTDOfficialPostProvisioning: %w", err)
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
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			COALESCE((SELECT MAX(publish_ordinal) FROM qotd_official_posts WHERE guild_id = $17 AND deck_id = $18), 0) + 1,
			$19, $20, $21, $22, $23, $24
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
		return nil, fmt.Errorf("Store.CreateQOTDOfficialPostProvisioning: %w", err)
	}
	return created, nil
}

func (s *Store) FinalizeQOTDOfficialPost(ctx context.Context, id int64, questionListThreadID, questionListEntryMessageID, discordThreadID, starterMessageID, answerChannelID string, publishedAt time.Time) (_ *QOTDOfficialPostRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("finalize qotd official post: %w", err)
		}
	}()
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
		publishedAt.UTC(),
		id,
	)
	updated, err := scanQOTDOfficialPostRecord(row)
	if err != nil {
		return nil, fmt.Errorf("Store.FinalizeQOTDOfficialPost: %w", err)
	}
	return updated, nil
}

func (s *Store) GetQOTDOfficialPostByID(ctx context.Context, id int64) (res *QOTDOfficialPostRecord, err error) {
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
		return nil, fmt.Errorf("Store.GetQOTDOfficialPostByID: %w", err)
	}
	return record, nil
}

func (s *Store) GetQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (res *QOTDOfficialPostRecord, err error) {
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
		return nil, fmt.Errorf("Store.GetQOTDOfficialPostByDate: %w", err)
	}
	return record, nil
}

func (s *Store) ListQOTDOfficialPostsByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (_ []QOTDOfficialPostRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list qotd official posts by date: %w", err)
		}
	}()
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
		return nil, fmt.Errorf("Store.ListQOTDOfficialPostsByDate: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDOfficialPostRecord, 0, 2)
	for rows.Next() {
		record, err := scanQOTDOfficialPostRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("Store.ListQOTDOfficialPostsByDate: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Store.ListQOTDOfficialPostsByDate: %w", err)
	}
	return records, nil
}

func (s *Store) GetAutomaticSlotQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (res *QOTDOfficialPostRecord, err error) {
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
		return nil, fmt.Errorf("Store.GetAutomaticSlotQOTDOfficialPostByDate: %w", err)
	}
	return record, nil
}

func (s *Store) GetScheduledQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (res *QOTDOfficialPostRecord, err error) {
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
		return nil, fmt.Errorf("Store.GetScheduledQOTDOfficialPostByDate: %w", err)
	}
	return record, nil
}
func (s *Store) DeleteQOTDOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (count int, err error) {
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return 0, nil
	}

	result, err := s.execContext(ctx,
		`DELETE FROM qotd_official_posts WHERE guild_id = $1 AND deck_id = $2`,
		guildID,
		deckID,
	)
	if err != nil {
		return 0, fmt.Errorf("Store.DeleteQOTDOfficialPostsByDeck: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("Store.DeleteQOTDOfficialPostsByDeck: %w", err)
	}
	return int(deleted), nil
}

func (s *Store) DeleteQOTDOfficialPostByID(ctx context.Context, id int64) (err error) {
	if id <= 0 {
		return nil
	}

	if _, err := s.execContext(ctx, `DELETE FROM qotd_official_posts WHERE id = $1`, id); err != nil {
		return fmt.Errorf("Store.DeleteQOTDOfficialPostByID: %w", err)
	}
	return nil
}

func (s *Store) DeleteQOTDUnpublishedOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (count int, err error) {
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
		return 0, nil
	}

	result, err := s.execContext(ctx,
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
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("Store.DeleteQOTDUnpublishedOfficialPostsByDeck: %w", err)
	}
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
