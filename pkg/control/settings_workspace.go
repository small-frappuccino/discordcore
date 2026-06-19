package control

import (
	"slices"
	"sort"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

type settingsCatalog struct {
	Global []settingsCatalogSection `json:"global"`
	Guild  []settingsCatalogSection `json:"guild"`
}

type settingsCatalogSection struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	Scope               string `json:"scope"`
	Kind                string `json:"kind"`
	SupportsInheritance bool   `json:"supports_inheritance,omitempty"`
	Advanced            bool   `json:"advanced,omitempty"`
}

type settingsOverview struct {
	ConfigPath string                   `json:"config_path"`
	Catalog    settingsCatalog          `json:"catalog"`
	Global     globalSettingsWorkspace  `json:"global"`
	Registry   guildRegistryWorkspace   `json:"registry"`
	Guilds     []configuredGuildSummary `json:"guilds"`
}

type configuredGuildSummary struct {
	GuildID             string `json:"guild_id"`
	ConfiguredChannels  int    `json:"configured_channels"`
	AllowedRoles        int    `json:"allowed_roles"`
	StatsChannels       int    `json:"stats_channels"`
	Partners            int    `json:"partners"`
	HasFeatureOverrides bool   `json:"has_feature_overrides"`
	HasRuntimeOverrides bool   `json:"has_runtime_overrides"`
}

type guildRegistryWorkspace struct {
	Scope           string               `json:"scope"`
	Entries         []guildRegistryEntry `json:"entries"`
	ConfiguredCount int                  `json:"configured_count"`
	AvailableCount  int                  `json:"available_count"`
}

type guildRegistryEntry struct {
	GuildID                 string   `json:"guild_id"`
	Name                    string   `json:"name,omitempty"`
	Icon                    string   `json:"icon,omitempty"`
	Permissions             int64    `json:"permissions"`
	Configured              bool     `json:"configured"`
	AvailableBotInstanceIDs []string `json:"available_bot_instance_ids,omitempty"`
	ConfiguredChannels      int      `json:"configured_channels,omitempty"`
	AllowedRoles            int      `json:"allowed_roles,omitempty"`
	StatsChannels           int      `json:"stats_channels,omitempty"`
	Partners                int      `json:"partners,omitempty"`
	HasFeatureOverrides     bool     `json:"has_feature_overrides,omitempty"`
	HasRuntimeOverrides     bool     `json:"has_runtime_overrides,omitempty"`
}

type guildRegistrySource struct {
	GuildID                 string
	Name                    string
	Icon                    string
	Permissions             int64
	AvailableBotInstanceIDs []string
}

type globalSettingsWorkspace struct {
	Scope         string                  `json:"scope"`
	ConfigVersion int64                   `json:"config_version"`
	Sections      globalSettingsSections  `json:"sections"`
	Effective     globalSettingsEffective `json:"effective"`
}

type globalSettingsSections struct {
	Features files.FeatureToggles    `json:"features"`
	Runtime  runtimeSettingsSections `json:"runtime"`
}

type globalSettingsEffective struct {
	Features files.ResolvedFeatureToggles `json:"features"`
	Runtime  runtimeSettingsSections      `json:"runtime"`
}

type guildSettingsWorkspace struct {
	Scope                   string                 `json:"scope"`
	GuildID                 string                 `json:"guild_id"`
	ConfigVersion           int64                  `json:"config_version"`
	AvailableBotInstanceIDs []string               `json:"available_bot_instance_ids,omitempty"`
	Sections                guildSettingsSections  `json:"sections"`
	Effective               guildSettingsEffective `json:"effective"`
}

type guildSettingsSections struct {
	BotInstanceTokensConfigured map[string]bool           `json:"bot_instance_tokens_configured"`
	BotInstanceStatuses         map[string]string         `json:"bot_instance_statuses,omitempty"`
	FeatureRouting              map[string]string         `json:"feature_routing,omitempty"`
	Features                    files.FeatureToggles      `json:"features"`
	Channels                    files.ChannelsConfig      `json:"channels"`
	Roles                       files.RolesConfig         `json:"roles"`
	Stats                       files.StatsConfig         `json:"stats"`
	Cache                       guildCacheSettingsSection `json:"cache"`
	UserPrune                   files.UserPruneConfig     `json:"user_prune"`
	PartnerBoard                files.PartnerBoardConfig  `json:"partner_board"`
	Runtime                     runtimeSettingsSections   `json:"runtime"`
}

