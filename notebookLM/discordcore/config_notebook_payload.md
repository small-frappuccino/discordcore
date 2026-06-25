# Domain Architecture: config

## Layout Topology
```text
config/
├── config_store_memory.go
├── config_store_postgres.go
└── interfaces.go
```

## Source Stream Aggregation

// === FILE: pkg/config/config_store_memory.go ===
```go
package config

import (
	"fmt"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

const defaultMemoryConfigStoreDescription = "memory://bot_config_state"

// MemoryConfigStore persists files.BotConfig in memory.
// It is primarily intended for tests and lightweight local workflows that do
// not need cross-process persistence.
type MemoryConfigStore struct {
	mu          sync.Mutex
	config      *files.BotConfig
	exists      bool
	description string
}

// Load loads.
func (s *MemoryConfigStore) Load() (*files.BotConfig, error) {
	cfg := &files.BotConfig{Guilds: []files.GuildConfig{}}
	if s == nil {
		return cfg, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return cfg, nil
	}

	out := files.CloneBotConfigPtr(s.config)
	if out == nil {
		return cfg, nil
	}
	if out.Guilds == nil {
		out.Guilds = []files.GuildConfig{}
	}
	return out, nil
}

// Save saves.
func (s *MemoryConfigStore) Save(cfg *files.BotConfig) error {
	if cfg == nil {
		return fmt.Errorf("cannot save nil config")
	}
	if s == nil {
		return fmt.Errorf("memory config store is not configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = files.CloneBotConfigPtr(cfg)
	if s.config == nil {
		s.config = &files.BotConfig{Guilds: []files.GuildConfig{}}
	}
	if s.config.Guilds == nil {
		s.config.Guilds = []files.GuildConfig{}
	}
	s.exists = true
	return nil
}

// Exists exists.
func (s *MemoryConfigStore) Exists() (bool, error) {
	if s == nil {
		return false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exists, nil
}

// Describe describes.
func (s *MemoryConfigStore) Describe() string {
	if s == nil || s.description == "" {
		return defaultMemoryConfigStoreDescription
	}
	return s.description
}

```

