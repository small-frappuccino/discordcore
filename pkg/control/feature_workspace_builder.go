package control

import (
	"fmt"
	"slices"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logging"
	"github.com/small-frappuccino/discordgo"
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

// Workspace workspaces.
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
		return featureWorkspace{}, fmt.Errorf("featureWorkspaceBuilder.Workspace: %w", err)
	}
	return builder.buildWorkspace(cfg, guildID, session)
}

// Feature features.
func (builder *featureWorkspaceBuilder) Feature(guildID, featureID string) (featureRecord, error) {
	if builder == nil || builder.configManager == nil {
		return featureRecord{}, fmt.Errorf("config manager unavailable")
	}

	cfg := builder.configManager.SnapshotConfig()
	return builder.FeatureFromConfig(cfg, guildID, featureID)
}

// FeatureFromConfig features from config.
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
		return featureRecord{}, fmt.Errorf("featureWorkspaceBuilder.FeatureFromConfig: %w", err)
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
			return featureWorkspace{}, fmt.Errorf("featureWorkspaceBuilder.buildWorkspace: %w", err)
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

	var configVersion int64
	if guildID != "" {
		if guild, ok := findGuildSettings(cfg, guildID); ok {
			configVersion = guild.ConfigVersion
		}
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
		ConfigVersion:         configVersion,
		Readiness:             readiness,
		Blockers:              blockers,
		Details:               builder.buildFeatureDetails(&cfg, guildID, def),
		EditableFields:        featureEditableFields(def, guildID),
	}
	if len(record.Blockers) == 0 {
		record.Blockers = nil
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
) *featureDetails {
	if def.LogEvent != "" {
		return buildLogFeatureDetails(cfg, guildID, def.LogEvent)
	}

	handler, ok := featureDetailBuilders[def.ID]
	if !ok {
		return nil
	}
	return handler(cfg, guildID)
}

var featureDetailBuilders = map[string]func(*files.BotConfig, string) *featureDetails{
	"services.automod": func(_ *files.BotConfig, _ string) *featureDetails {
		return &featureDetails{Mode: "logging_only"}
	},
	"moderation.mute_role": func(cfg *files.BotConfig, guildID string) *featureDetails {
		if guildID == "" {
			return nil
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			return &featureDetails{RoleID: strings.TrimSpace(guild.Roles.MuteRole)}
		}
		return nil
	},
	"services.commands": func(cfg *files.BotConfig, guildID string) *featureDetails {
		if guildID == "" {
			return nil
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok && strings.TrimSpace(guild.Channels.Commands) != "" {
			return &featureDetails{ChannelID: strings.TrimSpace(guild.Channels.Commands)}
		}
		return nil
	},

	"message_cache.cleanup_on_startup": func(cfg *files.BotConfig, guildID string) *featureDetails {
		return &featureDetails{RuntimeEnabled: cfg.ResolveRuntimeConfig(guildID).MessageCacheCleanup}
	},
	"message_cache.delete_on_log": func(cfg *files.BotConfig, guildID string) *featureDetails {
		return &featureDetails{RuntimeEnabled: cfg.ResolveRuntimeConfig(guildID).MessageDeleteOnLog}
	},
	"presence_watch.bot": func(cfg *files.BotConfig, guildID string) *featureDetails {
		return &featureDetails{WatchBot: cfg.ResolveRuntimeConfig(guildID).PresenceWatchBot}
	},
	"presence_watch.user": func(cfg *files.BotConfig, guildID string) *featureDetails {
		return &featureDetails{UserID: strings.TrimSpace(cfg.ResolveRuntimeConfig(guildID).PresenceWatchUserID)}
	},
	"safety.bot_role_perm_mirror": func(cfg *files.BotConfig, guildID string) *featureDetails {
		rc := cfg.ResolveRuntimeConfig(guildID)
		return &featureDetails{
			ActorRoleID:     strings.TrimSpace(rc.BotRolePermMirrorActorRoleID),
			RuntimeDisabled: rc.DisableBotRolePermMirror,
		}
	},
	"backfill.enabled": func(cfg *files.BotConfig, guildID string) *featureDetails {
		rc := cfg.ResolveRuntimeConfig(guildID)
		out := &featureDetails{
			StartDay:    strings.TrimSpace(rc.BackfillStartDay),
			InitialDate: strings.TrimSpace(rc.BackfillInitialDate),
		}
		if guildID == "" {
			out.ChannelID = strings.TrimSpace(cfg.RuntimeConfig.BackfillChannelID)
			return out
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			out.ChannelID = strings.TrimSpace(guild.Channels.BackfillChannelID())
		}
		return out
	},
	"stats_channels": func(cfg *files.BotConfig, guildID string) *featureDetails {
		if guildID == "" {
			return nil
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			return &featureDetails{
				ConfiguredChannelCount: len(guild.Stats.Channels),
				Channels:               buildStatsChannelDetails(guild.Stats.Channels),
			}
		}
		return nil
	},
	"auto_role_assignment": func(cfg *files.BotConfig, guildID string) *featureDetails {
		if guildID == "" {
			return nil
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			out := &featureDetails{
				ConfigEnabled:     guild.Roles.AutoAssignment.Enabled,
				TargetRoleID:      strings.TrimSpace(guild.Roles.AutoAssignment.TargetRoleID),
				RequiredRoleIDs:   slices.Clone(guild.Roles.AutoAssignment.RequiredRoles),
				RequiredRoleCount: len(guild.Roles.AutoAssignment.RequiredRoles),
				BoosterRoleID:     strings.TrimSpace(guild.Roles.BoosterRole),
			}
			if len(guild.Roles.AutoAssignment.RequiredRoles) > 0 {
				out.LevelRoleID = strings.TrimSpace(guild.Roles.AutoAssignment.RequiredRoles[0])
			}
			return out
		}
		return nil
	},
	"user_prune": func(cfg *files.BotConfig, guildID string) *featureDetails {
		if guildID == "" {
			return nil
		}
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			return &featureDetails{ConfigEnabled: guild.UserPrune.Enabled}
		}
		return nil
	},
}