type guildSettingsEffective struct {
	Features files.ResolvedFeatureToggles `json:"features"`
	Runtime  runtimeSettingsSections      `json:"runtime"`
}

type guildCacheSettingsSection struct {
	RolesCacheTTL   string `json:"roles_cache_ttl,omitempty"`
	MemberCacheTTL  string `json:"member_cache_ttl,omitempty"`
	GuildCacheTTL   string `json:"guild_cache_ttl,omitempty"`
	ChannelCacheTTL string `json:"channel_cache_ttl,omitempty"`
}

type runtimeSettingsSections struct {
	Database      files.DatabaseRuntimeConfig `json:"database"`
	Appearance    runtimeAppearanceSection    `json:"appearance"`
	Logging       runtimeLoggingSection       `json:"logging"`
	PresenceWatch runtimePresenceWatchSection `json:"presence_watch"`
	MessageCache  runtimeMessageCacheSection  `json:"message_cache"`
	Backfill      runtimeBackfillSection      `json:"backfill"`
	Safety        runtimeSafetySection        `json:"safety"`
	Webhook       runtimeWebhookSection       `json:"webhook"`
	Advanced      runtimeAdvancedSection      `json:"advanced"`
}

type runtimeAppearanceSection struct {
	BotTheme string `json:"bot_theme,omitempty"`
}

type runtimeLoggingSection struct {
	DisableDBCleanup     bool  `json:"disable_db_cleanup,omitempty"`
	DisableMessageLogs   bool  `json:"disable_message_logs,omitempty"`
	DisableEntryExitLogs bool  `json:"disable_entry_exit_logs,omitempty"`
	DisableReactionLogs  bool  `json:"disable_reaction_logs,omitempty"`
	DisableUserLogs      bool  `json:"disable_user_logs,omitempty"`
	DisableCleanLog      bool  `json:"disable_clean_log,omitempty"`
	ModerationLogging    *bool `json:"moderation_logging,omitempty"`
}

type runtimePresenceWatchSection struct {
	PresenceWatchUserID string `json:"presence_watch_user_id,omitempty"`
	PresenceWatchBot    bool   `json:"presence_watch_bot,omitempty"`
}

type runtimeMessageCacheSection struct {
	MessageCacheTTLHours int  `json:"message_cache_ttl_hours,omitempty"`
	MessageDeleteOnLog   bool `json:"message_delete_on_log,omitempty"`
	MessageCacheCleanup  bool `json:"message_cache_cleanup,omitempty"`
}

type runtimeBackfillSection struct {
	BackfillChannelID   string `json:"backfill_channel_id,omitempty"`
	BackfillStartDay    string `json:"backfill_start_day,omitempty"`
	BackfillInitialDate string `json:"backfill_initial_date,omitempty"`
}

type runtimeSafetySection struct {
	DisableBotRolePermMirror     bool   `json:"disable_bot_role_perm_mirror,omitempty"`
	BotRolePermMirrorActorRoleID string `json:"bot_role_perm_mirror_actor_role_id,omitempty"`
}

type runtimeWebhookSection struct {
	Updates    []files.WebhookEmbedUpdateConfig   `json:"updates,omitempty"`
	Validation files.WebhookEmbedValidationConfig `json:"validation"`
}

type runtimeAdvancedSection struct {
	GlobalMaxWorkers int `json:"global_max_workers,omitempty"`
}

type updateGlobalSettingsRequest struct {
	ConfigVersion *int64                   `json:"config_version,omitempty"`
	Features      *files.FeatureToggles    `json:"features,omitempty"`
	Runtime       *runtimeSettingsSections `json:"runtime,omitempty"`
}