// === FILE: pkg/config/config_store_postgres.go ===
```go
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresConfigStore persists files.BotConfig in PostgreSQL as one canonical JSONB document.
const DefaultPostgresConfigStoreKey = "primary"

type PostgresConfigStore struct {
	db     *pgxpool.Pool
	key    string
	logger *slog.Logger
}

// NewPostgresConfigStore news postgres config store.
func NewPostgresConfigStore(db *pgxpool.Pool, key string, logger *slog.Logger) *PostgresConfigStore {
	if logger == nil {
		logger = slog.Default()
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = DefaultPostgresConfigStoreKey
	}

	logger.Info("Architectural state transition: Coupling of isolated PostgreSQL storage adapter for configuration parameters")

	return &PostgresConfigStore{
		db:     db,
		key:    key,
		logger: logger,
	}
}

// Load loads.
func (s *PostgresConfigStore) Load() (*files.BotConfig, error) {
	cfg := &files.BotConfig{Guilds: []files.GuildConfig{}}
	if s == nil || s.db == nil {
		err := fmt.Errorf("postgres config store database handle is nil")
		files.EmitBlockingError(s.logger, "Blocking structural failure: Nil pointer blocked PostgreSQL driver initialization", err, files.GenerateRequestID())
		return cfg, err
	}

	s.logger.Info("Architectural state transition: Starting persistent loading of global tree",
		slog.String("store_key", s.key),
	)

	var globalRaw []byte
	queryGlobal := `SELECT config_json FROM bot_config_state WHERE config_key = $1`

	s.logger.Debug("Granular I/O inspection: Dump of dynamically generated SQL query (Load Global)",
		slog.String("query", queryGlobal),
		slog.String("param_1", s.key),
	)

	err := s.db.QueryRow(
		context.Background(),
		queryGlobal,
		s.key,
	).Scan(&globalRaw)

	if err != nil && err != pgx.ErrNoRows {
		errWrap := fmt.Errorf("load global config row from postgres: %w", err)
		files.EmitBlockingError(s.logger, "Blocking structural failure: SQL driver rejected global document read", errWrap, files.GenerateRequestID())
		return nil, errWrap
	}

	if err == pgx.ErrNoRows {
		s.logger.Warn("Mitigated degradation: Canonical document not found; matrix will adopt empty structure compensation routine",
			slog.String("missing_key", s.key),
		)
	} else if len(globalRaw) > 0 {
		s.logger.Debug("Granular transient state inspection: Raw deserialization enabled",
			slog.Int("payload_bytes", len(globalRaw)),
		)
		if err := json.Unmarshal(globalRaw, cfg); err != nil {
			errWrap := fmt.Errorf("decode global config row from postgres: %w", err)
			files.EmitBlockingError(s.logger, "Blocking structural failure: Corrupted JSON document parsing in global block", errWrap, files.GenerateRequestID())
			return nil, errWrap
		}
	}
	cfg.Guilds = []files.GuildConfig{}

	queryGuilds := `SELECT config_json FROM guild_configs`
	s.logger.Debug("Granular I/O inspection: Dump of dynamically generated SQL query (Load Guilds)",
		slog.String("query", queryGuilds),
	)

	rows, err := s.db.Query(
		context.Background(),
		queryGuilds,
	)
	if err != nil {
		errWrap := fmt.Errorf("query guild_configs: %w", err)
		files.EmitBlockingError(s.logger, "Blocking structural failure: Instance settings subgraph rejected by relational server", errWrap, files.GenerateRequestID())
		return nil, errWrap
	}
	defer rows.Close()

	for rows.Next() {
		var guildRaw []byte
		if err := rows.Scan(&guildRaw); err != nil {
			errWrap := fmt.Errorf("scan guild_configs row: %w", err)
			files.EmitBlockingError(s.logger, "Blocking structural failure: I/O cursor overflowed during bidirectional table tracking", errWrap, files.GenerateRequestID())
			return nil, errWrap
		}
		var guildCfg files.GuildConfig
		if err := json.Unmarshal(guildRaw, &guildCfg); err != nil {
			errWrap := fmt.Errorf("decode guild_configs json: %w", err)
			files.EmitBlockingError(s.logger, "Blocking structural failure: Corrupted JSON document parsing in guild sub-node", errWrap, files.GenerateRequestID())
			return nil, errWrap
		}
		cfg.Guilds = append(cfg.Guilds, guildCfg)
	}
	if err := rows.Err(); err != nil {
		errWrap := fmt.Errorf("iterate guild_configs rows: %w", err)
		files.EmitBlockingError(s.logger, "Blocking structural failure: SQL pagination pipe reported non-recoverable contention", errWrap, files.GenerateRequestID())
		return nil, errWrap
	}

	return cfg, nil
}

// Save saves.
func (s *PostgresConfigStore) Save(cfg *files.BotConfig) error {
	if cfg == nil {
		err := fmt.Errorf("cannot save nil config")
		files.EmitBlockingError(s.logger, "Blocking structural failure: Persistence attempt with nil global matrix", err, files.GenerateRequestID())
		return err
	}
	if s == nil || s.db == nil {
		err := fmt.Errorf("postgres config store database handle is nil")
		files.EmitBlockingError(s.logger, "Blocking structural failure: Synchronization blocked by nil relational driver", err, files.GenerateRequestID())
		return err
	}

	s.logger.Info("Architectural state transition: Initializing unified ACID transaction for I/O matrix write",
		slog.Int("guilds_payload", len(cfg.Guilds)),
	)

	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		errWrap := fmt.Errorf("begin config save tx: %w", err)
		files.EmitBlockingError(s.logger, "Blocking structural failure: Transaction negotiation aborted by DBMS", errWrap, files.GenerateRequestID())
		return errWrap
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
			s.logger.Warn("Mitigated degradation intercepted: Compensatory rollback of exposed transaction failed over TCP pipe",
				slog.String("error", rbErr.Error()),
			)
		}
	}()

	// Save global features/runtime to bot_config_state (without guilds)
	globalCopy := *cfg
	globalCopy.Guilds = nil
	globalRaw, err := json.Marshal(globalCopy)
	if err != nil {
		errWrap := fmt.Errorf("encode global config: %w", err)
		files.EmitBlockingError(s.logger, "Blocking structural failure: Marshal operation cleared primary write buffer", errWrap, files.GenerateRequestID())
		return errWrap
	}

	upsertGlobalQuery := `INSERT INTO bot_config_state (config_key, config_json)
		 VALUES ($1, $2::jsonb)
		 ON CONFLICT (config_key) DO UPDATE
		 SET config_json = EXCLUDED.config_json,
		     updated_at = NOW()`

	s.logger.Debug("Granular I/O inspection: Dump of conditional state SQL query (Upsert Global)",
		slog.String("query", upsertGlobalQuery),
		slog.String("param_1", s.key),
		slog.Int("payload_bytes", len(globalRaw)),
	)

	if _, err := tx.Exec(
		ctx,
		upsertGlobalQuery,
		s.key,
		string(globalRaw),
	); err != nil {
		errWrap := fmt.Errorf("save global config row: %w", err)
		files.EmitBlockingError(s.logger, "Blocking structural failure: Base topology upsert executable command rejected", errWrap, files.GenerateRequestID())
		return errWrap
	}

	upsertGuildQuery := `INSERT INTO guild_configs (guild_id, config_version, config_json)
			 VALUES ($1, $2, $3::jsonb)
			 ON CONFLICT (guild_id) DO UPDATE
			 SET config_version = EXCLUDED.config_version,
			     config_json = EXCLUDED.config_json,
			     updated_at = NOW()`

	// Upsert all guilds into guild_configs table
	for _, guild := range cfg.Guilds {
		guildRaw, err := json.Marshal(guild)
		if err != nil {
			errWrap := fmt.Errorf("encode guild config for %s: %w", guild.GuildID, err)
			files.EmitBlockingError(s.logger, "Blocking structural failure: Marshal operation failed on isolated hierarchical scope", errWrap, files.GenerateRequestID())
			return errWrap
		}

		s.logger.Debug("Granular transient state inspection: Injecting atomic relational branch for guild node",
			slog.String("guild_id", guild.GuildID),
			slog.Int64("config_version", guild.ConfigVersion),
			slog.Int("payload_bytes", len(guildRaw)),
		)

		if _, err := tx.Exec(
			ctx,
			upsertGuildQuery,
			guild.GuildID,
			guild.ConfigVersion,
			string(guildRaw),
		); err != nil {
			errWrap := fmt.Errorf("save guild_configs row %s: %w", guild.GuildID, err)
			files.EmitBlockingError(s.logger, "Blocking structural failure: Collision or transactional obstruction bound to sub-level", errWrap, files.GenerateRequestID())
			return errWrap
		}
	}

	if err := tx.Commit(ctx); err != nil {
		errWrap := fmt.Errorf("commit config save tx: %w", err)
		files.EmitBlockingError(s.logger, "Blocking structural failure: Consolidative 2PC protocol rejected; commit failed and locked state at source", errWrap, files.GenerateRequestID())
		return errWrap
	}

	s.logger.Info("Architectural state transition: SQL ACID transaction completed, I/O pipeline drained")
	return nil
}

// Exists exists.
func (s *PostgresConfigStore) Exists() (bool, error) {
	if s == nil || s.db == nil {
		err := fmt.Errorf("postgres config store database handle is nil")
		files.EmitBlockingError(s.logger, "Blocking structural failure: Static probe failed on node referential integrity", err, files.GenerateRequestID())
		return false, err
	}

	var exists bool
	queryExists := `SELECT EXISTS(SELECT 1 FROM bot_config_state WHERE config_key = $1)`

	s.logger.Debug("Granular I/O inspection: Conditional tracking of relational SQL verification",
		slog.String("query", queryExists),
		slog.String("param_1", s.key),
	)

	if err := s.db.QueryRow(
		context.Background(),
		queryExists,
		s.key,
	).Scan(&exists); err != nil {
		errWrap := fmt.Errorf("check config row in postgres: %w", err)
		files.EmitBlockingError(s.logger, "Blocking structural failure: Scalar boolean query collapsed during scan", errWrap, files.GenerateRequestID())
		return false, errWrap
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

```

