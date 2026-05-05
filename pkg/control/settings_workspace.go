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
	BotInstanceID       string `json:"bot_instance_id,omitempty"`
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
	Owner                   bool     `json:"owner"`
	Permissions             int64    `json:"permissions"`
	Configured              bool     `json:"configured"`
	BotInstanceID           string   `json:"bot_instance_id,omitempty"`
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
	Owner                   bool
	Permissions             int64
	AvailableBotInstanceIDs []string
}

type globalSettingsWorkspace struct {
	Scope     string                  `json:"scope"`
	Sections  globalSettingsSections  `json:"sections"`
	Effective globalSettingsEffective `json:"effective"`
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
	BotInstanceID           string                 `json:"bot_instance_id,omitempty"`
	AvailableBotInstanceIDs []string               `json:"available_bot_instance_ids,omitempty"`
	Sections                guildSettingsSections  `json:"sections"`
	Effective               guildSettingsEffective `json:"effective"`
}

type guildBotRoutingSection struct {
	BotInstanceID           string            `json:"bot_instance_id,omitempty"`
	AvailableBotInstanceIDs []string          `json:"available_bot_instance_ids,omitempty"`
	DomainBotInstanceIDs    map[string]string `json:"domain_bot_instance_ids,omitempty"`
	EditableDomains         []string          `json:"editable_domains,omitempty"`
}

type guildSettingsSections struct {
	BotRouting   guildBotRoutingSection      `json:"bot_routing"`
	Features     files.FeatureToggles      `json:"features"`
	Channels     files.ChannelsConfig      `json:"channels"`
	Roles        files.RolesConfig         `json:"roles"`
	Stats        files.StatsConfig         `json:"stats"`
	Cache        guildCacheSettingsSection `json:"cache"`
	UserPrune    files.UserPruneConfig     `json:"user_prune"`
	PartnerBoard files.PartnerBoardConfig  `json:"partner_board"`
	Runtime      runtimeSettingsSections   `json:"runtime"`
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
	DisableAutomodLogs   bool  `json:"disable_automod_logs,omitempty"`
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
	GlobalMaxWorkers        int                            `json:"global_max_workers,omitempty"`
	ModerationLogMode       string                         `json:"moderation_log_mode,omitempty"`
	LegacyWebhookEmbedPatch files.WebhookEmbedUpdateConfig `json:"legacy_webhook_embed_update,omitempty"`
}

type updateGlobalSettingsRequest struct {
	Features *files.FeatureToggles    `json:"features,omitempty"`
	Runtime  *runtimeSettingsSections `json:"runtime,omitempty"`
}

type updateGuildSettingsRequest struct {
	BotInstanceID *string                    `json:"bot_instance_id,omitempty"`
	BotRouting    *guildBotRoutingSection    `json:"bot_routing,omitempty"`
	Features      *files.FeatureToggles      `json:"features,omitempty"`
	Channels      *files.ChannelsConfig      `json:"channels,omitempty"`
	Roles         *files.RolesConfig         `json:"roles,omitempty"`
	Stats         *files.StatsConfig         `json:"stats,omitempty"`
	Cache         *guildCacheSettingsSection `json:"cache,omitempty"`
	UserPrune     *files.UserPruneConfig     `json:"user_prune,omitempty"`
	PartnerBoard  *files.PartnerBoardConfig  `json:"partner_board,omitempty"`
	Runtime       *runtimeSettingsSections   `json:"runtime,omitempty"`
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
				ID:          "bot_routing",
				Title:       "Bot routing",
				Description: "Default bot ownership for this server plus specialized domain overrides such as QOTD.",
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
				Description: "Allowed admin roles, mute role setup, verification roles, booster role anchoring, and auto-assignment rules.",
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
	defaultBotInstanceID string,
) settingsOverview {
	return settingsOverview{
		ConfigPath: configPath,
		Catalog:    buildSettingsCatalog(),
		Global:     buildGlobalSettingsWorkspace(cfg),
		Registry:   registry,
		Guilds:     buildConfiguredGuildSummaries(cfg, allowedGuilds, defaultBotInstanceID),
	}
}

