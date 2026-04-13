package control

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	discordlogging "github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type featureWorkspaceBuilder struct {
	configManager   *files.ConfigManager
	discordSessions discordSessionResolver
}

func newFeatureWorkspaceBuilder(
	configManager *files.ConfigManager,
	discordSessions discordSessionResolver,
) *featureWorkspaceBuilder {
	return &featureWorkspaceBuilder{
		configManager:   configManager,
		discordSessions: discordSessions,
	}
}

func (builder *featureWorkspaceBuilder) Workspace(guildID string) (featureWorkspace, error) {
	if builder == nil || builder.configManager == nil {
		return featureWorkspace{}, fmt.Errorf("config manager unavailable")
	}

	cfg := builder.configManager.SnapshotConfig()
	if guildID != "" {
		if _, ok := findGuildSettings(cfg, guildID); !ok {
			return featureWorkspace{}, fmt.Errorf("%w: guild_id=%s", files.ErrGuildConfigNotFound, guildID)
		}
	}

	session, err := builder.discordSessionForGuild(guildID)
	if err != nil {
		return featureWorkspace{}, err
	}
	return builder.buildWorkspace(cfg, guildID, session)
}

func (builder *featureWorkspaceBuilder) Feature(guildID, featureID string) (featureRecord, error) {
	if builder == nil || builder.configManager == nil {
		return featureRecord{}, fmt.Errorf("config manager unavailable")
	}

	cfg := builder.configManager.SnapshotConfig()
	return builder.FeatureFromConfig(cfg, guildID, featureID)
}

func (builder *featureWorkspaceBuilder) FeatureFromConfig(
	cfg files.BotConfig,
	guildID string,
	featureID string,
) (featureRecord, error) {
	if guildID != "" {
		if _, ok := findGuildSettings(cfg, guildID); !ok {
			return featureRecord{}, fmt.Errorf("%w: guild_id=%s", files.ErrGuildConfigNotFound, guildID)
		}
	}

	session, err := builder.discordSessionForGuild(guildID)
	if err != nil {
		return featureRecord{}, err
	}
	return builder.buildSingleFeatureRecord(cfg, guildID, featureID, session)
}

func (builder *featureWorkspaceBuilder) discordSessionForGuild(guildID string) (*discordgo.Session, error) {
	if builder == nil || builder.discordSessions == nil {
		return nil, nil
	}
	return builder.discordSessions(guildID)
}

func (builder *featureWorkspaceBuilder) buildWorkspace(
	cfg files.BotConfig,
	guildID string,
	session *discordgo.Session,
) (featureWorkspace, error) {
	records := make([]featureRecord, 0, len(featureDefinitions))
	for _, def := range featureDefinitions {
		record, err := builder.buildFeatureRecord(cfg, guildID, def, session)
		if err != nil {
			return featureWorkspace{}, err
		}
		records = append(records, record)
	}

	scope := "global"
	if guildID != "" {
		scope = "guild"
	}
	return featureWorkspace{
		Scope:    scope,
		GuildID:  guildID,
		Features: records,
	}, nil
}

func (builder *featureWorkspaceBuilder) buildSingleFeatureRecord(
	cfg files.BotConfig,
	guildID string,
	featureID string,
	session *discordgo.Session,
) (featureRecord, error) {
	def, ok := featureDefinitionsByID[featureID]
	if !ok {
		return featureRecord{}, fmt.Errorf("%w: %s", errUnknownFeatureID, featureID)
	}
	return builder.buildFeatureRecord(cfg, guildID, def, session)
}

