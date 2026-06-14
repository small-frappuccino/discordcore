package files

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresConfigStore persists BotConfig in PostgreSQL as one canonical JSONB document.
type PostgresConfigStore struct {
	db  *pgxpool.Pool
	key string
}

// NewPostgresConfigStore news postgres config store.
func NewPostgresConfigStore(db *pgxpool.Pool, key string) *PostgresConfigStore {
	key = strings.TrimSpace(key)
	if key == "" {
		key = DefaultPostgresConfigStoreKey
	}
	return &PostgresConfigStore{
		db:  db,
		key: key,
	}
}

// Load loads.
func (s *PostgresConfigStore) Load() (*BotConfig, error) {
	cfg := &BotConfig{Guilds: []GuildConfig{}}
	if s == nil || s.db == nil {
		return cfg, fmt.Errorf("postgres config store database handle is nil")
	}

	var globalRaw []byte
	err := s.db.QueryRow(
		context.Background(),
		`SELECT config_json FROM bot_config_state WHERE config_key = $1`,
		s.key,
	).Scan(&globalRaw)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("load global config row from postgres: %w", err)
	}
	if len(globalRaw) > 0 {
		if err := json.Unmarshal(globalRaw, cfg); err != nil {
			return nil, fmt.Errorf("decode global config row from postgres: %w", err)
		}
	}
	cfg.Guilds = []GuildConfig{}

	rows, err := s.db.Query(
		context.Background(),
		`SELECT config_json FROM guild_configs`,
	)
	if err != nil {
		return nil, fmt.Errorf("query guild_configs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var guildRaw []byte
		if err := rows.Scan(&guildRaw); err != nil {
			return nil, fmt.Errorf("scan guild_configs row: %w", err)
		}
		var guildCfg GuildConfig
		if err := json.Unmarshal(guildRaw, &guildCfg); err != nil {
			return nil, fmt.Errorf("decode guild_configs json: %w", err)
		}
		cfg.Guilds = append(cfg.Guilds, guildCfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate guild_configs rows: %w", err)
	}

	return cfg, nil
}

// Save saves.
func (s *PostgresConfigStore) Save(cfg *BotConfig) error {
	if cfg == nil {
		return fmt.Errorf("cannot save nil config")
	}
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres config store database handle is nil")
	}

	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin config save tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Save global features/runtime to bot_config_state (without guilds)
	globalCopy := *cfg
	globalCopy.Guilds = nil
	globalRaw, err := json.Marshal(globalCopy)
	if err != nil {
		return fmt.Errorf("encode global config: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO bot_config_state (config_key, config_json)
		 VALUES ($1, $2::jsonb)
		 ON CONFLICT (config_key) DO UPDATE
		 SET config_json = EXCLUDED.config_json,
		     updated_at = NOW()`,
		s.key,
		string(globalRaw),
	); err != nil {
		return fmt.Errorf("save global config row: %w", err)
	}

	// Upsert all guilds into guild_configs table
	for _, guild := range cfg.Guilds {
		guildRaw, err := json.Marshal(guild)
		if err != nil {
			return fmt.Errorf("encode guild config for %s: %w", guild.GuildID, err)
		}
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO guild_configs (guild_id, config_version, config_json)
			 VALUES ($1, $2, $3::jsonb)
			 ON CONFLICT (guild_id) DO UPDATE
			 SET config_version = EXCLUDED.config_version,
			     config_json = EXCLUDED.config_json,
			     updated_at = NOW()`,
			guild.GuildID,
			guild.ConfigVersion,
			string(guildRaw),
		); err != nil {
			return fmt.Errorf("save guild_configs row %s: %w", guild.GuildID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit config save tx: %w", err)
	}
	return nil
}

// Exists exists.
func (s *PostgresConfigStore) Exists() (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("postgres config store database handle is nil")
	}

	var exists bool
	if err := s.db.QueryRow(
		context.Background(),
		`SELECT EXISTS(SELECT 1 FROM bot_config_state WHERE config_key = $1)`,
		s.key,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check config row in postgres: %w", err)
	}
	return exists, nil
}

// Describe describes.
func (s *PostgresConfigStore) Describe() string {
	key := DefaultPostgresConfigStoreKey
	if s != nil && strings.TrimSpace(s.key) != "" {
		key = s.key
	}
	return "postgres://bot_config_state/" + key
}