type updateGuildSettingsRequest struct {
	ConfigVersion       *int64                     `json:"config_version,omitempty"`
	BotInstanceTokens   *map[string]string         `json:"bot_instance_tokens,omitempty"`
	BotInstanceStatuses *map[string]string         `json:"bot_instance_statuses,omitempty"`
	FeatureRouting      *map[string]string         `json:"feature_routing,omitempty"`
	Features            *files.FeatureToggles      `json:"features,omitempty"`
	Channels            *files.ChannelsConfig      `json:"channels,omitempty"`
	Roles               *files.RolesConfig         `json:"roles,omitempty"`
	Stats               *files.StatsConfig         `json:"stats,omitempty"`
	Cache               *guildCacheSettingsSection `json:"cache,omitempty"`
	UserPrune           *files.UserPruneConfig     `json:"user_prune,omitempty"`
	PartnerBoard        *files.PartnerBoardConfig  `json:"partner_board,omitempty"`
	Runtime             *runtimeSettingsSections   `json:"runtime,omitempty"`
}

func buildSettingsCatalog() settingsCatalog {
	return settingsCatalog{
		Global: []settingsCatalogSection{
			{
				ID:          "features",
				Title:       "Global feature toggles",
				Description: "Bot-wide service and capability defaults with inheritance-aware overrides.",
				Scope:       "global",
				Kind:        "object",
			},
			{
				ID:          "runtime",
				Title:       "Global runtime settings",
				Description: "Operational behavior, database connectivity, webhook patching, and runtime safety controls.",
				Scope:       "global",
				Kind:        "grouped_object",
			},
		},
		Guild: []settingsCatalogSection{
			{
				ID:          "bot_instance_tokens_configured",
				Title:       "Bot instances",
				Description: "Configured bot tokens mapped by their instance ID for this guild.",
				Scope:       "guild",
				Kind:        "object",
			},
			{
				ID:                  "features",
				Title:               "Guild feature overrides",
				Description:         "Per-guild tri-state overrides that inherit from global defaults when unset.",
				Scope:               "guild",
				Kind:                "object",
				SupportsInheritance: true,
			},
			{
				ID:          "channels",
				Title:       "Channel routing",
				Description: "Target channels for commands, logs, and moderation flows.",
				Scope:       "guild",
				Kind:        "object",
			},
			{
				ID:          "roles",
				Title:       "Roles and auto-assignment",
				Description: "Allowed admin roles, mute role setup, booster role anchoring, and auto-assignment rules.",
				Scope:       "guild",
				Kind:        "object",
			},
			{
				ID:          "stats",
				Title:       "Stats channels",
				Description: "Periodic member-count channel updates with channel-level templates and filters.",
				Scope:       "guild",
				Kind:        "collection",
			},
			{
				ID:          "cache",
				Title:       "Cache tuning",
				Description: "Per-guild cache TTL overrides for roles, members, guilds, and channels.",
				Scope:       "guild",
				Kind:        "object",
			},
			{
				ID:          "user_prune",
				Title:       "User prune",
				Description: "Automatic prune behavior plus legacy compatibility fields kept in the config file.",
				Scope:       "guild",
				Kind:        "object",
				Advanced:    true,
			},
			{
				ID:          "partner_board",
				Title:       "Partner board",
				Description: "Board target, render template, and partner directory entries.",
				Scope:       "guild",
				Kind:        "collection",
			},
			{
				ID:                  "runtime",
				Title:               "Guild runtime overrides",
				Description:         "Per-guild runtime overrides with effective values derived from global defaults.",
				Scope:               "guild",
				Kind:                "grouped_object",
				SupportsInheritance: true,
			},
		},
	}
}

func buildSettingsOverview(
	cfg files.BotConfig,
	configPath string,
	registry guildRegistryWorkspace,
	allowedGuilds map[string]struct{},
) settingsOverview {
	return settingsOverview{
		ConfigPath: configPath,
		Catalog:    buildSettingsCatalog(),
		Global:     buildGlobalSettingsWorkspace(cfg),
		Registry:   registry,
		Guilds:     buildConfiguredGuildSummaries(cfg, allowedGuilds),
	}
}

