package persistence

import (
	"context"
	"database/sql"
	"fmt"
)

func repairQOTDLegacySchema(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return fmt.Errorf("repair qotd legacy schema: transaction is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	hasOfficialPosts, err := tableExists(ctx, tx, "qotd_official_posts")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check official posts table: %w", err)
	}
	if !hasOfficialPosts {
		return nil
	}

	hasForumChannelID, err := columnExists(ctx, tx, "qotd_official_posts", "forum_channel_id")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check official posts forum channel column: %w", err)
	}
	hasChannelID, err := columnExists(ctx, tx, "qotd_official_posts", "channel_id")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check official posts channel column: %w", err)
	}
	hasDiscordThreadID, err := columnExists(ctx, tx, "qotd_official_posts", "discord_thread_id")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check official posts discord thread column: %w", err)
	}
	hasAnswerChannelID, err := columnExists(ctx, tx, "qotd_official_posts", "answer_channel_id")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check official posts answer channel column: %w", err)
	}
	hasResponseChannelSnapshot, err := columnExists(ctx, tx, "qotd_official_posts", "response_channel_id_snapshot")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check response channel snapshot column: %w", err)
	}
	channelColumn := "channel_id"
	if !hasChannelID {
		channelColumn = "forum_channel_id"
	}
	if (hasForumChannelID || hasChannelID) && hasDiscordThreadID && hasAnswerChannelID {
		if hasResponseChannelSnapshot {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE qotd_official_posts
SET answer_channel_id = COALESCE(
	NULLIF(discord_thread_id, ''),
	NULLIF(response_channel_id_snapshot, ''),
	NULLIF(answer_channel_id, ''),
	%s
)
`, channelColumn)); err != nil {
				return fmt.Errorf("repair qotd legacy schema: backfill answer channel from snapshot: %w", err)
			}
		} else {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE qotd_official_posts
SET answer_channel_id = COALESCE(
	NULLIF(discord_thread_id, ''),
	NULLIF(answer_channel_id, ''),
	%s
)
`, channelColumn)); err != nil {
				return fmt.Errorf("repair qotd legacy schema: backfill answer channel: %w", err)
			}
		}
	}

	if hasForumChannelID && !hasChannelID {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE qotd_official_posts RENAME COLUMN forum_channel_id TO channel_id`); err != nil {
			return fmt.Errorf("repair qotd legacy schema: rename official posts forum channel column: %w", err)
		}
		hasChannelID = true
	}

	hasLegacyReplyThreads, err := tableExists(ctx, tx, "qotd_reply_threads")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check legacy reply threads table: %w", err)
	}
	hasAnswerMessages, err := tableExists(ctx, tx, "qotd_answer_messages")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check answer messages table: %w", err)
	}
	if hasLegacyReplyThreads && hasAnswerMessages {
		replyThreadChannelColumn := "channel_id"
		if !hasChannelID {
			replyThreadChannelColumn = "forum_channel_id"
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO qotd_answer_messages (
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
)
SELECT
	guild_id,
	official_post_id,
	user_id,
	state,
	%s,
	discord_starter_message_id,
	created_via_interaction_id,
	created_at,
	updated_at,
	closed_at,
	archived_at
FROM qotd_reply_threads
ON CONFLICT (official_post_id, user_id) DO NOTHING
`, replyThreadChannelColumn)); err != nil {
			return fmt.Errorf("repair qotd legacy schema: migrate legacy reply threads: %w", err)
		}
	}

	hasThreadArchives, err := tableExists(ctx, tx, "qotd_thread_archives")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check thread archives table: %w", err)
	}
	if hasThreadArchives {
		hasReplyThreadConstraint, err := constraintExists(ctx, tx, "qotd_thread_archives", "qotd_thread_archives_reply_thread_id_fkey")
		if err != nil {
			return fmt.Errorf("repair qotd legacy schema: check reply thread foreign key: %w", err)
		}
		if hasReplyThreadConstraint {
			if _, err := tx.ExecContext(ctx, `
ALTER TABLE qotd_thread_archives
DROP CONSTRAINT qotd_thread_archives_reply_thread_id_fkey
`); err != nil {
				return fmt.Errorf("repair qotd legacy schema: drop reply thread foreign key: %w", err)
			}
		}

		hasReplyThreadID, err := columnExists(ctx, tx, "qotd_thread_archives", "reply_thread_id")
		if err != nil {
			return fmt.Errorf("repair qotd legacy schema: check reply thread id column: %w", err)
		}
		if hasReplyThreadID {
			if _, err := tx.ExecContext(ctx, `
ALTER TABLE qotd_thread_archives
DROP COLUMN reply_thread_id
`); err != nil {
				return fmt.Errorf("repair qotd legacy schema: drop reply thread id column: %w", err)
			}
		}
	}

	for _, indexName := range []string{
		"idx_qotd_reply_threads_provisioning_recovery",
		"idx_qotd_reply_threads_state",
		"idx_qotd_reply_threads_thread",
		"idx_qotd_reply_threads_unique_user",
	} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP INDEX IF EXISTS %s`, indexName)); err != nil {
			return fmt.Errorf("repair qotd legacy schema: drop legacy reply thread index %s: %w", indexName, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS qotd_reply_threads`); err != nil {
		return fmt.Errorf("repair qotd legacy schema: drop legacy reply threads table: %w", err)
	}

	if hasResponseChannelSnapshot {
		if _, err := tx.ExecContext(ctx, `
ALTER TABLE qotd_official_posts
DROP COLUMN response_channel_id_snapshot
`); err != nil {
			return fmt.Errorf("repair qotd legacy schema: drop response channel snapshot column: %w", err)
		}
	}

	hasPinned, err := columnExists(ctx, tx, "qotd_official_posts", "is_pinned")
	if err != nil {
		return fmt.Errorf("repair qotd legacy schema: check is_pinned column: %w", err)
	}
	if hasPinned {
		if _, err := tx.ExecContext(ctx, `
ALTER TABLE qotd_official_posts
DROP COLUMN is_pinned
`); err != nil {
			return fmt.Errorf("repair qotd legacy schema: drop is_pinned column: %w", err)
		}
	}

	return nil
}

func tableExists(ctx context.Context, tx *sql.Tx, tableName string) (bool, error) {
	var exists bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS(
	SELECT 1
	FROM information_schema.tables
	WHERE table_schema = current_schema()
	  AND table_name = $1
)`, tableName).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func columnExists(ctx context.Context, tx *sql.Tx, tableName, columnName string) (bool, error) {
	var exists bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS(
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = current_schema()
	  AND table_name = $1
	  AND column_name = $2
)`, tableName, columnName).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func constraintExists(ctx context.Context, tx *sql.Tx, tableName, constraintName string) (bool, error) {
	var exists bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS(
	SELECT 1
	FROM information_schema.table_constraints
	WHERE table_schema = current_schema()
	  AND table_name = $1
	  AND constraint_name = $2
)`, tableName, constraintName).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
