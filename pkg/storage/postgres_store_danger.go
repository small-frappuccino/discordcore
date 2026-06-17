package storage

import (
	"context"
	"fmt"
)

// PurgeGuildModerationData drops all moderation warnings and resets the case counter.
func (s *Store) PurgeGuildModerationData(ctx context.Context, guildID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM moderation_warnings WHERE guild_id = $1`, guildID); err != nil {
		return fmt.Errorf("delete moderation_warnings: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM moderation_cases WHERE guild_id = $1`, guildID); err != nil {
		return fmt.Errorf("delete moderation_cases: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// PurgeGuildQOTDData drops all QOTD questions, posts, and answers.
func (s *Store) PurgeGuildQOTDData(ctx context.Context, guildID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Due to FK constraints, delete in correct order
	if _, err := tx.Exec(ctx, `
		DELETE FROM qotd_answer_messages 
		WHERE official_post_id IN (SELECT id FROM qotd_official_posts WHERE guild_id = $1)
	`, guildID); err != nil {
		return fmt.Errorf("delete qotd_answer_messages: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM qotd_official_posts WHERE guild_id = $1`, guildID); err != nil {
		return fmt.Errorf("delete qotd_official_posts: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM qotd_questions WHERE guild_id = $1`, guildID); err != nil {
		return fmt.Errorf("delete qotd_questions: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM qotd_forum_surfaces WHERE guild_id = $1`, guildID); err != nil {
		return fmt.Errorf("delete qotd_forum_surfaces: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// PurgeGuildEngagementMetrics drops daily metrics (messages, reactions, joins, leaves).
func (s *Store) PurgeGuildEngagementMetrics(ctx context.Context, guildID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tables := []string{
		"daily_message_metrics",
		"daily_reaction_metrics",
		"daily_member_joins",
		"daily_member_leaves",
	}

	for _, table := range tables {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE guild_id = $1`, table), guildID); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// PurgeGuildCache drops transient state data.
func (s *Store) PurgeGuildCache(ctx context.Context, guildID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tables := []string{
		"persistent_cache",
		"avatars_history",
		"messages_history",
	}

	for _, table := range tables {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE guild_id = $1`, table), guildID); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// WipeGuildCompletely drops ALL data related to the guild.
func (s *Store) WipeGuildCompletely(ctx context.Context, guildID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Due to FKs, delete in correct order (QOTD answers, then posts, etc)
	if _, err := tx.Exec(ctx, `
		DELETE FROM qotd_answer_messages 
		WHERE official_post_id IN (SELECT id FROM qotd_official_posts WHERE guild_id = $1)
	`, guildID); err != nil {
		return fmt.Errorf("delete qotd_answer_messages: %w", err)
	}

	// All other tables that rely on guild_id
	tables := []string{
		"qotd_official_posts",
		"qotd_questions",
		"qotd_forum_surfaces",
		"ticket_sequences",
		"persistent_cache",
		"roles_current",
		"moderation_warnings",
		"moderation_cases",
		"daily_member_leaves",
		"daily_member_joins",
		"daily_reaction_metrics",
		"daily_message_metrics",
		"messages_history",
		"avatars_history",
		"avatars_current",
		"message_version_counters",
		"member_joins",
		"messages",
		"guild_configs",
		"guild_meta",
	}

	for _, table := range tables {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE guild_id = $1`, table), guildID); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
