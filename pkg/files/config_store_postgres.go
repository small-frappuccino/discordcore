package files

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// PostgresConfigStore persists BotConfig in PostgreSQL as one canonical JSONB document.
type PostgresConfigStore struct {
	db  *sql.DB
	key string
}

func NewPostgresConfigStore(db *sql.DB, key string) *PostgresConfigStore {
	key = strings.TrimSpace(key)
	if key == "" {
		key = DefaultPostgresConfigStoreKey
	}
	return &PostgresConfigStore{
		db:  db,
		key: key,
	}
}

func (s *PostgresConfigStore) Load() (*BotConfig, error) {
	cfg := &BotConfig{Guilds: []GuildConfig{}}
	if s == nil || s.db == nil {
		return cfg, fmt.Errorf("postgres config store database handle is nil")
	}

	var raw []byte
	err := s.db.QueryRow(
		`SELECT config_json FROM bot_config_state WHERE config_key = $1`,
		s.key,
	).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return cfg, nil
		}
		return nil, fmt.Errorf("load config row from postgres: %w", err)
	}
	if len(raw) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("decode config row from postgres: %w", err)
	}
	if cfg.Guilds == nil {
		cfg.Guilds = []GuildConfig{}
	}
	return cfg, nil
}

func (s *PostgresConfigStore) Save(cfg *BotConfig) error {
	if cfg == nil {
		return fmt.Errorf("cannot save nil config")
	}
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres config store database handle is nil")
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config for postgres: %w", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO bot_config_state (config_key, config_json)
		 VALUES ($1, $2::jsonb)
		 ON CONFLICT (config_key) DO UPDATE
		 SET config_json = EXCLUDED.config_json,
		     updated_at = NOW()`,
		s.key,
		string(raw),
	); err != nil {
		return fmt.Errorf("save config row to postgres: %w", err)
	}
	return nil
}

func (s *PostgresConfigStore) Exists() (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("postgres config store database handle is nil")
	}

	var exists bool
	if err := s.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM bot_config_state WHERE config_key = $1)`,
		s.key,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check config row in postgres: %w", err)
	}
	return exists, nil
}

func (s *PostgresConfigStore) Describe() string {
	key := DefaultPostgresConfigStoreKey
	if s != nil && strings.TrimSpace(s.key) != "" {
		key = s.key
	}
	return "postgres://bot_config_state/" + key
}
