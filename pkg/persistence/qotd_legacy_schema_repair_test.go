package persistence_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestPostgresMigratorUpRepairsLegacyQOTDSchemaFromVersion10(t *testing.T) {
	t.Parallel()

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve test database dsn: %v", err)
	}

	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	migrator := persistence.NewPostgresMigrator(db)
	if err := migrator.Down(context.Background(), 4); err != nil {
		t.Fatalf("rollback to version 10: %v", err)
	}

	version, err := migrator.Version(context.Background())
	if err != nil {
		t.Fatalf("read schema version after rollback: %v", err)
	}
	if version != 10 {
		t.Fatalf("expected schema version 10 after rollback, got %d", version)
	}

	if _, err := db.ExecContext(context.Background(), `
ALTER TABLE qotd_official_posts ADD COLUMN response_channel_id_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE qotd_official_posts ADD COLUMN is_pinned BOOLEAN NOT NULL DEFAULT FALSE;
CREATE TABLE qotd_reply_threads (
	id                         BIGSERIAL PRIMARY KEY,
	guild_id                   TEXT NOT NULL,
	official_post_id           BIGINT NOT NULL REFERENCES qotd_official_posts(id) ON DELETE CASCADE,
	user_id                    TEXT NOT NULL,
	state                      TEXT NOT NULL,
	forum_channel_id           TEXT NOT NULL,
	discord_thread_id          TEXT,
	discord_starter_message_id TEXT,
	created_via_interaction_id TEXT,
	provisioning_nonce         TEXT,
	created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	closed_at                  TIMESTAMPTZ,
	archived_at                TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_qotd_reply_threads_unique_user ON qotd_reply_threads(official_post_id, user_id);
CREATE UNIQUE INDEX idx_qotd_reply_threads_thread ON qotd_reply_threads(discord_thread_id) WHERE discord_thread_id IS NOT NULL;
CREATE INDEX idx_qotd_reply_threads_state ON qotd_reply_threads(official_post_id, state, created_at);
CREATE INDEX idx_qotd_reply_threads_provisioning_recovery ON qotd_reply_threads(guild_id, state, updated_at) WHERE discord_thread_id IS NULL;
ALTER TABLE qotd_thread_archives ADD COLUMN reply_thread_id BIGINT;
ALTER TABLE qotd_thread_archives
	ADD CONSTRAINT qotd_thread_archives_reply_thread_id_fkey
	FOREIGN KEY (reply_thread_id) REFERENCES qotd_reply_threads(id) ON DELETE CASCADE;
`); err != nil {
		t.Fatalf("recreate legacy qotd artifacts: %v", err)
	}

	var questionID int64
	if err := db.QueryRowContext(context.Background(), `
INSERT INTO qotd_questions (
	guild_id,
	deck_id,
	body,
	status,
	queue_position
) VALUES ('g1', 'default', 'Legacy question', 'used', 1)
RETURNING id
`).Scan(&questionID); err != nil {
		t.Fatalf("insert question: %v", err)
	}

	var officialPostID int64
	if err := db.QueryRowContext(context.Background(), `
INSERT INTO qotd_official_posts (
	guild_id,
	question_id,
	publish_date_utc,
	state,
	forum_channel_id,
	discord_thread_id,
	discord_starter_message_id,
	question_text_snapshot,
	published_at,
	grace_until,
	archive_at,
	deck_id,
	deck_name_snapshot,
	publish_mode,
	response_channel_id_snapshot,
	is_pinned
) VALUES (
	'g1',
	$1,
	DATE '2026-04-03',
	'current',
	'forum-legacy',
	'',
	'starter-legacy',
	'Legacy question',
	NOW(),
	NOW(),
	NOW(),
	'default',
	'Default',
	'scheduled',
	'response-legacy',
	TRUE
)
RETURNING id
`, questionID).Scan(&officialPostID); err != nil {
		t.Fatalf("insert official post: %v", err)
	}

	var replyThreadID int64
	if err := db.QueryRowContext(context.Background(), `
INSERT INTO qotd_reply_threads (
	guild_id,
	official_post_id,
	user_id,
	state,
	forum_channel_id,
	discord_thread_id,
	discord_starter_message_id,
	created_via_interaction_id
) VALUES (
	'g1',
	$1,
	'user-1',
	'active',
	'legacy-answer-channel',
	'',
	'legacy-answer-message',
	'interaction-1'
)
RETURNING id
`, officialPostID).Scan(&replyThreadID); err != nil {
		t.Fatalf("insert legacy reply thread: %v", err)
	}

	if _, err := db.ExecContext(context.Background(), `
INSERT INTO qotd_thread_archives (
	guild_id,
	official_post_id,
	reply_thread_id,
	source_kind,
	discord_thread_id,
	archived_at
) VALUES (
	'g1',
	$1,
	$2,
	'answer',
	'archived-thread-1',
	NOW()
)
`, officialPostID, replyThreadID); err != nil {
		t.Fatalf("insert thread archive: %v", err)
	}

	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("upgrade with legacy repair: %v", err)
	}

	version, err = migrator.Version(context.Background())
	if err != nil {
		t.Fatalf("read schema version after upgrade: %v", err)
	}
	if version != 14 {
		t.Fatalf("expected schema version 14 after upgrade, got %d", version)
	}

	var answerChannelID string
	if err := db.QueryRowContext(context.Background(), `
SELECT answer_channel_id
FROM qotd_official_posts
WHERE id = $1
`, officialPostID).Scan(&answerChannelID); err != nil {
		t.Fatalf("read repaired official post answer channel: %v", err)
	}
	if answerChannelID != "response-legacy" {
		t.Fatalf("expected answer_channel_id to prefer legacy response snapshot, got %q", answerChannelID)
	}

	var migratedMessages int
	if err := db.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM qotd_answer_messages
