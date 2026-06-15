package files

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

	slog.Info("Transição de estado arquitetural: Acoplamento do adaptador de armazenamento PostgreSQL isolado para parâmetros de configuração")

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
		emitBlockingError("Falha estrutural bloqueante: Ponteiro nulo bloqueou a inicialização do driver PostgreSQL", err, generateRequestID())
		return cfg, err
	}

	slog.Info("Transição de estado arquitetural: Iniciando carregamento persistente da árvore global",
		slog.String("store_key", s.key),
	)

	var globalRaw []byte
	queryGlobal := `SELECT config_json FROM bot_config_state WHERE config_key = $1`

	slog.Debug("Inspeção granular de I/O: Despejo de query SQL gerada dinamicamente (Load Global)",
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
		emitBlockingError("Falha estrutural bloqueante: Driver SQL rejeitou a leitura de documento global", errWrap, generateRequestID())
		return nil, errWrap
	}

	if err == pgx.ErrNoRows {
		slog.Warn("Degradação mitigada: Documento canônico não encontrado; a matriz adotará a rotina de compensação de estrutura vazia",
			slog.String("missing_key", s.key),
		)
	} else if len(globalRaw) > 0 {
		slog.Debug("Inspeção granular de estado transiente: Desserialização bruta ativada",
			slog.Int("payload_bytes", len(globalRaw)),
		)
		if err := json.Unmarshal(globalRaw, cfg); err != nil {
			errWrap := fmt.Errorf("decode global config row from postgres: %w", err)
			emitBlockingError("Falha estrutural bloqueante: Parsing de documento JSON corrompido no bloco global", errWrap, generateRequestID())
			return nil, errWrap
		}
	}
	cfg.Guilds = []GuildConfig{}

	queryGuilds := `SELECT config_json FROM guild_configs`
	slog.Debug("Inspeção granular de I/O: Despejo de query SQL gerada dinamicamente (Load Guilds)",
		slog.String("query", queryGuilds),
	)

	rows, err := s.db.Query(
		context.Background(),
		queryGuilds,
	)
	if err != nil {
		errWrap := fmt.Errorf("query guild_configs: %w", err)
		emitBlockingError("Falha estrutural bloqueante: Subgrafo de configurações das instâncias rejeitado pelo servidor relacional", errWrap, generateRequestID())
		return nil, errWrap
	}
	defer rows.Close()

	for rows.Next() {
		var guildRaw []byte
		if err := rows.Scan(&guildRaw); err != nil {
			errWrap := fmt.Errorf("scan guild_configs row: %w", err)
			emitBlockingError("Falha estrutural bloqueante: Cursor I/O estourou durante rastreamento bidirecional da tabela", errWrap, generateRequestID())
			return nil, errWrap
		}
		var guildCfg GuildConfig
		if err := json.Unmarshal(guildRaw, &guildCfg); err != nil {
			errWrap := fmt.Errorf("decode guild_configs json: %w", err)
			emitBlockingError("Falha estrutural bloqueante: Parsing de documento JSON corrompido em sub-nó de guilda", errWrap, generateRequestID())
			return nil, errWrap
		}
		cfg.Guilds = append(cfg.Guilds, guildCfg)
	}
	if err := rows.Err(); err != nil {
		errWrap := fmt.Errorf("iterate guild_configs rows: %w", err)
		emitBlockingError("Falha estrutural bloqueante: Pipe de paginação SQL reportou contenção não recuperável", errWrap, generateRequestID())
		return nil, errWrap
	}

	return cfg, nil
}

// Save saves.
func (s *PostgresConfigStore) Save(cfg *BotConfig) error {
	if cfg == nil {
		err := fmt.Errorf("cannot save nil config")
		emitBlockingError("Falha estrutural bloqueante: Tentativa de persistência com matriz global nula", err, generateRequestID())
		return err
	}
	if s == nil || s.db == nil {
		err := fmt.Errorf("postgres config store database handle is nil")
		emitBlockingError("Falha estrutural bloqueante: Sincronização bloqueada por driver relacional nulo", err, generateRequestID())
		return err
	}

	slog.Info("Transição de estado arquitetural: Inicializando transação unificada ACID para gravação da matriz I/O",
		slog.Int("guilds_payload", len(cfg.Guilds)),
	)

	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		errWrap := fmt.Errorf("begin config save tx: %w", err)
		emitBlockingError("Falha estrutural bloqueante: Negociação de transação abortada pelo SGBD", errWrap, generateRequestID())
		return errWrap
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
			slog.Warn("Degradação mitigada interceptada: Rollback compensatório de transação exposta falhou sobre pipe TCP",
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
		emitBlockingError("Falha estrutural bloqueante: Operação marshal anulou o buffer de escrita primária", errWrap, generateRequestID())
		return errWrap
	}

	upsertGlobalQuery := `INSERT INTO bot_config_state (config_key, config_json)
		 VALUES ($1, $2::jsonb)
		 ON CONFLICT (config_key) DO UPDATE
		 SET config_json = EXCLUDED.config_json,
		     updated_at = NOW()`

	slog.Debug("Inspeção granular de I/O: Despejo de query SQL de estado condicional (Upsert Global)",
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
		emitBlockingError("Falha estrutural bloqueante: Commando executável upsert de topologia base rejeitado", errWrap, generateRequestID())
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
			emitBlockingError("Falha estrutural bloqueante: Operação marshal falhou sobre o escopo hierárquico isolado", errWrap, generateRequestID())
			return errWrap
		}

		slog.Debug("Inspeção granular de estado transiente: Injetando ramificação relacional atômica para nó da guilda",
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
			emitBlockingError("Falha estrutural bloqueante: Colisão ou obstrução transacional atrelada ao sub-nível", errWrap, generateRequestID())
			return errWrap
		}
	}

	if err := tx.Commit(ctx); err != nil {
		errWrap := fmt.Errorf("commit config save tx: %w", err)
		emitBlockingError("Falha estrutural bloqueante: Protocolo 2PC consolidativo rejeitado; o commit falhou e travou o estado na origem", errWrap, generateRequestID())
		return errWrap
	}

	slog.Info("Transição de estado arquitetural: Transação ACID SQL completada, pipeline I/O drenado")
	return nil
}

// Exists exists.
func (s *PostgresConfigStore) Exists() (bool, error) {
	if s == nil || s.db == nil {
		err := fmt.Errorf("postgres config store database handle is nil")
		emitBlockingError("Falha estrutural bloqueante: Probe estático falhou sobre integridade referencial do nó", err, generateRequestID())
		return false, err
	}

	var exists bool
	queryExists := `SELECT EXISTS(SELECT 1 FROM bot_config_state WHERE config_key = $1)`

	slog.Debug("Inspeção granular de I/O: Rastreamento condicional de verificação SQL relacional",
		slog.String("query", queryExists),
		slog.String("param_1", s.key),
	)

	if err := s.db.QueryRow(
		context.Background(),
		queryExists,
		s.key,
	).Scan(&exists); err != nil {
		errWrap := fmt.Errorf("check config row in postgres: %w", err)
		emitBlockingError("Falha estrutural bloqueante: Query booleana escalar colapsou na varredura", errWrap, generateRequestID())
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
