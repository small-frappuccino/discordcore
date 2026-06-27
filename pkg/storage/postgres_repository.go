package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

type PostgresFeatureRepo struct {
	db *sql.DB
}

func NewPostgresFeatureRepo(db *sql.DB) *PostgresFeatureRepo {
	return &PostgresFeatureRepo{
		db: db,
	}
}

// FetchAllActive executa a query em massa para reduzir viagens de I/O de rede.
func (r *PostgresFeatureRepo) FetchAllActive(ctx context.Context) ([]core.GuildFeatureConfig, error) {
	// A query é simples, focada e usa índices adequados no PostgreSQL.
	query := `
		SELECT guild_id, feature_name, application_id, bot_token 
		FROM guild_feature_assignments 
		WHERE is_active = true;
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao executar query de features: %w", err)
	}
	defer rows.Close()

	// Pré-alocamos o slice com um palpite de capacidade para evitar alocações
	// dinâmicas na Heap à medida que o array cresce (ex: cap inicial de 1000).
	configs := make([]core.GuildFeatureConfig, 0, 1000)

	for rows.Next() {
		var cfg core.GuildFeatureConfig
		err := rows.Scan(&cfg.GuildID, &cfg.FeatureName, &cfg.ApplicationID, &cfg.BotToken)
		if err != nil {
			return nil, fmt.Errorf("falha a fazer parsing (scan) da linha: %w", err)
		}

		// NOTA DE SEGURANÇA: Se tivermos encriptação AES no banco de dados para os tokens,
		// este seria o momento exato para chamar `crypto.Decrypt(cfg.BotToken)`.

		configs = append(configs, cfg)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return configs, nil
}