WHERE official_post_id = $1
  AND user_id = 'user-1'
  AND answer_channel_id = 'legacy-answer-channel'
  AND discord_message_id = 'legacy-answer-message'
`, officialPostID).Scan(&migratedMessages); err != nil {
		t.Fatalf("count migrated answer messages: %v", err)
	}
	if migratedMessages != 1 {
		t.Fatalf("expected one migrated answer message, got %d", migratedMessages)
	}

	if err := tableAbsent(db, "qotd_reply_threads"); err != nil {
		t.Fatal(err)
	}
	if err := columnAbsent(db, "qotd_official_posts", "response_channel_id_snapshot"); err != nil {
		t.Fatal(err)
	}
	if err := columnAbsent(db, "qotd_official_posts", "is_pinned"); err != nil {
		t.Fatal(err)
	}
	if err := columnAbsent(db, "qotd_thread_archives", "reply_thread_id"); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresMigratorUpRepairsLegacyQOTDSurfaceChannelColumn(t *testing.T) {
	t.Parallel()

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve test database dsn: %v", err)
	}

	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	migrator := persistence.NewPostgresMigrator(db)
	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("apply baseline schema: %v", err)
	}

	if _, err := db.ExecContext(context.Background(), `
INSERT INTO qotd_forum_surfaces (
	guild_id,
	deck_id,
	channel_id,
	question_list_thread_id
) VALUES (
	'g1',
	'default',
	'channel-1',
	'thread-1'
)
`); err != nil {
		t.Fatalf("insert qotd surface: %v", err)
	}

	if _, err := db.ExecContext(context.Background(), `
ALTER TABLE qotd_forum_surfaces
RENAME COLUMN channel_id TO forum_channel_id
`); err != nil {
		t.Fatalf("drift qotd surfaces schema: %v", err)
	}

	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("repair drifted qotd surfaces schema: %v", err)
	}

	var repairedChannelID string
	if err := db.QueryRowContext(context.Background(), `
SELECT channel_id
FROM qotd_forum_surfaces
WHERE guild_id = 'g1' AND deck_id = 'default'
`).Scan(&repairedChannelID); err != nil {
		t.Fatalf("read repaired qotd surface: %v", err)
	}
	if repairedChannelID != "channel-1" {
		t.Fatalf("expected repaired qotd surface channel_id to be preserved, got %q", repairedChannelID)
	}

	if err := columnAbsent(db, "qotd_forum_surfaces", "forum_channel_id"); err != nil {
		t.Fatal(err)
	}
}

func tableAbsent(db *sql.DB, tableName string) error {
	var exists bool
	if err := db.QueryRowContext(context.Background(), `
SELECT EXISTS(
	SELECT 1
	FROM information_schema.tables
	WHERE table_schema = current_schema()
	  AND table_name = $1
)`, tableName).Scan(&exists); err != nil {
		return fmt.Errorf("query table %s existence: %w", tableName, err)
	}
	if exists {
		return fmt.Errorf("expected table %s to be absent", tableName)
	}
	return nil
}

func columnAbsent(db *sql.DB, tableName, columnName string) error {
	var exists bool
	if err := db.QueryRowContext(context.Background(), `
SELECT EXISTS(
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = current_schema()
	  AND table_name = $1
	  AND column_name = $2
)`, tableName, columnName).Scan(&exists); err != nil {
		return fmt.Errorf("query column %s.%s existence: %w", tableName, columnName, err)
	}
	if exists {
		return fmt.Errorf("expected column %s.%s to be absent", tableName, columnName)
	}
	return nil
}
