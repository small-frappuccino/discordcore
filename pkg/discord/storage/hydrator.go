package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

// HydrateRegistry faz a ponte entre a persistência (PostgreSQL) e a RAM (Registry).
func HydrateRegistry(ctx context.Context, repo FeatureRepository, registry *core.InMemoryFeatureRegistry) error {
	slog.Info("A iniciar a hidratação da tabela de roteamento Multi-Tenant...")

	configs, err := repo.FetchAllActive(ctx)
	if err != nil {
		return fmt.Errorf("não foi possível extrair a configuração da base de dados: %w", err)
	}

	for _, cfg := range configs {
		// Convertemos o DTO do banco para o modelo de domínio em memória
		botInstance := &core.BotInstance{
			ApplicationID: cfg.ApplicationID,
			GuildID:       cfg.GuildID,
			Token:         cfg.BotToken,
		}

		// Inserimos atómicamente no mapa protegido por RWMutex
		registry.UpdateRoute(cfg.GuildID, cfg.FeatureName, botInstance)
	}

	slog.Info("Hidratação concluída", "total_rotas_carregadas", len(configs))
	return nil
}
