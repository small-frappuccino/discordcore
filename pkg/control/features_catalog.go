package control

import (
	"errors"

	discordlogging "github.com/small-frappuccino/discordcore/pkg/discord/logging"
)

var errUnknownFeatureID = errors.New("unknown feature id")

type featurePatchBadRequestError struct {
	message string
}

func (e featurePatchBadRequestError) Error() string {
	return e.message
}

type featureDefinition struct {
	ID                    string
	Category              string
	Label                 string
	Description           string
	Area                  featureAreaID
	Tags                  []string
	SupportsGuildOverride bool
	GlobalEditableFields  []string
	GuildEditableFields   []string
	LogEvent              discordlogging.LogEventType
}

type featureCatalogEntry struct {
	ID                    string        `json:"id"`
	Category              string        `json:"category"`
	Label                 string        `json:"label"`
	Description           string        `json:"description"`
	Area                  featureAreaID `json:"area"`
	Tags                  []string      `json:"tags,omitempty"`
	SupportsGuildOverride bool          `json:"supports_guild_override"`
	GlobalEditableFields  []string      `json:"global_editable_fields,omitempty"`
	GuildEditableFields   []string      `json:"guild_editable_fields,omitempty"`
}

type featureWorkspace struct {
	Scope    string          `json:"scope"`
	GuildID  string          `json:"guild_id,omitempty"`
	Features []featureRecord `json:"features"`
}

type featureRecord struct {
	ID                    string           `json:"id"`
	Category              string           `json:"category"`
	Label                 string           `json:"label"`
	Description           string           `json:"description"`
	Scope                 string           `json:"scope"`
	Area                  featureAreaID    `json:"area"`
	Tags                  []string         `json:"tags,omitempty"`
	SupportsGuildOverride bool             `json:"supports_guild_override"`
	OverrideState         string           `json:"override_state"`
	EffectiveEnabled      bool             `json:"effective_enabled"`
	EffectiveSource       string           `json:"effective_source"`
	Readiness             string           `json:"readiness"`
	Blockers              []featureBlocker `json:"blockers,omitempty"`
	Details               map[string]any   `json:"details,omitempty"`
	EditableFields        []string         `json:"editable_fields,omitempty"`
}

type featureBlocker struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

type featureStatsChannelDetail struct {
	ChannelID    string `json:"channel_id,omitempty"`
	Label        string `json:"label,omitempty"`
	NameTemplate string `json:"name_template,omitempty"`
	MemberType   string `json:"member_type,omitempty"`
	RoleID       string `json:"role_id,omitempty"`
}

