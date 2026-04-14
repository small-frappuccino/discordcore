package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *Store) GetQOTDForumSurfaceByDeck(ctx context.Context, guildID, deckID string) (*QOTDForumSurfaceRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	if guildID == "" || deckID == "" {
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
			forum_channel_id,
			question_list_thread_id,
			created_at,
			updated_at
		FROM qotd_forum_surfaces
		WHERE guild_id = ? AND deck_id = ?`,
		guildID,
		deckID,
	)
	record, err := scanQOTDForumSurfaceRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get qotd forum surface by deck: %w", err)
	}
	return record, nil
}

func (s *Store) UpsertQOTDForumSurface(ctx context.Context, rec QOTDForumSurfaceRecord) (*QOTDForumSurfaceRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	normalized, err := normalizeQOTDForumSurfaceRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("upsert qotd forum surface: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := s.queryRowContext(ctx,
		`INSERT INTO qotd_forum_surfaces (
			guild_id,
			deck_id,
			forum_channel_id,
			question_list_thread_id
		)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (guild_id, deck_id) DO UPDATE
		SET
			forum_channel_id = EXCLUDED.forum_channel_id,
			question_list_thread_id = EXCLUDED.question_list_thread_id,
			updated_at = NOW()
		RETURNING
			id,
			guild_id,
			deck_id,
			forum_channel_id,
			question_list_thread_id,
			created_at,
			updated_at`,
		normalized.GuildID,
		normalized.DeckID,
		normalized.ForumChannelID,
		zeroEmptyString(normalized.QuestionListThreadID),
	)
	updated, err := scanQOTDForumSurfaceRecord(row)
	if err != nil {
		return nil, fmt.Errorf("upsert qotd forum surface: %w", err)
	}
	return updated, nil
}

func (s *Store) CreateQOTDAnswerMessage(ctx context.Context, rec QOTDAnswerMessageRecord) (*QOTDAnswerMessageRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	normalized, err := normalizeQOTDAnswerMessageRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("create qotd answer message: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		return nil, fmt.Errorf("create qotd answer message: %w", err)
	}
	return created, nil
}

func (s *Store) FinalizeQOTDAnswerMessage(ctx context.Context, id int64, discordMessageID string) (*QOTDAnswerMessageRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("finalize qotd answer message: id is required")
	}
	discordMessageID = strings.TrimSpace(discordMessageID)
	if discordMessageID == "" {
		return nil, fmt.Errorf("finalize qotd answer message: discord message id is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := s.queryRowContext(ctx,
		`UPDATE qotd_answer_messages
		SET
			discord_message_id = ?,
			updated_at = NOW()
		WHERE id = ?
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
		return nil, fmt.Errorf("finalize qotd answer message: %w", err)
	}
	return record, nil
}

func (s *Store) GetQOTDAnswerMessageByOfficialPostAndUser(ctx context.Context, officialPostID int64, userID string) (*QOTDAnswerMessageRecord, error) {
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
		return nil, fmt.Errorf("get qotd answer message by official post and user: %w", err)
	}
	return record, nil
}

func (s *Store) ListQOTDAnswerMessagesByOfficialPost(ctx context.Context, officialPostID int64) ([]QOTDAnswerMessageRecord, error) {
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
		return nil, fmt.Errorf("list qotd answer messages by official post: %w", err)
	}
	defer rows.Close()

	records := make([]QOTDAnswerMessageRecord, 0, 16)
	for rows.Next() {
		record, err := scanQOTDAnswerMessageRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list qotd answer messages by official post: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list qotd answer messages by official post: %w", err)
	}
	return records, nil
}

func (s *Store) UpdateQOTDAnswerMessageState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (*QOTDAnswerMessageRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("update qotd answer message state: id is required")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return nil, fmt.Errorf("update qotd answer message state: state is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := s.queryRowContext(ctx,
		`UPDATE qotd_answer_messages
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
		return nil, fmt.Errorf("update qotd answer message state: %w", err)
	}
	return record, nil
}

func normalizeQOTDForumSurfaceRecord(rec QOTDForumSurfaceRecord) (QOTDForumSurfaceRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.ForumChannelID = strings.TrimSpace(rec.ForumChannelID)
	rec.QuestionListThreadID = strings.TrimSpace(rec.QuestionListThreadID)

	if rec.GuildID == "" {
		return QOTDForumSurfaceRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.DeckID == "" {
		return QOTDForumSurfaceRecord{}, fmt.Errorf("deck_id is required")
	}
	if rec.ForumChannelID == "" {
		return QOTDForumSurfaceRecord{}, fmt.Errorf("forum_channel_id is required")
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

func scanQOTDForumSurfaceRecord(scanner qotdRowScanner) (*QOTDForumSurfaceRecord, error) {
	var record QOTDForumSurfaceRecord
	var questionListThreadID sql.NullString
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.DeckID,
		&record.ForumChannelID,
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
