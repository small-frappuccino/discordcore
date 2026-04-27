package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

var guildScopedDeleteTables = []string{
	"qotd_answer_messages",
	"qotd_thread_archives",
	"qotd_official_posts",
	"qotd_questions",
	"qotd_forum_surfaces",
	"qotd_collected_questions",
	"moderation_warnings",
	"moderation_cases",
	"daily_message_metrics",
	"daily_reaction_metrics",
	"daily_member_joins",
	"daily_member_leaves",
	"message_version_counters",
	"messages_history",
	"messages",
	"roles_current",
	"avatars_history",
	"avatars_current",
	"member_joins",
	"guild_meta",
}

func (s *Store) DeleteGuildData(ctx context.Context, guildID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return fmt.Errorf("delete guild data: guild_id is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete guild data: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, tableName := range guildScopedDeleteTables {
		if _, err := txExecContext(ctx, tx, fmt.Sprintf("DELETE FROM %s WHERE guild_id=?", tableName), guildID); err != nil {
			return fmt.Errorf("delete guild data: delete from %s: %w", tableName, err)
		}
	}
	if err := deleteGuildPersistentCacheTx(ctx, tx, guildID); err != nil {
		return fmt.Errorf("delete guild data: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete guild data: %w", err)
	}
	return nil
}

func deleteGuildPersistentCacheTx(ctx context.Context, tx *sql.Tx, guildID string) error {
	memberPrefix := guildID + ":"
	channelPrefix := guildID + ":"
	deletes := []struct {
		query string
		args  []any
	}{
		{query: `DELETE FROM persistent_cache WHERE cache_type=? AND cache_key LIKE ?`, args: []any{"member", memberPrefix + "%"}},
		{query: `DELETE FROM persistent_cache WHERE cache_type=? AND cache_key LIKE ?`, args: []any{"member", "member:" + memberPrefix + "%"}},
		{query: `DELETE FROM persistent_cache WHERE cache_key=?`, args: []any{guildID}},
		{query: `DELETE FROM persistent_cache WHERE cache_key=?`, args: []any{"guild:" + guildID}},
		{query: `DELETE FROM persistent_cache WHERE cache_key=?`, args: []any{"roles:" + guildID}},
		{query: `DELETE FROM persistent_cache WHERE cache_type=? AND cache_key LIKE ?`, args: []any{"channel", channelPrefix + "%"}},
		{query: `DELETE FROM persistent_cache WHERE cache_type=? AND cache_key LIKE ?`, args: []any{"channel", "channel:" + channelPrefix + "%"}},
	}

	for _, deleteStmt := range deletes {
		if _, err := txExecContext(ctx, tx, deleteStmt.query, deleteStmt.args...); err != nil {
			return err
		}
	}
	return nil
}