// === FILE: pkg/config/interfaces.go ===
```go
package config

import (
	"context"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// Provider defines the interface for reading configuration states.
type Provider interface {
	Config() *files.BotConfig
	GuildConfig(guildID string) *files.GuildConfig
	UpdateConfig(ctx context.Context, fn func(*files.BotConfig) error) (files.BotConfig, error)
	LoadConfig() error
	UpdateRuntimeConfig(fn func(*files.RuntimeConfig) error) (files.RuntimeConfig, error)
	UpdateGuildConfig(guildID string, fn func(*files.GuildConfig) error) error
	RolePanels(guildID string) ([]files.RolePanelConfig, error)
	RolePanel(guildID, key string) (files.RolePanelConfig, error)
	SetRolePanelEmbed(guildID, key string, embed files.RolePanelConfig) error
	AddRolePanelField(guildID, key string, field files.RolePanelEmbedFieldConfig) error
	RemoveRolePanelField(guildID, key string, fieldIndex int) error
	UpsertRolePanelButton(guildID, key string, button files.RolePanelButtonConfig) error
	DeleteRolePanelButton(guildID, key, roleID string) error
	DeleteRolePanel(guildID, key string) error
	ListRolePanelPostings(guildID, key string) ([]files.RolePanelPostingConfig, error)
	AddRolePanelPosting(guildID, key string, posting files.RolePanelPostingConfig) error
	RemoveRolePanelPosting(guildID, key, messageID string) error
	RemoveRolePanelPostings(guildID, key string, messageIDs []string) error
	ClearRolePanelPostings(guildID, key string) error
	FindRolePanelPosting(guildID, messageID string) (string, files.RolePanelPostingConfig, error)
	RolePanelButtonByRoleID(guildID, roleID string) (files.RolePanelConfig, files.RolePanelButtonConfig, error)
}

// Loader defines the read paths for the bot configuration.
type Loader interface {
	Load() (*files.BotConfig, error)
	Exists() (bool, error)
}

// Saver defines the write path for the bot configuration.
type Saver interface {
	Save(*files.BotConfig) error
}

// Store persists the canonical BotConfig by combining read, write capabilities.
type Store interface {
	Loader
	Saver
}

```

