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

func processFeature(ctx context.Context, registry *core.InMemoryFeatureRegistry, cfg core.GuildFeatureConfig, err error) error {
	if err != nil {
		return fmt.Errorf("erro durante iteração de features: %w", err)
	}

	featEnum, ok := stringToFeature(cfg.FeatureName)
	if !ok {
		if slog.Default().Enabled(ctx, slog.LevelWarn) {
			slog.LogAttrs(ctx, slog.LevelWarn, "Feature desconhecida ignorada na hidratação", slog.String("feature", cfg.FeatureName))
		}
		return nil
	}

	botInstance := core.BotInstance{
		ApplicationID: cfg.ApplicationID,
		GuildID:       cfg.GuildID,
		Token:         core.Token(cfg.BotToken),
	}

	routeErr := registry.UpdateRoute(cfg.GuildID, featEnum, botInstance)
	if routeErr != nil {
		if slog.Default().Enabled(ctx, slog.LevelError) {
			slog.LogAttrs(ctx, slog.LevelError, "Rejeitada mutação durante hidratação",
				slog.String("guilda", cfg.GuildID),
				slog.String("feature", cfg.FeatureName),
				slog.Any("erro", routeErr))
		}
	}
	return nil
}

// HydrateRegistry faz a ponte entre a persistência (PostgreSQL) e a RAM (Registry).
func HydrateRegistry[R core.FeatureRepository](ctx context.Context, repo R, registry *core.InMemoryFeatureRegistry) error {
	seq, err := repo.FetchAllActive(ctx)
	if err != nil {
		return err
	}

	var loopErr error
	for cfg, err := range seq {
		if loopErr != nil {
			continue
		}
		loopErr = processFeature(ctx, registry, cfg, err)
	}
	return loopErr
}
