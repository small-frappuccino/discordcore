package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/errutil"
)

func repairQOTDLegacySchema(ctx context.Context, tx *sql.Tx) (err error) {
	defer func() { err = errutil.Wrap(err, "repair qotd legacy schema") }()
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	hasOfficialPosts, err := tableExists(ctx, tx, "qotd_official_posts")
	if err != nil {
		return fmt.Errorf("check official posts table: %w", err)
	}
	if !hasOfficialPosts {
		return nil
	}

	hasLegacyChannelColumn, err := columnExists(ctx, tx, "qotd_official_posts", "forum_channel_id")
	if err != nil {
		return fmt.Errorf("check official posts legacy channel column: %w", err)
	}
	hasChannelID, err := columnExists(ctx, tx, "qotd_official_posts", "channel_id")
	if err != nil {
		return fmt.Errorf("check official posts channel column: %w", err)
	}
	hasDiscordThreadID, err := columnExists(ctx, tx, "qotd_official_posts", "discord_thread_id")
	if err != nil {
		return fmt.Errorf("check official posts discord thread column: %w", err)
	}
	hasAnswerChannelID, err := columnExists(ctx, tx, "qotd_official_posts", "answer_channel_id")
	if err != nil {
		return fmt.Errorf("check official posts answer channel column: %w", err)
	}
	hasResponseChannelSnapshot, err := columnExists(ctx, tx, "qotd_official_posts", "response_channel_id_snapshot")
	if err != nil {
		return fmt.Errorf("check response channel snapshot column: %w", err)
	}
	channelColumn := "channel_id"
	if !hasChannelID {
		channelColumn = "forum_channel_id"
	}
	if (hasLegacyChannelColumn || hasChannelID) && hasDiscordThreadID && hasAnswerChannelID {
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
				return fmt.Errorf("backfill answer channel from snapshot: %w", err)
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
				return fmt.Errorf("backfill answer channel: %w", err)
			}
		}
	}

	if hasLegacyChannelColumn && !hasChannelID {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE qotd_official_posts RENAME COLUMN forum_channel_id TO channel_id`); err != nil {
			return fmt.Errorf("rename official posts legacy channel column: %w", err)
		}
		hasChannelID = true
	}

	hasForumSurfaces, err := tableExists(ctx, tx, "qotd_forum_surfaces")
	if err != nil {
		return fmt.Errorf("check qotd surfaces table: %w", err)
	}
	if hasForumSurfaces {
		hasLegacySurfaceChannelColumn, err := columnExists(ctx, tx, "qotd_forum_surfaces", "forum_channel_id")
		if err != nil {
			return fmt.Errorf("check qotd surfaces legacy channel column: %w", err)
		}
		hasSurfaceChannelID, err := columnExists(ctx, tx, "qotd_forum_surfaces", "channel_id")
		if err != nil {
			return fmt.Errorf("check qotd surfaces channel column: %w", err)
		}
		switch {
		case hasLegacySurfaceChannelColumn && hasSurfaceChannelID:
			if _, err := tx.ExecContext(ctx, `
UPDATE qotd_forum_surfaces
SET channel_id = COALESCE(NULLIF(channel_id, ''), forum_channel_id)
WHERE channel_id IS NULL OR channel_id = ''
`); err != nil {
				return fmt.Errorf("backfill qotd surfaces channel column: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `
ALTER TABLE qotd_forum_surfaces
DROP COLUMN forum_channel_id
`); err != nil {
				return fmt.Errorf("drop qotd surfaces legacy channel column: %w", err)
			}
		case hasLegacySurfaceChannelColumn:
			if _, err := tx.ExecContext(ctx, `ALTER TABLE qotd_forum_surfaces RENAME COLUMN forum_channel_id TO channel_id`); err != nil {
				return fmt.Errorf("rename qotd surfaces legacy channel column: %w", err)
			}
		}
	}

	hasLegacyReplyThreads, err := tableExists(ctx, tx, "qotd_reply_threads")
	if err != nil {
		return fmt.Errorf("check legacy reply threads table: %w", err)
	}
	hasAnswerMessages, err := tableExists(ctx, tx, "qotd_answer_messages")
	if err != nil {
		return fmt.Errorf("check answer messages table: %w", err)
	}
	if hasLegacyReplyThreads && hasAnswerMessages {
		hasLegacyReplyThreadChannel, err := columnExists(ctx, tx, "qotd_reply_threads", "forum_channel_id")
		if err != nil {
			return fmt.Errorf("check legacy reply thread forum channel column: %w", err)
		}
		hasReplyThreadChannelID, err := columnExists(ctx, tx, "qotd_reply_threads", "channel_id")
		if err != nil {
			return fmt.Errorf("check legacy reply thread channel column: %w", err)
		}

		replyThreadChannelColumn := ""
		switch {
		case hasReplyThreadChannelID:
			replyThreadChannelColumn = "channel_id"
		case hasLegacyReplyThreadChannel:
			replyThreadChannelColumn = "forum_channel_id"
		default:
			return fmt.Errorf("qotd_reply_threads missing channel column")
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
			return fmt.Errorf("migrate legacy reply threads: %w", err)
		}
	}

	for _, indexName := range []string{
		"idx_qotd_reply_threads_provisioning_recovery",
		"idx_qotd_reply_threads_state",
		"idx_qotd_reply_threads_thread",
		"idx_qotd_reply_threads_unique_user",
	} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP INDEX IF EXISTS %s`, indexName)); err != nil {
			return fmt.Errorf("drop legacy reply thread index %s: %w", indexName, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS qotd_reply_threads`); err != nil {
		return fmt.Errorf("drop legacy reply threads table: %w", err)
	}

	if hasResponseChannelSnapshot {
		if _, err := tx.ExecContext(ctx, `
ALTER TABLE qotd_official_posts
DROP COLUMN response_channel_id_snapshot
`); err != nil {
			return fmt.Errorf("drop response channel snapshot column: %w", err)
		}
	}

	hasPinned, err := columnExists(ctx, tx, "qotd_official_posts", "is_pinned")
	if err != nil {
		return fmt.Errorf("check is_pinned column: %w", err)
	}
	if hasPinned {
		if _, err := tx.ExecContext(ctx, `
ALTER TABLE qotd_official_posts
DROP COLUMN is_pinned
`); err != nil {
			return fmt.Errorf("drop is_pinned column: %w", err)
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
