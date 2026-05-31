package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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

	row := s.queryRowContext(ctx,
		`SELECT
			id,
			guild_id,
			deck_id,
			channel_id,
			question_list_thread_id,
			created_at,
			updated_at
		FROM qotd_forum_surfaces
		WHERE guild_id = ? AND deck_id = ?`,
		guildID,
		deckID,
	)
	record, err := scanQOTDSurfaceRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDSurfaceByDeck: %w", err)
	}
	return record, nil
}

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

	row := s.queryRowContext(ctx,
		`INSERT INTO qotd_forum_surfaces (
			guild_id,
			deck_id,
			channel_id,
			question_list_thread_id
		)
		VALUES ($1, $2, $3, $4)
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

	if _, err := s.execContext(ctx,
		`DELETE FROM qotd_forum_surfaces WHERE guild_id = $1 AND deck_id = $2`,
		guildID,
		deckID,
	); err != nil {
		return err
	}
	return nil
}

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

	row := s.queryRowContext(ctx,
		`INSERT INTO qotd_answer_messages (
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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

	row := s.queryRowContext(ctx,
		`UPDATE qotd_answer_messages
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

	row := s.queryRowContext(ctx,
		`SELECT
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
		WHERE official_post_id = ? AND user_id = ?`,
		officialPostID,
		userID,
	)
	record, err := scanQOTDAnswerMessageRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Store.GetQOTDAnswerMessageByOfficialPostAndUser: %w", err)
	}
	return record, nil
}

func (s *Store) ListQOTDAnswerMessagesByOfficialPost(ctx context.Context, officialPostID int64) (_ []QOTDAnswerMessageRecord, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list qotd answer messages by official post: %w", err)
		}
	}()
	if officialPostID <= 0 {
		return nil, nil
	}

	rows, err := s.queryContext(ctx,
		`SELECT
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
		WHERE official_post_id = ?
		ORDER BY id ASC`,
		officialPostID,
	)
	if err != nil {
		return nil, fmt.Errorf("Store.ListQOTDAnswerMessagesByOfficialPost: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDAnswerMessageRecord, 0, 16)
	for rows.Next() {
		record, err := scanQOTDAnswerMessageRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("Store.ListQOTDAnswerMessagesByOfficialPost: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Store.ListQOTDAnswerMessagesByOfficialPost: %w", err)
	}
	return records, nil
}

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

	row := s.queryRowContext(ctx,
		`UPDATE qotd_answer_messages
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
