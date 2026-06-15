package files

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/small-frappuccino/discordcore/pkg/log"
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

	slog.Info("Architectural state transition: Coupling of isolated PostgreSQL storage adapter for configuration parameters")

	return &PostgresConfigStore{
		db:  db,
		key: key,
	}
}

// Load loads.
func (s *PostgresConfigStore) Load() (*BotConfig, error) {
	cfg := &BotConfig{Guilds: []GuildConfig{}}
	if s == nil || s.db == nil {
		err := fmt.Errorf("postgres config store database handle is nil")
		log.EmitBlockingError("Blocking structural failure: Nil pointer blocked PostgreSQL driver initialization", err, log.GenerateRequestID())
		return cfg, err
	}

	slog.Info("Architectural state transition: Starting persistent loading of global tree",
		slog.String("store_key", s.key),
	)

	var globalRaw []byte
	queryGlobal := `SELECT config_json FROM bot_config_state WHERE config_key = $1`

	slog.Debug("Granular I/O inspection: Dump of dynamically generated SQL query (Load Global)",
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
		log.EmitBlockingError("Blocking structural failure: SQL driver rejected global document read", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	if err == pgx.ErrNoRows {
		slog.Warn("Mitigated degradation: Canonical document not found; matrix will adopt empty structure compensation routine",
			slog.String("missing_key", s.key),
		)
	} else if len(globalRaw) > 0 {
		slog.Debug("Granular transient state inspection: Raw deserialization enabled",
			slog.Int("payload_bytes", len(globalRaw)),
		)
		if err := json.Unmarshal(globalRaw, cfg); err != nil {
			errWrap := fmt.Errorf("decode global config row from postgres: %w", err)
			log.EmitBlockingError("Blocking structural failure: Corrupted JSON document parsing in global block", errWrap, log.GenerateRequestID())
			return nil, errWrap
		}
	}
	cfg.Guilds = []GuildConfig{}

	queryGuilds := `SELECT config_json FROM guild_configs`
	slog.Debug("Granular I/O inspection: Dump of dynamically generated SQL query (Load Guilds)",
		slog.String("query", queryGuilds),
	)

	rows, err := s.db.Query(
		context.Background(),
		queryGuilds,
	)
	if err != nil {
		errWrap := fmt.Errorf("query guild_configs: %w", err)
		log.EmitBlockingError("Blocking structural failure: Instance settings subgraph rejected by relational server", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}
	defer rows.Close()

	for rows.Next() {
		var guildRaw []byte
		if err := rows.Scan(&guildRaw); err != nil {
			errWrap := fmt.Errorf("scan guild_configs row: %w", err)
			log.EmitBlockingError("Blocking structural failure: I/O cursor overflowed during bidirectional table tracking", errWrap, log.GenerateRequestID())
			return nil, errWrap
		}
		var guildCfg GuildConfig
		if err := json.Unmarshal(guildRaw, &guildCfg); err != nil {
			errWrap := fmt.Errorf("decode guild_configs json: %w", err)
			log.EmitBlockingError("Blocking structural failure: Corrupted JSON document parsing in guild sub-node", errWrap, log.GenerateRequestID())
			return nil, errWrap
		}
		cfg.Guilds = append(cfg.Guilds, guildCfg)
	}
	if err := rows.Err(); err != nil {
		errWrap := fmt.Errorf("iterate guild_configs rows: %w", err)
		log.EmitBlockingError("Blocking structural failure: SQL pagination pipe reported non-recoverable contention", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	return cfg, nil
}

// Save saves.
func (s *PostgresConfigStore) Save(cfg *BotConfig) error {
	if cfg == nil {
		err := fmt.Errorf("cannot save nil config")
		log.EmitBlockingError("Blocking structural failure: Persistence attempt with nil global matrix", err, log.GenerateRequestID())
		return err
	}
	if s == nil || s.db == nil {
		err := fmt.Errorf("postgres config store database handle is nil")
		log.EmitBlockingError("Blocking structural failure: Synchronization blocked by nil relational driver", err, log.GenerateRequestID())
		return err
	}

	slog.Info("Architectural state transition: Initializing unified ACID transaction for I/O matrix write",
		slog.Int("guilds_payload", len(cfg.Guilds)),
	)

	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		errWrap := fmt.Errorf("begin config save tx: %w", err)
		log.EmitBlockingError("Blocking structural failure: Transaction negotiation aborted by DBMS", errWrap, log.GenerateRequestID())
		return errWrap
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
			slog.Warn("Mitigated degradation intercepted: Compensatory rollback of exposed transaction failed over TCP pipe",
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
		log.EmitBlockingError("Blocking structural failure: Marshal operation cleared primary write buffer", errWrap, log.GenerateRequestID())
		return errWrap
	}

	upsertGlobalQuery := `INSERT INTO bot_config_state (config_key, config_json)
		 VALUES ($1, $2::jsonb)
		 ON CONFLICT (config_key) DO UPDATE
		 SET config_json = EXCLUDED.config_json,
		     updated_at = NOW()`

	slog.Debug("Granular I/O inspection: Dump of conditional state SQL query (Upsert Global)",
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
		log.EmitBlockingError("Blocking structural failure: Base topology upsert executable command rejected", errWrap, log.GenerateRequestID())
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
			log.EmitBlockingError("Blocking structural failure: Marshal operation failed on isolated hierarchical scope", errWrap, log.GenerateRequestID())
			return errWrap
		}

		slog.Debug("Granular transient state inspection: Injecting atomic relational branch for guild node",
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
			log.EmitBlockingError("Blocking structural failure: Collision or transactional obstruction bound to sub-level", errWrap, log.GenerateRequestID())
			return errWrap
		}
	}

	if err := tx.Commit(ctx); err != nil {
		errWrap := fmt.Errorf("commit config save tx: %w", err)
		log.EmitBlockingError("Blocking structural failure: Consolidative 2PC protocol rejected; commit failed and locked state at source", errWrap, log.GenerateRequestID())
		return errWrap
	}

	slog.Info("Architectural state transition: SQL ACID transaction completed, I/O pipeline drained")
	return nil
}

// Exists exists.
func (s *PostgresConfigStore) Exists() (bool, error) {
	if s == nil || s.db == nil {
		err := fmt.Errorf("postgres config store database handle is nil")
		log.EmitBlockingError("Blocking structural failure: Static probe failed on node referential integrity", err, log.GenerateRequestID())
		return false, err
	}

	var exists bool
	queryExists := `SELECT EXISTS(SELECT 1 FROM bot_config_state WHERE config_key = $1)`

	slog.Debug("Granular I/O inspection: Conditional tracking of relational SQL verification",
		slog.String("query", queryExists),
		slog.String("param_1", s.key),
	)

	if err := s.db.QueryRow(
		context.Background(),
		queryExists,
		s.key,
	).Scan(&exists); err != nil {
		errWrap := fmt.Errorf("check config row in postgres: %w", err)
		log.EmitBlockingError("Blocking structural failure: Scalar boolean query collapsed during scan", errWrap, log.GenerateRequestID())
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
