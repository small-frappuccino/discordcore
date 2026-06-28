package storage

import (
	"context"
	"database/sql"
	"fmt"
	"iter"

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

// FetchAllActive executa a query em massa para reduzir viagens de I/O de rede
// yieldando de volta pelo iter.Seq2 para garantir alocação zero para slices e propagar erros.
func (r *PostgresFeatureRepo) FetchAllActive(ctx context.Context) (iter.Seq2[core.GuildFeatureConfig, error], error) {
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

	seq := func(yield func(core.GuildFeatureConfig, error) bool) {
		defer rows.Close()
		var cfg core.GuildFeatureConfig
		for rows.Next() {
			err := rows.Scan(&cfg.GuildID, &cfg.FeatureName, &cfg.ApplicationID, &cfg.BotToken)
			if err != nil {
				yield(cfg, err)
				return
			}
			if !yield(cfg, nil) {
				return
			}
		}
	}

	return seq, nil
}