func buildGlobalSettingsWorkspace(cfg files.BotConfig) globalSettingsWorkspace {
	return globalSettingsWorkspace{
		Scope:         "global",
		ConfigVersion: cfg.ConfigVersion,
		Sections: globalSettingsSections{
			Features: cfg.Features,
			Runtime:  groupRuntimeSettings(cfg.RuntimeConfig),
		},
		Effective: globalSettingsEffective{
			Features: cfg.ResolveFeatures(""),
			Runtime:  groupRuntimeSettings(cfg.ResolveRuntimeConfig("")),
		},
	}
}

func buildGuildSettingsWorkspaceWithBindings(
	cfg files.BotConfig,
	guild files.GuildConfig,
	availableBotInstanceIDs []string,
) guildSettingsWorkspace {
	return guildSettingsWorkspace{
		Scope:                   "guild",
		GuildID:                 guild.GuildID,
		ConfigVersion:           guild.ConfigVersion,
		AvailableBotInstanceIDs: slices.Clone(availableBotInstanceIDs),
		Sections: guildSettingsSections{
			BotInstanceTokensConfigured: buildBotInstanceTokensSection(guild.BotInstanceTokens),
			BotInstanceStatuses:         guild.BotInstanceStatuses,
			FeatureRouting:              guild.FeatureRouting,
			Features:                    guild.Features,
			Channels:                    guild.Channels,
			Roles:                       guild.Roles,
			Stats:                       guild.Stats,
			Cache: guildCacheSettingsSection{
				RolesCacheTTL:   guild.RolesCacheTTL,
				MemberCacheTTL:  guild.MemberCacheTTL,
				GuildCacheTTL:   guild.GuildCacheTTL,
				ChannelCacheTTL: guild.ChannelCacheTTL,
			},
			UserPrune:    guild.UserPrune,
			PartnerBoard: guild.PartnerBoard,
			Runtime:      groupRuntimeSettings(guild.RuntimeConfig),
		},
		Effective: guildSettingsEffective{
			Features: cfg.ResolveFeatures(guild.GuildID),
			Runtime:  groupRuntimeSettings(cfg.ResolveRuntimeConfig(guild.GuildID)),
		},
	}
}