func (builder *featureWorkspaceBuilder) buildFeatureRecord(
	cfg files.BotConfig,
	guildID string,
	def featureDefinition,
	session *discordgo.Session,
) (featureRecord, error) {
	if guildID != "" {
		if _, ok := findGuildSettings(cfg, guildID); !ok {
			return featureRecord{}, fmt.Errorf("%w: guild_id=%s", files.ErrGuildConfigNotFound, guildID)
		}
	}

	effectiveEnabled := resolvedFeatureValue(&cfg, guildID, def.ID)
	readiness, blockers := buildFeatureReadiness(&cfg, builder.configManager, guildID, def, effectiveEnabled, session)
	scope := "global"
	if guildID != "" {
		scope = "guild"
	}

	record := featureRecord{
		ID:                    def.ID,
		Category:              def.Category,
		Label:                 def.Label,
		Description:           def.Description,
		Scope:                 scope,
		Area:                  def.Area,
		Tags:                  slices.Clone(def.Tags),
		SupportsGuildOverride: def.SupportsGuildOverride,
		OverrideState:         featureOverrideState(&cfg, guildID, def.ID),
		EffectiveEnabled:      effectiveEnabled,
		EffectiveSource:       featureEffectiveSource(&cfg, guildID, def.ID),
		Readiness:             readiness,
		Blockers:              blockers,
		Details:               builder.buildFeatureDetails(&cfg, guildID, def),
		EditableFields:        featureEditableFields(def, guildID),
	}
	if len(record.Blockers) == 0 {
		record.Blockers = nil
	}
	if len(record.Details) == 0 {
		record.Details = nil
	}
	if len(record.EditableFields) == 0 {
		record.EditableFields = nil
	}
	return record, nil
}

func (builder *featureWorkspaceBuilder) buildFeatureDetails(
	cfg *files.BotConfig,
	guildID string,
	def featureDefinition,
) map[string]any {
	if def.LogEvent != "" {
		return buildLogFeatureDetails(cfg, guildID, def.LogEvent)
	}

	handler, ok := featureDetailBuilders[def.ID]
	if !ok {
		return map[string]any{}
	}
	return handler(cfg, guildID)
}

