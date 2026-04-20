package storage

import (
	"context"
	"fmt"
	"strings"
)

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