func buildBotInstanceTokensSection(tokens map[string]files.EncryptedString) map[string]bool {
	if len(tokens) == 0 {
		return nil
	}
	out := make(map[string]bool)
	for k, v := range tokens {
		if len(v) > 0 {
			out[k] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildConfiguredGuildSummaries(
	cfg files.BotConfig,
	allowedGuilds map[string]struct{},
) []configuredGuildSummary {
	out := make([]configuredGuildSummary, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if !guildAllowed(guild.GuildID, allowedGuilds) {
			continue
		}
		out = append(out, buildConfiguredGuildSummary(guild))
	}
	return out
}

func buildGuildRegistryWorkspace(
	cfg files.BotConfig,
	sources []guildRegistrySource,
	allowedGuilds map[string]struct{},
) guildRegistryWorkspace {
	configured := make(map[string]configuredGuildSummary, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if !guildAllowed(guild.GuildID, allowedGuilds) {
			continue
		}
		configured[guild.GuildID] = buildConfiguredGuildSummary(guild)
	}

	entries := make([]guildRegistryEntry, 0, len(sources)+len(configured))
	seen := make(map[string]struct{}, len(sources)+len(configured))
	configuredCount := 0

	for _, source := range sources {
		guildID := strings.TrimSpace(source.GuildID)
		if guildID == "" {
			continue
		}
		if _, exists := seen[guildID]; exists {
			continue
		}

		entry := guildRegistryEntry{
			GuildID:                 guildID,
			Name:                    strings.TrimSpace(source.Name),
			Icon:                    strings.TrimSpace(source.Icon),
			Permissions:             source.Permissions,
			AvailableBotInstanceIDs: slices.Clone(source.AvailableBotInstanceIDs),
		}
		if summary, ok := configured[guildID]; ok {
			entry.Configured = true
			applyConfiguredSummary(&entry, summary)
			configuredCount++
		}

		entries = append(entries, entry)
		seen[guildID] = struct{}{}
	}

	for guildID, summary := range configured {
		if _, exists := seen[guildID]; exists {
			continue
		}

		entry := guildRegistryEntry{
			GuildID:    guildID,
			Configured: true,
		}
		applyConfiguredSummary(&entry, summary)
		entries = append(entries, entry)
		seen[guildID] = struct{}{}
		configuredCount++
	}

	sort.Slice(entries, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(entries[i].Name))
		rightName := strings.ToLower(strings.TrimSpace(entries[j].Name))
		switch {
		case leftName == "" && rightName != "":
			return false
		case leftName != "" && rightName == "":
			return true
		case leftName != rightName:
			return leftName < rightName
		default:
			return entries[i].GuildID < entries[j].GuildID
		}
	})

	return guildRegistryWorkspace{
		Scope:           "guild_registry",
		Entries:         entries,
		ConfiguredCount: configuredCount,
		AvailableCount:  len(entries) - configuredCount,
	}
}

func buildConfiguredGuildSummary(guild files.GuildConfig) configuredGuildSummary {
	return configuredGuildSummary{
		GuildID:             guild.GuildID,
		ConfiguredChannels:  countConfiguredChannels(guild.Channels),
		AllowedRoles:        len(guild.Roles.Allowed),
		StatsChannels:       len(guild.Stats.Channels),
		Partners:            len(guild.PartnerBoard.Partners),
		HasFeatureOverrides: hasFeatureOverrides(guild.Features),
		HasRuntimeOverrides: hasRuntimeOverrides(guild.RuntimeConfig),
	}
}

func applyConfiguredSummary(entry *guildRegistryEntry, summary configuredGuildSummary) {
	if entry == nil {
		return
	}
	entry.ConfiguredChannels = summary.ConfiguredChannels
	entry.AllowedRoles = summary.AllowedRoles
	entry.StatsChannels = summary.StatsChannels
	entry.Partners = summary.Partners
	entry.HasFeatureOverrides = summary.HasFeatureOverrides
	entry.HasRuntimeOverrides = summary.HasRuntimeOverrides
}

func groupRuntimeSettings(rc files.RuntimeConfig) runtimeSettingsSections {
	return runtimeSettingsSections{
		Database:   rc.Database,
		Appearance: runtimeAppearanceSection{BotTheme: rc.BotTheme},
		Logging: runtimeLoggingSection{
			DisableDBCleanup:     rc.DisableDBCleanup,
			DisableMessageLogs:   rc.DisableMessageLogs,
			DisableEntryExitLogs: rc.DisableEntryExitLogs,
			DisableReactionLogs:  rc.DisableReactionLogs,
			DisableUserLogs:      rc.DisableUserLogs,
			DisableCleanLog:      rc.DisableCleanLog,
			ModerationLogging:    rc.ModerationLogging,
		},
		PresenceWatch: runtimePresenceWatchSection{
			PresenceWatchUserID: rc.PresenceWatchUserID,
			PresenceWatchBot:    rc.PresenceWatchBot,
		},
		MessageCache: runtimeMessageCacheSection{
			MessageCacheTTLHours: rc.MessageCacheTTLHours,
			MessageDeleteOnLog:   rc.MessageDeleteOnLog,
			MessageCacheCleanup:  rc.MessageCacheCleanup,
		},
		Backfill: runtimeBackfillSection{
			BackfillChannelID:   rc.BackfillChannelID,
			BackfillStartDay:    rc.BackfillStartDay,
			BackfillInitialDate: rc.BackfillInitialDate,
		},
		Safety: runtimeSafetySection{
			DisableBotRolePermMirror:     rc.DisableBotRolePermMirror,
			BotRolePermMirrorActorRoleID: rc.BotRolePermMirrorActorRoleID,
		},
		Webhook: runtimeWebhookSection{
			Updates:    rc.NormalizedWebhookEmbedUpdates(),
			Validation: rc.WebhookEmbedValidation,
		},
		Advanced: runtimeAdvancedSection{
			GlobalMaxWorkers: rc.GlobalMaxWorkers,
		},
	}
}

func flattenRuntimeSettingsSections(in runtimeSettingsSections) files.RuntimeConfig {
	return files.RuntimeConfig{
		Database:                     in.Database,
		BotTheme:                     in.Appearance.BotTheme,
		DisableDBCleanup:             in.Logging.DisableDBCleanup,
		DisableMessageLogs:           in.Logging.DisableMessageLogs,
		DisableEntryExitLogs:         in.Logging.DisableEntryExitLogs,
		DisableReactionLogs:          in.Logging.DisableReactionLogs,
		DisableUserLogs:              in.Logging.DisableUserLogs,
		DisableCleanLog:              in.Logging.DisableCleanLog,
		ModerationLogging:            in.Logging.ModerationLogging,
		PresenceWatchUserID:          in.PresenceWatch.PresenceWatchUserID,
		PresenceWatchBot:             in.PresenceWatch.PresenceWatchBot,
		MessageCacheTTLHours:         in.MessageCache.MessageCacheTTLHours,
		MessageDeleteOnLog:           in.MessageCache.MessageDeleteOnLog,
		MessageCacheCleanup:          in.MessageCache.MessageCacheCleanup,
		GlobalMaxWorkers:             in.Advanced.GlobalMaxWorkers,
		BackfillChannelID:            in.Backfill.BackfillChannelID,
		BackfillStartDay:             in.Backfill.BackfillStartDay,
		BackfillInitialDate:          in.Backfill.BackfillInitialDate,
		DisableBotRolePermMirror:     in.Safety.DisableBotRolePermMirror,
		BotRolePermMirrorActorRoleID: in.Safety.BotRolePermMirrorActorRoleID,
		WebhookEmbedUpdates:          in.Webhook.Updates,
		WebhookEmbedValidation:       in.Webhook.Validation,
	}
}

func findGuildSettings(cfg files.BotConfig, guildID string) (files.GuildConfig, bool) {
	for _, guild := range cfg.Guilds {
		if guild.GuildID == guildID {
			return guild, true
		}
	}
	return files.GuildConfig{}, false
}

func findGuildSettingsMutable(cfg *files.BotConfig, guildID string) (*files.GuildConfig, bool) {
	if cfg == nil {
		return nil, false
	}
	for idx := range cfg.Guilds {
		if cfg.Guilds[idx].GuildID == guildID {
			return &cfg.Guilds[idx], true
		}
	}
	return nil, false
}

func guildAllowed(guildID string, allowedGuilds map[string]struct{}) bool {
	if allowedGuilds == nil {
		return true
	}
	_, ok := allowedGuilds[guildID]
	return ok
}

func countConfiguredChannels(ch files.ChannelsConfig) int {
	count := 0
	values := []string{
		ch.Commands,
		ch.AvatarLogging,
		ch.RoleUpdate,
		ch.MemberJoin,
		ch.MemberLeave,
		ch.MessageEdit,
		ch.MessageDelete,
		ch.AutomodAction,
		ch.ModerationCase,
		ch.EntryBackfill,
	}
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	return count
}

func hasFeatureOverrides(ft files.FeatureToggles) bool {
	return ft.HasAnyOverride()
}

func hasRuntimeOverrides(rc files.RuntimeConfig) bool {
	if rc.Database != (files.DatabaseRuntimeConfig{}) {
		return true
	}
	return rc.BotTheme != "" ||
		rc.DisableDBCleanup ||
		rc.DisableMessageLogs ||
		rc.DisableEntryExitLogs ||
		rc.DisableReactionLogs ||
		rc.DisableUserLogs ||
		rc.DisableCleanLog ||
		rc.ModerationLogging != nil ||
		rc.PresenceWatchUserID != "" ||
		rc.PresenceWatchBot ||
		rc.MessageCacheTTLHours != 0 ||
		rc.MessageDeleteOnLog ||
		rc.MessageCacheCleanup ||
		rc.GlobalMaxWorkers != 0 ||
		rc.BackfillChannelID != "" ||
		rc.BackfillStartDay != "" ||
		rc.BackfillInitialDate != "" ||
		rc.DisableBotRolePermMirror ||
		rc.BotRolePermMirrorActorRoleID != "" ||
		len(rc.NormalizedWebhookEmbedUpdates()) > 0 ||
		rc.WebhookEmbedValidation != (files.WebhookEmbedValidationConfig{})
}
