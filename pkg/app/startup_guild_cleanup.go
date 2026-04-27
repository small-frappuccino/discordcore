package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func pruneStartupGuildReferences(ctx context.Context, configManager *files.ConfigManager, store *storage.Store) ([]string, error) {
	if configManager == nil {
		return nil, nil
	}

	removedGuildIDs := disallowedConfiguredGuildIDs(configManager.Config())
	if len(removedGuildIDs) == 0 {
		return nil, nil
	}

	if _, err := configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		if cfg == nil || len(cfg.Guilds) == 0 {
			return nil
		}

		filtered := make([]files.GuildConfig, 0, len(cfg.Guilds))
		for _, guild := range cfg.Guilds {
			if !isAllowedRuntimeGuild(guild.GuildID) {
				continue
			}
			filtered = append(filtered, guild)
		}
		cfg.Guilds = filtered
		return nil
	}); err != nil {
		return nil, fmt.Errorf("prune disallowed guild config: %w", err)
	}

	if store == nil {
		return removedGuildIDs, nil
	}

	for _, guildID := range removedGuildIDs {
		if err := deleteGuildReferences(ctx, nil, store, guildID); err != nil {
			return nil, err
		}
	}

	return removedGuildIDs, nil
}

func disallowedConfiguredGuildIDs(cfg *files.BotConfig) []string {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(cfg.Guilds))
	removed := make([]string, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		guildID := strings.TrimSpace(guild.GuildID)
		if guildID == "" || isAllowedRuntimeGuild(guildID) {
			continue
		}
		if _, ok := seen[guildID]; ok {
			continue
		}
		seen[guildID] = struct{}{}
		removed = append(removed, guildID)
	}

	sort.Strings(removed)
	return removed
}

func deleteGuildReferences(ctx context.Context, configManager *files.ConfigManager, store *storage.Store, guildID string) error {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil
	}

	if configManager != nil {
		if _, err := configManager.UpdateConfig(func(cfg *files.BotConfig) error {
			if cfg == nil || len(cfg.Guilds) == 0 {
				return nil
			}

			filtered := make([]files.GuildConfig, 0, len(cfg.Guilds))
			for _, guild := range cfg.Guilds {
				if strings.TrimSpace(guild.GuildID) == guildID {
					continue
				}
				filtered = append(filtered, guild)
			}
			cfg.Guilds = filtered
			return nil
		}); err != nil {
			return fmt.Errorf("delete guild config for %s: %w", guildID, err)
		}
	}

	if store != nil {
		if err := store.DeleteGuildData(ctx, guildID); err != nil {
			return fmt.Errorf("delete guild data for %s: %w", guildID, err)
		}
	}

	return nil
}