var featureDefinitions = []featureDefinition{
	{ID: "services.monitoring", Category: "services", Label: "Monitoring", Description: "Core monitoring service lifecycle and shared event processing.", Area: featureAreaMaintenance, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "services.automod", Category: "services", Label: "Automod service", Description: "Discord native AutoMod event listener used for moderation logging. No local rules engine is active yet.", Area: featureAreaModeration, Tags: []string{featureTagModerationAutomod}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "moderation.mute_role", Category: "moderation", Label: "Mute role", Description: "Role applied by the mute command when the selected server needs role-based muting.", Area: featureAreaModeration, Tags: []string{featureTagModerationMuteRole}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "role_id"}},
	{ID: "moderation.ban", Category: "moderation", Label: "Ban command", Description: "Enable the slash command that bans a single member.", Area: featureAreaModeration, Tags: []string{featureTagModerationCommand}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "moderation.massban", Category: "moderation", Label: "Mass ban command", Description: "Enable the slash command that bans multiple members in one action.", Area: featureAreaModeration, Tags: []string{featureTagModerationCommand}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "moderation.kick", Category: "moderation", Label: "Kick command", Description: "Enable the slash command that removes a member from the server.", Area: featureAreaModeration, Tags: []string{featureTagModerationCommand}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "moderation.timeout", Category: "moderation", Label: "Timeout command", Description: "Enable the slash command that applies a temporary member timeout.", Area: featureAreaModeration, Tags: []string{featureTagModerationCommand}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "moderation.warn", Category: "moderation", Label: "Warn command", Description: "Enable the slash command that records a moderation warning.", Area: featureAreaModeration, Tags: []string{featureTagModerationCommand}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "moderation.warnings", Category: "moderation", Label: "Warnings command", Description: "Enable the slash command that lists recent warnings for a member.", Area: featureAreaModeration, Tags: []string{featureTagModerationCommand}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "services.commands", Category: "services", Label: "Commands", Description: "Slash-command handling plus the optional command channel route used by guild configuration.", Area: featureAreaCommands, Tags: []string{featureTagCommandsPrimary, featureTagHomeCommands}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}},
	{ID: "services.admin_commands", Category: "services", Label: "Admin commands", Description: "Privileged command workflows scoped by the configured allowed roles.", Area: featureAreaCommands, Tags: []string{featureTagCommandsAdmin, featureTagHomeAdminCommands}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "allowed_role_ids"}},
	{ID: "logging.avatar_logging", Category: "logging", Label: "Avatar logging", Description: "Record avatar changes in the configured user log channel.", Area: featureAreaLogging, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventAvatarChange},
	{ID: "logging.role_update", Category: "logging", Label: "Role update logging", Description: "Record member role changes in the configured user log channel.", Area: featureAreaLogging, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventRoleChange},
	{ID: "logging.member_join", Category: "logging", Label: "Member join logging", Description: "Record member join events in the configured entry/exit log channel.", Area: featureAreaLogging, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventMemberJoin},
	{ID: "logging.member_leave", Category: "logging", Label: "Member leave logging", Description: "Record member leave events in the configured entry/exit log channel.", Area: featureAreaLogging, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventMemberLeave},
	{ID: "logging.message_process", Category: "logging", Label: "Message process logging", Description: "Track message processing events without a dedicated routing channel.", Area: featureAreaLogging, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}, LogEvent: discordlogging.LogEventMessageProcess},
	{ID: "logging.message_edit", Category: "logging", Label: "Message edit logging", Description: "Record edited messages in the configured message log channel.", Area: featureAreaLogging, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventMessageEdit},
	{ID: "logging.message_delete", Category: "logging", Label: "Message delete logging", Description: "Record deleted messages in the configured message log channel.", Area: featureAreaLogging, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventMessageDelete},
	{ID: "logging.reaction_metric", Category: "logging", Label: "Reaction metrics", Description: "Track reaction metrics without a dedicated routing channel.", Area: featureAreaLogging, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}, LogEvent: discordlogging.LogEventReactionMetric},
	{ID: "logging.automod_action", Category: "logging", Label: "Automod action logging", Description: "Record AutoMod executions in a validated moderation log channel.", Area: featureAreaModeration, Tags: []string{featureTagModerationRoute}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventAutomodAction},
	{ID: "logging.moderation_case", Category: "logging", Label: "Moderation case logging", Description: "Record moderation cases in an exclusive moderation log channel.", Area: featureAreaModeration, Tags: []string{featureTagModerationRoute}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventModerationCase},
	{ID: "logging.clean_action", Category: "logging", Label: "Clean action logging", Description: "Record clean actions in the configured clean log channel.", Area: featureAreaLogging, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventCleanAction},
	{ID: "message_cache.cleanup_on_startup", Category: "message_cache", Label: "Message cache cleanup", Description: "Allow startup cleanup when the runtime cache-cleanup switch is also enabled.", Area: featureAreaMaintenance, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "message_cache.delete_on_log", Category: "message_cache", Label: "Delete on log", Description: "Delete cached messages after logging when the runtime delete-on-log switch is enabled.", Area: featureAreaMaintenance, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "presence_watch.bot", Category: "presence_watch", Label: "Presence watch (bot)", Description: "Track presence changes for the bot identity when runtime watching is enabled.", Area: featureAreaRoles, Tags: []string{featureTagRolesAdvanced, featureTagRolesPresenceWatchBot}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled", "watch_bot"}, GuildEditableFields: []string{"enabled", "watch_bot"}},
	{ID: "presence_watch.user", Category: "presence_watch", Label: "Presence watch (user)", Description: "Track presence changes for a specific user ID.", Area: featureAreaRoles, Tags: []string{featureTagRolesAdvanced, featureTagRolesPresenceWatchUser}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled", "user_id"}, GuildEditableFields: []string{"enabled", "user_id"}},
	{ID: "maintenance.db_cleanup", Category: "maintenance", Label: "Database cleanup", Description: "Periodic database cleanup maintenance job.", Area: featureAreaMaintenance, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "safety.bot_role_perm_mirror", Category: "safety", Label: "Bot role permission mirror", Description: "Mirror bot role permission changes with an optional actor role guard.", Area: featureAreaRoles, Tags: []string{featureTagRolesAdvanced, featureTagRolesPermissionMirror}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled", "actor_role_id"}, GuildEditableFields: []string{"enabled", "actor_role_id"}},
	{ID: "backfill.enabled", Category: "backfill", Label: "Entry/exit backfill", Description: "Backfill entry and exit metrics when routing and runtime dates are configured.", Area: featureAreaMaintenance, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled", "channel_id", "start_day", "initial_date"}, GuildEditableFields: []string{"enabled", "channel_id", "start_day", "initial_date"}},
	{ID: "stats_channels", Category: "stats", Label: "Stats channels", Description: "Periodic member-count channel updates driven by per-guild channel definitions.", Area: featureAreaStats, Tags: []string{featureTagStatsPrimary, featureTagHomeStats}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "config_enabled", "update_interval_mins"}},
	{ID: "auto_role_assignment", Category: "roles", Label: "Auto role assignment", Description: "Automatic role assignment driven by target and ordered required roles.", Area: featureAreaRoles, Tags: []string{featureTagRolesAutoAssign, featureTagHomeAutoRole}, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "config_enabled", "target_role_id", "required_role_ids"}},
	{ID: "user_prune", Category: "maintenance", Label: "User prune", Description: "Periodic user prune workflow plus its guild-level pruning configuration.", Area: featureAreaMaintenance, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "config_enabled", "grace_days", "scan_interval_mins", "initial_delay_secs", "kicks_per_second", "max_kicks_per_run", "exempt_role_ids", "dry_run"}},
}

var featureDefinitionsByID = func() map[string]featureDefinition {
	out := make(map[string]featureDefinition, len(featureDefinitions))
	for _, def := range featureDefinitions {
		out[def.ID] = def
	}
	return out
}()