func buildLogFeatureDetails(
	cfg *files.BotConfig,
	guildID string,
	logEvent logging.LogEventType,
) *featureDetails {
	capability, ok := logging.LogEventCapabilities()[logEvent]
	if !ok {
		return nil
	}

	out := &featureDetails{
		RequiresChannel:         capability.RequiresChannel,
		RequiredIntentsMask:     capability.RequiredIntentsMask,
		RequiredPermissionsMask: capability.RequiredPermsMask,
		ValidateChannelPerms:    capability.ValidateChannelPerms,
		ExclusiveModeration:     capability.RequireExclusiveModeration,
	}
	if len(capability.Toggles) > 0 {
		out.RuntimeTogglePath = capability.Toggles[0]
	}
	if guildID != "" {
		if guild, ok := findGuildSettings(*cfg, guildID); ok {
			if channelID := logFeatureChannelID(&guild, logEvent); channelID != "" {
				out.ChannelID = channelID
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

func logFeatureChannelID(guild *files.GuildConfig, eventType logging.LogEventType) string {
	if guild == nil {
		return ""
	}
	switch eventType {
	case logging.LogEventAvatarChange:
		return strings.TrimSpace(guild.Channels.AvatarLogging)
	case logging.LogEventRoleChange:
		return strings.TrimSpace(guild.Channels.RoleUpdate)
	case logging.LogEventMemberJoin:
		return strings.TrimSpace(guild.Channels.MemberJoin)
	case logging.LogEventMemberLeave:
		return strings.TrimSpace(guild.Channels.MemberLeave)
	case logging.LogEventMessageEdit:
		return strings.TrimSpace(guild.Channels.MessageEdit)
	case logging.LogEventMessageDelete:
		return strings.TrimSpace(guild.Channels.MessageDelete)
	case logging.LogEventAutomodAction:
		return strings.TrimSpace(guild.Channels.AutomodAction)
	case logging.LogEventModerationCase:
		return strings.TrimSpace(guild.Channels.ModerationCase)
	case logging.LogEventCleanAction:
		return strings.TrimSpace(guild.Channels.CleanAction)
	default:
		return ""
	}
}
