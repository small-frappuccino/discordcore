package control

import (
	"context"
	"fmt"
	"slices"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func (s *Server) applyGlobalSettingsUpdate(payload updateGlobalSettingsRequest) (files.BotConfig, error) {
	current := s.configManager.SnapshotConfig()
	if payload.ConfigVersion == nil {
		return files.BotConfig{}, fmt.Errorf("optimistic concurrency control: config_version required")
	} else if *payload.ConfigVersion != current.ConfigVersion {
		return files.BotConfig{}, fmt.Errorf("optimistic concurrency control: config_version mismatch")
	}

	updated, err := s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		if payload.ConfigVersion != nil {
			cfg.ConfigVersion++
		}
		if payload.Features != nil {
			cfg.Features = *payload.Features
		}
		if payload.Runtime != nil {
			next, err := files.NormalizeRuntimeConfig(flattenRuntimeSettingsSections(*payload.Runtime))
			if err != nil {
				return fmt.Errorf("Server.applyGlobalSettingsUpdate: %w", err)
			}
			cfg.RuntimeConfig = next
		}
		return nil
	})
	if err != nil {
		return files.BotConfig{}, fmt.Errorf("failed to update global settings: %w", err)
	}

	return updated, nil
}

func (s *Server) registerGuildSettings(ctx context.Context, auth requestAuthorization, guildID string) (files.BotConfig, error) {
	if s.guildRegistration == nil {
		return files.BotConfig{}, fmt.Errorf("%w: bootstrap is not configured for guild_id=%s", errGuildRegistrationUnavailable, guildID)
	}

	if err := s.guildRegistration(ctx, guildID); err != nil {
		return files.BotConfig{}, fmt.Errorf("failed to register guild settings: %w", err)
	}

	updated := s.configManager.SnapshotConfig()
	if _, ok := findGuildSettings(updated, guildID); !ok {
		return files.BotConfig{}, fmt.Errorf("registered guild settings not found for %s", guildID)
	}

	return updated, nil
}

func (s *Server) applyGuildSettingsUpdate(
	ctx context.Context,
	auth requestAuthorization,
	guildID string,
	payload updateGuildSettingsRequest,
	updateBotInstanceTokens bool,
) (files.BotConfig, bool, error) {
	current := s.configManager.SnapshotConfig()
	if guildDisk, ok := findGuildSettings(current, guildID); !ok {
		if s.guildRegistration != nil {
			if err := s.guildRegistration(ctx, guildID); err != nil {
				return files.BotConfig{}, false, fmt.Errorf("failed to auto-register guild settings: %w", err)
			}
		} else {
			return files.BotConfig{}, false, fmt.Errorf("guild settings not found for %s and registration is unavailable", guildID)
		}
	} else if payload.ConfigVersion == nil {
		return files.BotConfig{}, false, fmt.Errorf("optimistic concurrency control: config_version required")
	} else if *payload.ConfigVersion != guildDisk.ConfigVersion {
		return files.BotConfig{}, false, fmt.Errorf("optimistic concurrency control: config_version mismatch")
	}

	invalidateAccessCache := false
	updated, err := s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		guild, ok := findGuildSettingsMutable(cfg, guildID)
		if !ok {
			return fmt.Errorf("%w: register this guild first (guild_id=%s)", errGuildRegistrationRequired, guildID)
		}
		if payload.ConfigVersion != nil {
			guild.ConfigVersion++
		}
		if updateBotInstanceTokens {
			if guild.BotInstanceTokens == nil {
				guild.BotInstanceTokens = make(map[string]files.EncryptedString)
			}
			for k, v := range *payload.BotInstanceTokens {
				if v == "" {
					delete(guild.BotInstanceTokens, k)
				} else {
					guild.BotInstanceTokens[k] = files.EncryptedString(v)
				}
			}
		}
		if payload.BotInstanceStatuses != nil {
			if guild.BotInstanceStatuses == nil {
				guild.BotInstanceStatuses = make(map[string]string)
			}
			for k, v := range *payload.BotInstanceStatuses {
				if v == "" {
					delete(guild.BotInstanceStatuses, k)
				} else {
					guild.BotInstanceStatuses[k] = v
				}
			}
		}
		if payload.FeatureRouting != nil {
			guild.FeatureRouting = *payload.FeatureRouting
		}
		if payload.Features != nil {
			guild.Features = *payload.Features
		}
		if payload.Channels != nil {
			guild.Channels = *payload.Channels
		}
		if payload.Roles != nil {
			invalidateAccessCache = dashboardAccessRolesChanged(guild.Roles, *payload.Roles)
			guild.Roles = *payload.Roles
		}
		if payload.Stats != nil {
			guild.Stats = *payload.Stats
		}
		if payload.Cache != nil {
			guild.RolesCacheTTL = payload.Cache.RolesCacheTTL
			guild.MemberCacheTTL = payload.Cache.MemberCacheTTL
			guild.GuildCacheTTL = payload.Cache.GuildCacheTTL
			guild.ChannelCacheTTL = payload.Cache.ChannelCacheTTL
		}
		if payload.UserPrune != nil {
			guild.UserPrune = *payload.UserPrune
		}
		if payload.PartnerBoard != nil {
			next, err := files.NormalizePartnerBoardConfig(*payload.PartnerBoard)
			if err != nil {
				return fmt.Errorf("Server.applyGuildSettingsUpdate: %w", err)
			}
			guild.PartnerBoard = next
		}
		if payload.Runtime != nil {
			next, err := files.NormalizeRuntimeConfig(flattenRuntimeSettingsSections(*payload.Runtime))
			if err != nil {
				return fmt.Errorf("Server.applyGuildSettingsUpdate: %w", err)
			}
			guild.RuntimeConfig = next
		}

		for feature, instanceID := range guild.FeatureRouting {
			if _, ok := guild.BotInstanceTokens[instanceID]; !ok {
				delete(guild.FeatureRouting, feature)
			}
		}

		return nil
	})
	if err != nil {
		return files.BotConfig{}, false, fmt.Errorf("failed to update guild settings: %w", err)
	}

	if _, ok := findGuildSettings(updated, guildID); !ok {
		return files.BotConfig{}, false, fmt.Errorf("updated guild settings not found for %s", guildID)
	}

	return updated, invalidateAccessCache, nil
}

func (s *Server) deleteGuildSettings(guildID string) (bool, error) {
	invalidateAccessCache := false
	_, err := s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID != guildID {
				continue
			}
			invalidateAccessCache = dashboardAccessRolesConfigured(cfg.Guilds[idx].Roles)
			cfg.Guilds = slices.Delete(cfg.Guilds, idx, idx+1)
			return nil
		}
		return fmt.Errorf("%w: guild_id=%s", files.ErrGuildConfigNotFound, guildID)
	})
	if err != nil {
		return false, fmt.Errorf("failed to delete guild settings: %w", err)
	}
	return invalidateAccessCache, nil
}
