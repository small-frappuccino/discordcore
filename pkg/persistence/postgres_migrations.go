package persistence

var postgresMigrations = []migration{
	{
		Version: 1,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS messages (
				guild_id        TEXT NOT NULL,
				message_id      TEXT NOT NULL,
				channel_id      TEXT NOT NULL,
				author_id       TEXT NOT NULL,
				author_username TEXT,
				author_avatar   TEXT,
				content         TEXT,
				cached_at       TIMESTAMPTZ NOT NULL,
				expires_at      TIMESTAMPTZ,
				PRIMARY KEY (guild_id, message_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_messages_expires ON messages(expires_at)`,
			`CREATE TABLE IF NOT EXISTS member_joins (
				guild_id   TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				joined_at  TIMESTAMPTZ NOT NULL,
				PRIMARY KEY (guild_id, user_id)
			)`,
			`CREATE TABLE IF NOT EXISTS avatars_current (
				guild_id    TEXT NOT NULL,
				user_id     TEXT NOT NULL,
				avatar_hash TEXT NOT NULL,
				updated_at  TIMESTAMPTZ NOT NULL,
				PRIMARY KEY (guild_id, user_id)
			)`,
			`CREATE TABLE IF NOT EXISTS avatars_history (
				id         BIGSERIAL PRIMARY KEY,
				guild_id   TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				old_hash   TEXT,
				new_hash   TEXT,
				changed_at TIMESTAMPTZ NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_avatars_hist_gid_uid ON avatars_history(guild_id, user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_avatars_hist_changed ON avatars_history(changed_at)`,
			`CREATE TABLE IF NOT EXISTS messages_history (
				id            BIGSERIAL PRIMARY KEY,
				guild_id      TEXT NOT NULL,
				message_id    TEXT NOT NULL,
				channel_id    TEXT NOT NULL,
				author_id     TEXT NOT NULL,
				version       INTEGER NOT NULL,
				event_type    TEXT NOT NULL,
				content       TEXT,
				attachments   INTEGER NOT NULL DEFAULT 0,
				embeds_count  INTEGER NOT NULL DEFAULT 0,
				stickers      INTEGER NOT NULL DEFAULT 0,
				created_at    TIMESTAMPTZ NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_msg_hist_gid_mid ON messages_history(guild_id, message_id)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_hist_gid_mid_ver ON messages_history(guild_id, message_id, version)`,
			`CREATE TABLE IF NOT EXISTS guild_meta (
				guild_id  TEXT PRIMARY KEY,
				bot_since TIMESTAMPTZ,
				owner_id  TEXT
			)`,
			`CREATE TABLE IF NOT EXISTS runtime_meta (
				key TEXT PRIMARY KEY,
				ts  TIMESTAMPTZ NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS moderation_cases (
				guild_id         TEXT PRIMARY KEY,
				last_case_number BIGINT NOT NULL DEFAULT 0
			)`,
			`CREATE TABLE IF NOT EXISTS roles_current (
				guild_id   TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				role_id    TEXT NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL,
				PRIMARY KEY (guild_id, user_id, role_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_roles_current_member ON roles_current(guild_id, user_id)`,
			`CREATE TABLE IF NOT EXISTS persistent_cache (
				cache_key  TEXT PRIMARY KEY,
				cache_type TEXT NOT NULL,
				data       TEXT NOT NULL,
				expires_at TIMESTAMPTZ NOT NULL,
				cached_at  TIMESTAMPTZ NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_persistent_cache_type ON persistent_cache(cache_type)`,
			`CREATE INDEX IF NOT EXISTS idx_persistent_cache_expires ON persistent_cache(expires_at)`,
			`CREATE TABLE IF NOT EXISTS daily_message_metrics (
				guild_id   TEXT NOT NULL,
				channel_id TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				day        DATE NOT NULL,
				count      BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, channel_id, user_id, day)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_msg_by_guild_day ON daily_message_metrics(guild_id, day)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_msg_by_channel_day ON daily_message_metrics(channel_id, day)`,
			`CREATE TABLE IF NOT EXISTS daily_reaction_metrics (
				guild_id   TEXT NOT NULL,
				channel_id TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				day        DATE NOT NULL,
				count      BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, channel_id, user_id, day)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_react_by_guild_day ON daily_reaction_metrics(guild_id, day)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_react_by_channel_day ON daily_reaction_metrics(channel_id, day)`,
			`CREATE TABLE IF NOT EXISTS daily_member_joins (
				guild_id TEXT NOT NULL,
				user_id  TEXT NOT NULL,
				day      DATE NOT NULL,
				count    BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, user_id, day)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_joins_by_guild_day ON daily_member_joins(guild_id, day)`,
			`CREATE TABLE IF NOT EXISTS daily_member_leaves (
				guild_id TEXT NOT NULL,
				user_id  TEXT NOT NULL,
				day      DATE NOT NULL,
				count    BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, user_id, day)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_leaves_by_guild_day ON daily_member_leaves(guild_id, day)`,
		},
		DownSQL: []string{
			`DROP TABLE IF EXISTS daily_member_leaves`,
			`DROP TABLE IF EXISTS daily_member_joins`,
			`DROP TABLE IF EXISTS daily_reaction_metrics`,
			`DROP TABLE IF EXISTS daily_message_metrics`,
			`DROP TABLE IF EXISTS persistent_cache`,
			`DROP TABLE IF EXISTS roles_current`,
			`DROP TABLE IF EXISTS moderation_cases`,
			`DROP TABLE IF EXISTS runtime_meta`,
			`DROP TABLE IF EXISTS guild_meta`,
			`DROP TABLE IF EXISTS messages_history`,
			`DROP TABLE IF EXISTS avatars_history`,
			`DROP TABLE IF EXISTS avatars_current`,
			`DROP TABLE IF EXISTS member_joins`,
			`DROP TABLE IF EXISTS messages`,
		},
	},
	{
		Version: 2,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS bot_config_state (
				config_key TEXT PRIMARY KEY,
				config_json JSONB NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		},
		DownSQL: []string{
			`DROP TABLE IF EXISTS bot_config_state`,
		},
	},
	{
		Version: 3,
		UpSQL: []string{
			`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`,
			`UPDATE member_joins
			 SET last_seen_at = COALESCE(last_seen_at, joined_at)
			 WHERE last_seen_at IS NULL`,
		},
		DownSQL: []string{
			`ALTER TABLE member_joins DROP COLUMN IF EXISTS last_seen_at`,
		},
	},
	{
		Version: 4,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS message_version_counters (
				guild_id     TEXT NOT NULL,
				message_id   TEXT NOT NULL,
				last_version BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, message_id)
			)`,
			`INSERT INTO message_version_counters (guild_id, message_id, last_version)
			 SELECT guild_id, message_id, MAX(version)::BIGINT
			 FROM messages_history
			 GROUP BY guild_id, message_id
			 ON CONFLICT (guild_id, message_id) DO UPDATE
			 SET last_version = GREATEST(message_version_counters.last_version, EXCLUDED.last_version)`,
		},
		DownSQL: []string{
			`DROP TABLE IF EXISTS message_version_counters`,
		},
	},
	{
		Version: 5,
		UpSQL: []string{
			`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS is_bot BOOLEAN`,
			`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ`,
			`CREATE INDEX IF NOT EXISTS idx_member_joins_active ON member_joins(guild_id, left_at)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_member_joins_active`,
			`ALTER TABLE member_joins DROP COLUMN IF EXISTS left_at`,
			`ALTER TABLE member_joins DROP COLUMN IF EXISTS is_bot`,
		},
	},
	{
		Version: 6,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS moderation_warnings (
				id           BIGSERIAL PRIMARY KEY,
				guild_id     TEXT NOT NULL,
				user_id      TEXT NOT NULL,
				case_number  BIGINT NOT NULL,
				moderator_id TEXT NOT NULL,
				reason       TEXT NOT NULL,
				created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_moderation_warnings_case ON moderation_warnings(guild_id, case_number)`,
			`CREATE INDEX IF NOT EXISTS idx_moderation_warnings_user ON moderation_warnings(guild_id, user_id, created_at DESC)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_moderation_warnings_user`,
			`DROP INDEX IF EXISTS idx_moderation_warnings_case`,
			`DROP TABLE IF EXISTS moderation_warnings`,
		},
	},
	{
		Version: 7,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS qotd_questions (
				id                     BIGSERIAL PRIMARY KEY,
				guild_id               TEXT NOT NULL,
				body                   TEXT NOT NULL,
				status                 TEXT NOT NULL,
				queue_position         BIGINT NOT NULL,
				created_by             TEXT,
				scheduled_for_date_utc DATE,
				used_at                TIMESTAMPTZ,
				created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_queue ON qotd_questions(guild_id, queue_position)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_schedule ON qotd_questions(guild_id, scheduled_for_date_utc) WHERE scheduled_for_date_utc IS NOT NULL`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_questions_status ON qotd_questions(guild_id, status, queue_position)`,
			`CREATE TABLE IF NOT EXISTS qotd_official_posts (
				id                         BIGSERIAL PRIMARY KEY,
				guild_id                   TEXT NOT NULL,
				question_id                BIGINT NOT NULL REFERENCES qotd_questions(id) ON DELETE RESTRICT,
				publish_date_utc           DATE NOT NULL,
				state                      TEXT NOT NULL,
				channel_id                 TEXT NOT NULL,
				discord_thread_id          TEXT,
				discord_starter_message_id TEXT,
				question_text_snapshot     TEXT NOT NULL,
				published_at               TIMESTAMPTZ,
				grace_until                TIMESTAMPTZ NOT NULL,
				archive_at                 TIMESTAMPTZ NOT NULL,
				closed_at                  TIMESTAMPTZ,
				archived_at                TIMESTAMPTZ,
				last_reconciled_at         TIMESTAMPTZ,
				created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_publish_date ON qotd_official_posts(guild_id, publish_date_utc)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_thread ON qotd_official_posts(discord_thread_id) WHERE discord_thread_id IS NOT NULL`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_official_posts_archive ON qotd_official_posts(archived_at, archive_at)`,
			`CREATE TABLE IF NOT EXISTS qotd_thread_archives (
				id                BIGSERIAL PRIMARY KEY,
				guild_id          TEXT NOT NULL,
				official_post_id  BIGINT NOT NULL REFERENCES qotd_official_posts(id) ON DELETE CASCADE,
				source_kind       TEXT NOT NULL,
				discord_thread_id TEXT NOT NULL,
				archived_at       TIMESTAMPTZ NOT NULL,
				created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_thread_archives_thread ON qotd_thread_archives(discord_thread_id)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_thread_archives_post ON qotd_thread_archives(official_post_id, source_kind, archived_at DESC)`,
			`CREATE TABLE IF NOT EXISTS qotd_message_archives (
				id                   BIGSERIAL PRIMARY KEY,
				thread_archive_id    BIGINT NOT NULL REFERENCES qotd_thread_archives(id) ON DELETE CASCADE,
				discord_message_id   TEXT NOT NULL,
				author_id            TEXT,
				author_name_snapshot TEXT,
				author_is_bot        BOOLEAN NOT NULL DEFAULT FALSE,
				content              TEXT,
				embeds_json          JSONB,
				attachments_json     JSONB,
				created_at           TIMESTAMPTZ NOT NULL
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_message_archives_unique_message ON qotd_message_archives(thread_archive_id, discord_message_id)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_message_archives_created ON qotd_message_archives(thread_archive_id, created_at ASC)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_message_archives_created`,
			`DROP INDEX IF EXISTS idx_qotd_message_archives_unique_message`,
			`DROP TABLE IF EXISTS qotd_message_archives`,
			`DROP INDEX IF EXISTS idx_qotd_thread_archives_post`,
			`DROP INDEX IF EXISTS idx_qotd_thread_archives_thread`,
			`DROP TABLE IF EXISTS qotd_thread_archives`,
			`DROP INDEX IF EXISTS idx_qotd_official_posts_archive`,
			`DROP INDEX IF EXISTS idx_qotd_official_posts_thread`,
			`DROP INDEX IF EXISTS idx_qotd_official_posts_publish_date`,
			`DROP TABLE IF EXISTS qotd_official_posts`,
			`DROP INDEX IF EXISTS idx_qotd_questions_status`,
			`DROP INDEX IF EXISTS idx_qotd_questions_schedule`,
			`DROP INDEX IF EXISTS idx_qotd_questions_queue`,
			`DROP TABLE IF EXISTS qotd_questions`,
		},
	},
	{
		Version: 8,
		UpSQL: []string{
			`ALTER TABLE qotd_official_posts ADD COLUMN IF NOT EXISTS publish_mode TEXT`,
			`UPDATE qotd_official_posts
			 SET publish_mode = 'scheduled'
			 WHERE publish_mode IS NULL OR publish_mode = ''`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN publish_mode SET DEFAULT 'scheduled'`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN publish_mode SET NOT NULL`,
			`DROP INDEX IF EXISTS idx_qotd_official_posts_publish_date`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_scheduled_publish_date
			 ON qotd_official_posts(guild_id, publish_date_utc)
			 WHERE publish_mode = 'scheduled'`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_official_posts_scheduled_publish_date`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_publish_date ON qotd_official_posts(guild_id, publish_date_utc)`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS publish_mode`,
		},
	},
	{
		Version: 9,
		UpSQL:   []string{},
		DownSQL: []string{},
	},
	{
		Version: 10,
		UpSQL: []string{
			`ALTER TABLE qotd_questions ADD COLUMN IF NOT EXISTS deck_id TEXT`,
			`UPDATE qotd_questions
			 SET deck_id = 'default'
			 WHERE deck_id IS NULL OR deck_id = ''`,
			`ALTER TABLE qotd_questions ALTER COLUMN deck_id SET DEFAULT 'default'`,
			`ALTER TABLE qotd_questions ALTER COLUMN deck_id SET NOT NULL`,
			`DROP INDEX IF EXISTS idx_qotd_questions_queue`,
			`DROP INDEX IF EXISTS idx_qotd_questions_status`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_queue
			 ON qotd_questions(guild_id, deck_id, queue_position)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_questions_status
			 ON qotd_questions(guild_id, deck_id, status, queue_position)`,
			`ALTER TABLE qotd_official_posts ADD COLUMN IF NOT EXISTS deck_id TEXT`,
			`UPDATE qotd_official_posts
			 SET deck_id = 'default'
			 WHERE deck_id IS NULL OR deck_id = ''`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN deck_id SET DEFAULT 'default'`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN deck_id SET NOT NULL`,
			`ALTER TABLE qotd_official_posts ADD COLUMN IF NOT EXISTS deck_name_snapshot TEXT`,
			`UPDATE qotd_official_posts
			 SET deck_name_snapshot = 'Default'
			 WHERE deck_name_snapshot IS NULL OR deck_name_snapshot = ''`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN deck_name_snapshot SET DEFAULT 'Default'`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN deck_name_snapshot SET NOT NULL`,
		},
		DownSQL: []string{
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS deck_name_snapshot`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS deck_id`,
			`DROP INDEX IF EXISTS idx_qotd_questions_status`,
			`DROP INDEX IF EXISTS idx_qotd_questions_queue`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_queue ON qotd_questions(guild_id, queue_position)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_questions_status ON qotd_questions(guild_id, status, queue_position)`,
			`ALTER TABLE qotd_questions DROP COLUMN IF EXISTS deck_id`,
		},
	},
	{
		Version: 11,
		UpSQL: []string{
			`ALTER TABLE qotd_official_posts
			 DROP CONSTRAINT IF EXISTS qotd_official_posts_question_id_fkey`,
		},
		DownSQL: []string{
			`ALTER TABLE qotd_official_posts
			 ADD CONSTRAINT qotd_official_posts_question_id_fkey
			 FOREIGN KEY (question_id) REFERENCES qotd_questions(id) ON DELETE RESTRICT`,
		},
	},
	{
		Version: 12,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS qotd_collected_questions (
				id                         BIGSERIAL PRIMARY KEY,
				guild_id                   TEXT NOT NULL,
				source_channel_id          TEXT NOT NULL,
				source_message_id          TEXT NOT NULL,
				source_author_id           TEXT,
				source_author_name_snapshot TEXT,
				source_created_at          TIMESTAMPTZ NOT NULL,
				embed_title                TEXT NOT NULL DEFAULT '',
				question_text              TEXT NOT NULL,
				created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_collected_questions_message
			 ON qotd_collected_questions(guild_id, source_message_id)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_collected_questions_recent
			 ON qotd_collected_questions(guild_id, source_created_at DESC, id DESC)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_collected_questions_recent`,
			`DROP INDEX IF EXISTS idx_qotd_collected_questions_message`,
			`DROP TABLE IF EXISTS qotd_collected_questions`,
		},
	},
	{
		Version: 13,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS qotd_forum_surfaces (
				id                     BIGSERIAL PRIMARY KEY,
				guild_id               TEXT NOT NULL,
				deck_id                TEXT NOT NULL,
				channel_id             TEXT NOT NULL,
				question_list_thread_id TEXT,
				created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_forum_surfaces_deck
			 ON qotd_forum_surfaces(guild_id, deck_id)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_forum_surfaces_thread
			 ON qotd_forum_surfaces(question_list_thread_id)
			 WHERE question_list_thread_id IS NOT NULL AND question_list_thread_id <> ''`,
			`ALTER TABLE qotd_official_posts
			 ADD COLUMN IF NOT EXISTS question_list_thread_id TEXT`,
			`ALTER TABLE qotd_official_posts
			 ADD COLUMN IF NOT EXISTS question_list_entry_message_id TEXT`,
			`ALTER TABLE qotd_official_posts
			 ADD COLUMN IF NOT EXISTS answer_channel_id TEXT`,
			`UPDATE qotd_official_posts
			 SET answer_channel_id = COALESCE(NULLIF(discord_thread_id, ''), channel_id)
			 WHERE answer_channel_id IS NULL OR answer_channel_id = ''`,
			`UPDATE qotd_official_posts
			 SET answer_channel_id = ''
			 WHERE answer_channel_id IS NULL`,
			`ALTER TABLE qotd_official_posts
			 ALTER COLUMN answer_channel_id SET DEFAULT ''`,
			`ALTER TABLE qotd_official_posts
			 ALTER COLUMN answer_channel_id SET NOT NULL`,
			`CREATE TABLE IF NOT EXISTS qotd_answer_messages (
				id                         BIGSERIAL PRIMARY KEY,
				guild_id                   TEXT NOT NULL,
				official_post_id           BIGINT NOT NULL REFERENCES qotd_official_posts(id) ON DELETE CASCADE,
				user_id                    TEXT NOT NULL,
				state                      TEXT NOT NULL,
				answer_channel_id          TEXT NOT NULL,
				discord_message_id         TEXT,
				created_via_interaction_id TEXT,
				created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				closed_at                  TIMESTAMPTZ,
				archived_at                TIMESTAMPTZ
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_answer_messages_unique_user
			 ON qotd_answer_messages(official_post_id, user_id)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_answer_messages_message
			 ON qotd_answer_messages(discord_message_id)
			 WHERE discord_message_id IS NOT NULL AND discord_message_id <> ''`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_answer_messages_state
			 ON qotd_answer_messages(official_post_id, state, created_at)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_answer_messages_state`,
			`DROP INDEX IF EXISTS idx_qotd_answer_messages_message`,
			`DROP INDEX IF EXISTS idx_qotd_answer_messages_unique_user`,
			`DROP TABLE IF EXISTS qotd_answer_messages`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS answer_channel_id`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS question_list_entry_message_id`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS question_list_thread_id`,
			`DROP INDEX IF EXISTS idx_qotd_forum_surfaces_thread`,
			`DROP INDEX IF EXISTS idx_qotd_forum_surfaces_deck`,
			`DROP TABLE IF EXISTS qotd_forum_surfaces`,
		},
	},
	{
		Version: 14,
		UpSQL:   []string{},
		DownSQL: []string{},
	},
	{
		Version: 15,
		UpSQL: []string{
			`ALTER TABLE qotd_questions
			 ADD COLUMN IF NOT EXISTS display_id BIGINT`,
			`WITH ordered AS (
				SELECT
					id,
					ROW_NUMBER() OVER (
						PARTITION BY guild_id, deck_id
						ORDER BY queue_position ASC, id ASC
					)::BIGINT AS next_display_id
				FROM qotd_questions
			)
			UPDATE qotd_questions AS questions
			SET display_id = ordered.next_display_id
			FROM ordered
			WHERE questions.id = ordered.id`,
			`UPDATE qotd_questions
			 SET display_id = 1
			 WHERE display_id IS NULL`,
			`ALTER TABLE qotd_questions
			 ALTER COLUMN display_id SET NOT NULL`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_display_id
			 ON qotd_questions(guild_id, deck_id, display_id)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_questions_display_id`,
			`ALTER TABLE qotd_questions DROP COLUMN IF EXISTS display_id`,
		},
	},
	{
		Version: 16,
		UpSQL: []string{
			`ALTER TABLE qotd_questions
			 ADD COLUMN IF NOT EXISTS published_once_at TIMESTAMPTZ`,
			`UPDATE qotd_questions
			 SET published_once_at = used_at
			 WHERE published_once_at IS NULL
			   AND used_at IS NOT NULL`,
			`UPDATE qotd_questions AS questions
			 SET published_once_at = published.published_at
			 FROM (
				SELECT question_id, MAX(published_at) AS published_at
				FROM qotd_official_posts
				WHERE published_at IS NOT NULL
				GROUP BY question_id
			 ) AS published
			 WHERE questions.id = published.question_id
			   AND questions.published_once_at IS NULL`,
		},
		DownSQL: []string{
			`ALTER TABLE qotd_questions DROP COLUMN IF EXISTS published_once_at`,
		},
	},
	{
		Version: 17,
		UpSQL: []string{
			`ALTER TABLE qotd_official_posts
			 ADD COLUMN IF NOT EXISTS consume_automatic_slot BOOLEAN`,
			`UPDATE qotd_official_posts
			 SET consume_automatic_slot = TRUE
			 WHERE consume_automatic_slot IS NULL`,
			`ALTER TABLE qotd_official_posts
			 ALTER COLUMN consume_automatic_slot SET DEFAULT TRUE`,
			`ALTER TABLE qotd_official_posts
			 ALTER COLUMN consume_automatic_slot SET NOT NULL`,
		},
		DownSQL: []string{
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS consume_automatic_slot`,
		},
	},
}
