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
}
