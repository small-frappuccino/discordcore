package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

// stringToFeature mapeia o nome do banco para a enum em memória O(1).
func stringToFeature(name string) (core.Feature, bool) {
	switch name {
	case "ban":
		return core.FeatureBan, true
	case "kick":
		return core.FeatureKick, true
	case "timeout":
		return core.FeatureTimeout, true
	case "deafen":
		return core.FeatureDeafen, true
	case "move_member":
		return core.FeatureMoveMember, true
	case "msg_delete":
		return core.FeatureMsgDelete, true
	case "channel_purge":
		return core.FeatureChannelPurge, true
	case "role_add":
		return core.FeatureRoleAdd, true
	case "role_remove":
		return core.FeatureRoleRemove, true
	}
	return 0, false
}

// HydrateRegistry faz a ponte entre a persistência (PostgreSQL) e a RAM (Registry).
func HydrateRegistry[R core.FeatureRepository](ctx context.Context, repo R, registry *core.InMemoryFeatureRegistry) error {
	if slog.Default().Enabled(ctx, slog.LevelInfo) {
		slog.Info("A iniciar a hidratação da tabela de roteamento Multi-Tenant...")
	}

	seq, err := repo.FetchAllActive(ctx)
	if err != nil {
		return fmt.Errorf("não foi possível extrair a configuração da base de dados: %w", err)
	}

	for cfg, err := range seq {
		if err != nil {
			return fmt.Errorf("erro durante iteração de features: %w", err)
		}

		featEnum, ok := stringToFeature(cfg.FeatureName)
		if !ok {
			if slog.Default().Enabled(ctx, slog.LevelWarn) {
				slog.Warn("Feature desconhecida ignorada na hidratação", "feature", cfg.FeatureName)
			}
			continue
		}

		botInstance := core.BotInstance{
			ApplicationID: cfg.ApplicationID,
			GuildID:       cfg.GuildID,
			Token:         core.Token(cfg.BotToken),
		}

		err := registry.UpdateRoute(cfg.GuildID, featEnum, botInstance)
		if err != nil {
			if slog.Default().Enabled(ctx, slog.LevelError) {
				slog.Error("Rejeitada mutação durante hidratação", "guilda", cfg.GuildID, "feature", cfg.FeatureName, "erro", err)
			}
		}
	}

	if slog.Default().Enabled(ctx, slog.LevelInfo) {
		slog.Info("Hidratação concluída")
	}
	return nil
}