func buildGlobalSettingsWorkspace(cfg files.BotConfig) globalSettingsWorkspace {
	return globalSettingsWorkspace{
		Scope: "global",
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
	defaultBotInstanceID string,
) guildSettingsWorkspace {
	return guildSettingsWorkspace{
		Scope:                   "guild",
		GuildID:                 guild.GuildID,
		BotInstanceID:           guild.EffectiveBotInstanceID(defaultBotInstanceID),
		AvailableBotInstanceIDs: slices.Clone(availableBotInstanceIDs),
		Sections: guildSettingsSections{
			BotRouting: buildGuildBotRoutingSection(guild, availableBotInstanceIDs, defaultBotInstanceID),
			Features: guild.Features,
			Channels: guild.Channels,
			Roles:    guild.Roles,
			Stats:    guild.Stats,
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

func buildGuildBotRoutingSection(
	guild files.GuildConfig,
	availableBotInstanceIDs []string,
	defaultBotInstanceID string,
) guildBotRoutingSection {
	return guildBotRoutingSection{
		BotInstanceID:           guild.EffectiveBotInstanceID(defaultBotInstanceID),
		AvailableBotInstanceIDs: slices.Clone(availableBotInstanceIDs),
		DomainBotInstanceIDs:    cloneEditableDomainBotInstanceIDs(guild.DomainBotInstanceIDs),
		EditableDomains:         settingsEditableBotRoutingDomains(),
	}
}

func settingsEditableBotRoutingDomains() []string {
	return []string{files.BotDomainQOTD}
}

func cloneEditableDomainBotInstanceIDs(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}

	editableDomains := make(map[string]struct{}, len(settingsEditableBotRoutingDomains()))
	for _, domain := range settingsEditableBotRoutingDomains() {
		editableDomains[files.NormalizeBotDomain(domain)] = struct{}{}
	}

	cloned := make(map[string]string, len(input))
	for domain, botInstanceID := range input {
		normalizedDomain := files.NormalizeBotDomain(domain)
		if _, ok := editableDomains[normalizedDomain]; !ok {
			continue
		}
		normalizedBotInstanceID := files.NormalizeBotInstanceID(botInstanceID)
		if normalizedBotInstanceID == "" {
			continue
		}
		cloned[normalizedDomain] = normalizedBotInstanceID
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func buildConfiguredGuildSummaries(
	cfg files.BotConfig,
	allowedGuilds map[string]struct{},
	defaultBotInstanceID string,
) []configuredGuildSummary {
	out := make([]configuredGuildSummary, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if !guildAllowed(guild.GuildID, allowedGuilds) {
			continue
		}
		out = append(out, buildConfiguredGuildSummary(guild, defaultBotInstanceID))
	}
	return out
}

func buildGuildRegistryWorkspace(
	cfg files.BotConfig,
	sources []guildRegistrySource,
	allowedGuilds map[string]struct{},
	defaultBotInstanceID string,
) guildRegistryWorkspace {
	configured := make(map[string]configuredGuildSummary, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if !guildAllowed(guild.GuildID, allowedGuilds) {
			continue
		}
		configured[guild.GuildID] = buildConfiguredGuildSummary(guild, defaultBotInstanceID)
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
			Owner:                   source.Owner,
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

func buildConfiguredGuildSummary(guild files.GuildConfig, defaultBotInstanceID string) configuredGuildSummary {
	return configuredGuildSummary{
		GuildID:             guild.GuildID,
		BotInstanceID:       guild.EffectiveBotInstanceID(defaultBotInstanceID),
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
	entry.BotInstanceID = summary.BotInstanceID
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
			DisableAutomodLogs:   rc.DisableAutomodLogs,
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
			GlobalMaxWorkers:        rc.GlobalMaxWorkers,
			ModerationLogMode:       rc.ModerationLogMode,
			LegacyWebhookEmbedPatch: rc.WebhookEmbedUpdate,
		},
	}
}

func flattenRuntimeSettingsSections(in runtimeSettingsSections) files.RuntimeConfig {
	return files.RuntimeConfig{
		Database:                     in.Database,
		BotTheme:                     in.Appearance.BotTheme,
		DisableDBCleanup:             in.Logging.DisableDBCleanup,
		DisableAutomodLogs:           in.Logging.DisableAutomodLogs,
		DisableMessageLogs:           in.Logging.DisableMessageLogs,
		DisableEntryExitLogs:         in.Logging.DisableEntryExitLogs,
		DisableReactionLogs:          in.Logging.DisableReactionLogs,
		DisableUserLogs:              in.Logging.DisableUserLogs,
		DisableCleanLog:              in.Logging.DisableCleanLog,
		ModerationLogging:            in.Logging.ModerationLogging,
		ModerationLogMode:            in.Advanced.ModerationLogMode,
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
		WebhookEmbedUpdate:           in.Advanced.LegacyWebhookEmbedPatch,
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
		ch.VerificationCleanup,
	}
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	return count
}

func hasFeatureOverrides(ft files.FeatureToggles) bool {
	ptrs := []*bool{
		ft.Services.Monitoring,
		ft.Services.Automod,
		ft.Services.Commands,
		ft.Services.AdminCommands,
		ft.Logging.AvatarLogging,
		ft.Logging.RoleUpdate,
		ft.Logging.MemberJoin,
		ft.Logging.MemberLeave,
		ft.Logging.MessageProcess,
		ft.Logging.MessageEdit,
		ft.Logging.MessageDelete,
		ft.Logging.ReactionMetric,
		ft.Logging.AutomodAction,
		ft.Logging.ModerationCase,
		ft.Logging.CleanAction,
		ft.MessageCache.CleanupOnStartup,
		ft.MessageCache.DeleteOnLog,
		ft.PresenceWatch.Bot,
		ft.PresenceWatch.User,
		ft.Maintenance.DBCleanup,
		ft.Safety.BotRolePermMirror,
		ft.Backfill.Enabled,
		ft.MuteRole,
		ft.StatsChannels,
		ft.AutoRoleAssign,
		ft.UserPrune,
	}
	for _, item := range ptrs {
		if item != nil {
			return true
		}
	}
	return false
}

func hasRuntimeOverrides(rc files.RuntimeConfig) bool {
	if rc.Database != (files.DatabaseRuntimeConfig{}) {
		return true
	}
	if rc.BotTheme != "" ||
		rc.DisableDBCleanup ||
		rc.DisableAutomodLogs ||
		rc.DisableMessageLogs ||
		rc.DisableEntryExitLogs ||
		rc.DisableReactionLogs ||
		rc.DisableUserLogs ||
		rc.DisableCleanLog ||
		rc.ModerationLogging != nil ||
		rc.ModerationLogMode != "" ||
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
		rc.WebhookEmbedValidation != (files.WebhookEmbedValidationConfig{}) {
		return true
	}
	return !rc.WebhookEmbedUpdate.IsZero()
}