var featureDetailBuilders = map[string]func(*files.BotConfig, string) map[string]any{
	"services.automod": func(_ *files.BotConfig, _ string) map[string]any {
		return map[string]any{"mode": "logging_only"}
	},
	"moderation.mute_role": func(cfg *files.BotConfig, guildID string) map[string]any {
		if guildID == "" {
			return map[string]any{}
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			return map[string]any{"role_id": strings.TrimSpace(guild.Roles.MuteRole)}
		}
		return map[string]any{}
	},
	"services.commands": func(cfg *files.BotConfig, guildID string) map[string]any {
		if guildID == "" {
			return map[string]any{}
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok && strings.TrimSpace(guild.Channels.Commands) != "" {
			return map[string]any{"channel_id": strings.TrimSpace(guild.Channels.Commands)}
		}
		return map[string]any{}
	},
	"services.admin_commands": func(cfg *files.BotConfig, guildID string) map[string]any {
		if guildID == "" {
			return map[string]any{}
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			return map[string]any{
				"allowed_role_ids":   slices.Clone(guild.Roles.Allowed),
				"allowed_role_count": len(guild.Roles.Allowed),
			}
		}
		return map[string]any{}
	},
	"message_cache.cleanup_on_startup": func(cfg *files.BotConfig, guildID string) map[string]any {
		return map[string]any{"runtime_enabled": cfg.ResolveRuntimeConfig(guildID).MessageCacheCleanup}
	},
	"message_cache.delete_on_log": func(cfg *files.BotConfig, guildID string) map[string]any {
		return map[string]any{"runtime_enabled": cfg.ResolveRuntimeConfig(guildID).MessageDeleteOnLog}
	},
	"presence_watch.bot": func(cfg *files.BotConfig, guildID string) map[string]any {
		return map[string]any{"watch_bot": cfg.ResolveRuntimeConfig(guildID).PresenceWatchBot}
	},
	"presence_watch.user": func(cfg *files.BotConfig, guildID string) map[string]any {
		return map[string]any{"user_id": strings.TrimSpace(cfg.ResolveRuntimeConfig(guildID).PresenceWatchUserID)}
	},
	"safety.bot_role_perm_mirror": func(cfg *files.BotConfig, guildID string) map[string]any {
		rc := cfg.ResolveRuntimeConfig(guildID)
		return map[string]any{
			"actor_role_id":    strings.TrimSpace(rc.BotRolePermMirrorActorRoleID),
			"runtime_disabled": rc.DisableBotRolePermMirror,
		}
	},
	"backfill.enabled": func(cfg *files.BotConfig, guildID string) map[string]any {
		rc := cfg.ResolveRuntimeConfig(guildID)
		out := map[string]any{
			"start_day":    strings.TrimSpace(rc.BackfillStartDay),
			"initial_date": strings.TrimSpace(rc.BackfillInitialDate),
		}
		if guildID == "" {
			out["channel_id"] = strings.TrimSpace(cfg.RuntimeConfig.BackfillChannelID)
			return out
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			out["channel_id"] = strings.TrimSpace(guild.Channels.BackfillChannelID())
		}
		return out
	},
	"stats_channels": func(cfg *files.BotConfig, guildID string) map[string]any {
		if guildID == "" {
			return map[string]any{}
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			return map[string]any{
				"config_enabled":           guild.Stats.Enabled,
				"update_interval_mins":     guild.Stats.UpdateIntervalMins,
				"configured_channel_count": len(guild.Stats.Channels),
				"channels":                 buildStatsChannelDetails(guild.Stats.Channels),
			}
		}
		return map[string]any{}
	},
	"auto_role_assignment": func(cfg *files.BotConfig, guildID string) map[string]any {
		if guildID == "" {
			return map[string]any{}
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			out := map[string]any{
				"config_enabled":      guild.Roles.AutoAssignment.Enabled,
				"target_role_id":      strings.TrimSpace(guild.Roles.AutoAssignment.TargetRoleID),
				"required_role_ids":   slices.Clone(guild.Roles.AutoAssignment.RequiredRoles),
				"required_role_count": len(guild.Roles.AutoAssignment.RequiredRoles),
				"booster_role_id":     strings.TrimSpace(guild.Roles.BoosterRole),
			}
			if len(guild.Roles.AutoAssignment.RequiredRoles) > 0 {
				out["level_role_id"] = strings.TrimSpace(guild.Roles.AutoAssignment.RequiredRoles[0])
			}
			return out
		}
		return map[string]any{}
	},
	"user_prune": func(cfg *files.BotConfig, guildID string) map[string]any {
		if guildID == "" {
			return map[string]any{}
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			prune := guild.UserPrune
			return map[string]any{
				"config_enabled":     prune.Enabled,
				"grace_days":         prune.GraceDays,
				"scan_interval_mins": prune.ScanIntervalMins,
				"initial_delay_secs": prune.InitialDelaySecs,
				"kicks_per_second":   prune.KicksPerSecond,
				"max_kicks_per_run":  prune.MaxKicksPerRun,
				"exempt_role_ids":    slices.Clone(prune.ExemptRoleIDs),
				"exempt_role_count":  len(prune.ExemptRoleIDs),
				"dry_run":            prune.DryRun,
			}
		}
		return map[string]any{}
	},
}

func buildLogFeatureDetails(
	cfg *files.BotConfig,
	guildID string,
	logEvent discordlogging.LogEventType,
) map[string]any {
	out := map[string]any{}

	capability, ok := discordlogging.LogEventCapabilities()[logEvent]
	if !ok {
		return out
	}

	out["requires_channel"] = capability.RequiresChannel
	out["required_intents_mask"] = capability.RequiredIntentsMask
	out["required_permissions_mask"] = capability.RequiredPermsMask
	out["validate_channel_permissions"] = capability.ValidateChannelPerms
	out["exclusive_moderation_channel"] = capability.RequireExclusiveModeration
	if len(capability.Toggles) > 0 {
		out["runtime_toggle_path"] = capability.Toggles[0]
	}
	if guildID != "" {
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			if channelID := logFeatureChannelID(&guild, logEvent); channelID != "" {
				out["channel_id"] = channelID
			}
		}
	}

	return out
}

func buildStatsChannelDetails(channels []files.StatsChannelConfig) []featureStatsChannelDetail {
	if len(channels) == 0 {
		return []featureStatsChannelDetail{}
	}

	out := make([]featureStatsChannelDetail, 0, len(channels))
	for _, channel := range channels {
		out = append(out, featureStatsChannelDetail{
			ChannelID:    strings.TrimSpace(channel.ChannelID),
			Label:        strings.TrimSpace(channel.Label),
			NameTemplate: strings.TrimSpace(channel.NameTemplate),
			MemberType:   strings.TrimSpace(channel.MemberType),
			RoleID:       strings.TrimSpace(channel.RoleID),
		})
	}
	return out
}
