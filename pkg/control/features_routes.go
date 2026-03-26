package control

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	discordlogging "github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/files"
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
	SupportsGuildOverride bool
	GlobalEditableFields  []string
	GuildEditableFields   []string
	LogEvent              discordlogging.LogEventType
}

type featureCatalogEntry struct {
	ID                    string   `json:"id"`
	Category              string   `json:"category"`
	Label                 string   `json:"label"`
	Description           string   `json:"description"`
	SupportsGuildOverride bool     `json:"supports_guild_override"`
	GlobalEditableFields  []string `json:"global_editable_fields,omitempty"`
	GuildEditableFields   []string `json:"guild_editable_fields,omitempty"`
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
	{ID: "services.monitoring", Category: "services", Label: "Monitoring", Description: "Core monitoring service lifecycle and shared event processing.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "services.automod", Category: "services", Label: "Automod service", Description: "Discord native AutoMod event listener used for moderation logging. No local rules engine is active yet.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "moderation.mute_role", Category: "moderation", Label: "Mute role", Description: "Role applied by the mute command when the selected server needs role-based muting.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "role_id"}},
	{ID: "services.commands", Category: "services", Label: "Commands", Description: "Slash-command handling plus the optional command channel route used by guild configuration.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}},
	{ID: "services.admin_commands", Category: "services", Label: "Admin commands", Description: "Privileged command workflows scoped by the configured allowed roles.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "allowed_role_ids"}},
	{ID: "logging.avatar_logging", Category: "logging", Label: "Avatar logging", Description: "Record avatar changes in the configured user log channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventAvatarChange},
	{ID: "logging.role_update", Category: "logging", Label: "Role update logging", Description: "Record member role changes in the configured user log channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventRoleChange},
	{ID: "logging.member_join", Category: "logging", Label: "Member join logging", Description: "Record member join events in the configured entry/exit log channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventMemberJoin},
	{ID: "logging.member_leave", Category: "logging", Label: "Member leave logging", Description: "Record member leave events in the configured entry/exit log channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventMemberLeave},
	{ID: "logging.message_process", Category: "logging", Label: "Message process logging", Description: "Track message processing events without a dedicated routing channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}, LogEvent: discordlogging.LogEventMessageProcess},
	{ID: "logging.message_edit", Category: "logging", Label: "Message edit logging", Description: "Record edited messages in the configured message log channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventMessageEdit},
	{ID: "logging.message_delete", Category: "logging", Label: "Message delete logging", Description: "Record deleted messages in the configured message log channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventMessageDelete},
	{ID: "logging.reaction_metric", Category: "logging", Label: "Reaction metrics", Description: "Track reaction metrics without a dedicated routing channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}, LogEvent: discordlogging.LogEventReactionMetric},
	{ID: "logging.automod_action", Category: "logging", Label: "Automod action logging", Description: "Record AutoMod executions in a validated moderation log channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventAutomodAction},
	{ID: "logging.moderation_case", Category: "logging", Label: "Moderation case logging", Description: "Record moderation cases in an exclusive moderation log channel.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "channel_id"}, LogEvent: discordlogging.LogEventModerationCase},
	{ID: "message_cache.cleanup_on_startup", Category: "message_cache", Label: "Message cache cleanup", Description: "Allow startup cleanup when the runtime cache-cleanup switch is also enabled.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "message_cache.delete_on_log", Category: "message_cache", Label: "Delete on log", Description: "Delete cached messages after logging when the runtime delete-on-log switch is enabled.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "presence_watch.bot", Category: "presence_watch", Label: "Presence watch (bot)", Description: "Track presence changes for the bot identity when runtime watching is enabled.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled", "watch_bot"}, GuildEditableFields: []string{"enabled", "watch_bot"}},
	{ID: "presence_watch.user", Category: "presence_watch", Label: "Presence watch (user)", Description: "Track presence changes for a specific user ID.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled", "user_id"}, GuildEditableFields: []string{"enabled", "user_id"}},
	{ID: "maintenance.db_cleanup", Category: "maintenance", Label: "Database cleanup", Description: "Periodic database cleanup maintenance job.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "safety.bot_role_perm_mirror", Category: "safety", Label: "Bot role permission mirror", Description: "Mirror bot role permission changes with an optional actor role guard.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled", "actor_role_id"}, GuildEditableFields: []string{"enabled", "actor_role_id"}},
	{ID: "backfill.enabled", Category: "backfill", Label: "Entry/exit backfill", Description: "Backfill entry and exit metrics when routing and runtime dates are configured.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled", "channel_id", "start_day", "initial_date"}, GuildEditableFields: []string{"enabled", "channel_id", "start_day", "initial_date"}},
	{ID: "stats_channels", Category: "stats", Label: "Stats channels", Description: "Periodic member-count channel updates driven by per-guild channel definitions.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "config_enabled", "update_interval_mins"}},
	{ID: "auto_role_assignment", Category: "roles", Label: "Auto role assignment", Description: "Automatic role assignment driven by target and ordered required roles.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "config_enabled", "target_role_id", "required_role_ids"}},
	{ID: "user_prune", Category: "maintenance", Label: "User prune", Description: "Periodic user prune workflow plus its guild-level pruning configuration.", SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "config_enabled", "grace_days", "scan_interval_mins", "initial_delay_secs", "kicks_per_second", "max_kicks_per_run", "exempt_role_ids", "dry_run"}},
}

var featureDefinitionsByID = func() map[string]featureDefinition {
	out := make(map[string]featureDefinition, len(featureDefinitions))
	for _, def := range featureDefinitions {
		out[def.ID] = def
	}
	return out
}()

func (s *Server) handleFeatureRoutes(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorizeRequest(w, r); !ok {
		return
	}
	if s.configManager == nil {
		http.Error(w, "config manager unavailable", http.StatusInternalServerError)
		return
	}

	path := normalizeFeatureRoutePath(r.URL.Path)
	switch {
	case path == "/v1/features/catalog":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleFeatureCatalogGet(w)
	case path == "/v1/features":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleGlobalFeaturesList(w)
	default:
		featureID, ok := splitGlobalFeatureRoute(path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			s.handleGlobalFeatureGet(w, featureID)
		case http.MethodPatch:
			s.handleGlobalFeaturePatch(w, r, featureID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (s *Server) handleFeatureCatalogGet(w http.ResponseWriter) {
	items := make([]featureCatalogEntry, 0, len(featureDefinitions))
	for _, def := range featureDefinitions {
		items = append(items, featureCatalogEntry{
			ID:                    def.ID,
			Category:              def.Category,
			Label:                 def.Label,
			Description:           def.Description,
			SupportsGuildOverride: def.SupportsGuildOverride,
			GlobalEditableFields:  slices.Clone(def.GlobalEditableFields),
			GuildEditableFields:   slices.Clone(def.GuildEditableFields),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"catalog": items,
	})
}

func (s *Server) handleGlobalFeaturesList(w http.ResponseWriter) {
	cfg := s.configManager.SnapshotConfig()
	session, err := s.currentDiscordSession()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve global feature session: %v", err), http.StatusServiceUnavailable)
		return
	}
	workspace, err := buildFeatureWorkspace(cfg, s.configManager, "", session)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build global feature workspace: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": workspace,
	})
}

func (s *Server) handleGlobalFeatureGet(w http.ResponseWriter, featureID string) {
	session, err := s.currentDiscordSession()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve global feature session: %v", err), http.StatusServiceUnavailable)
		return
	}
	record, err := buildSingleFeatureRecord(s.configManager.SnapshotConfig(), s.configManager, "", featureID, session)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnknownFeatureID) {
			status = http.StatusNotFound
		}
		http.Error(w, fmt.Sprintf("failed to read feature: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"feature": record,
	})
}

func (s *Server) handleGlobalFeaturePatch(w http.ResponseWriter, r *http.Request, featureID string) {
	updated, err := s.applyFeaturePatch(r, "", featureID)
	if err != nil {
		status := statusForFeatureMutationError(err)
		http.Error(w, fmt.Sprintf("failed to update feature: %v", err), status)
		return
	}

	session, err := s.currentDiscordSession()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve global feature session: %v", err), http.StatusServiceUnavailable)
		return
	}
	record, err := buildSingleFeatureRecord(updated, s.configManager, "", featureID, session)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build updated feature: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"feature": record,
	})
}

func (s *Server) handleGuildFeaturesList(w http.ResponseWriter, guildID string) {
	cfg := s.configManager.SnapshotConfig()
	if _, ok := findGuildSettings(cfg, guildID); !ok {
		http.Error(w, fmt.Sprintf("guild settings not found for %s", guildID), http.StatusNotFound)
		return
	}

	session, err := s.discordSessionForGuild(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve guild feature session: %v", err), http.StatusServiceUnavailable)
		return
	}
	workspace, err := buildFeatureWorkspace(cfg, s.configManager, guildID, session)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build guild feature workspace: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": workspace,
	})
}

func (s *Server) handleGuildFeatureGet(w http.ResponseWriter, guildID, featureID string) {
	cfg := s.configManager.SnapshotConfig()
	if _, ok := findGuildSettings(cfg, guildID); !ok {
		http.Error(w, fmt.Sprintf("guild settings not found for %s", guildID), http.StatusNotFound)
		return
	}

	session, err := s.discordSessionForGuild(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve guild feature session: %v", err), http.StatusServiceUnavailable)
		return
	}
	record, err := buildSingleFeatureRecord(cfg, s.configManager, guildID, featureID, session)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnknownFeatureID) {
			status = http.StatusNotFound
		}
		http.Error(w, fmt.Sprintf("failed to read guild feature: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"feature":  record,
	})
}

func (s *Server) handleGuildFeaturePatch(w http.ResponseWriter, r *http.Request, guildID, featureID string) {
	updated, err := s.applyFeaturePatch(r, guildID, featureID)
	if err != nil {
		status := statusForFeatureMutationError(err)
		http.Error(w, fmt.Sprintf("failed to update guild feature: %v", err), status)
		return
	}

	session, err := s.discordSessionForGuild(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve guild feature session: %v", err), http.StatusServiceUnavailable)
		return
	}
	record, err := buildSingleFeatureRecord(updated, s.configManager, guildID, featureID, session)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build updated guild feature: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"feature":  record,
	})
}

func (s *Server) applyFeaturePatch(r *http.Request, guildID, featureID string) (files.BotConfig, error) {
	def, ok := featureDefinitionsByID[featureID]
	if !ok {
		return files.BotConfig{}, fmt.Errorf("%w: %s", errUnknownFeatureID, featureID)
	}

	payload, err := decodeFeaturePatchPayload(r)
	if err != nil {
		return files.BotConfig{}, err
	}
	if len(payload) == 0 {
		return files.BotConfig{}, featurePatchBadRequestError{message: "payload must contain at least one field"}
	}

	updated, err := s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		if guildID == "" {
			return applyGlobalFeaturePatch(cfg, def, payload)
		}
		guild, ok := findGuildSettingsMutable(cfg, guildID)
		if !ok {
			return fmt.Errorf("%w: register this guild first (guild_id=%s)", errGuildRegistrationRequired, guildID)
		}
		return applyGuildFeaturePatch(cfg, guild, def, payload)
	})
	if err != nil {
		return files.BotConfig{}, err
	}
	return updated, nil
}

func decodeFeaturePatchPayload(r *http.Request) (map[string]json.RawMessage, error) {
	if r == nil || r.Body == nil {
		return nil, featurePatchBadRequestError{message: "request body is required"}
	}

	defer r.Body.Close()

	body, err := io.ReadAll(io.LimitReader(r.Body, defaultMaxBodyBytes+1))
	if err != nil {
		return nil, featurePatchBadRequestError{message: fmt.Sprintf("invalid payload: %v", err)}
	}
	if len(body) > defaultMaxBodyBytes {
		return nil, featurePatchBadRequestError{message: fmt.Sprintf("payload exceeds %d bytes", defaultMaxBodyBytes)}
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, featurePatchBadRequestError{message: fmt.Sprintf("invalid payload: %v", err)}
	}
	return payload, nil
}

func statusForFeatureMutationError(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	if errors.Is(err, errUnknownFeatureID) {
		return http.StatusNotFound
	}

	var badRequest featurePatchBadRequestError
	if errors.As(err, &badRequest) {
		return http.StatusBadRequest
	}

	return statusForSettingsMutationError(err)
}

func buildFeatureWorkspace(
	cfg files.BotConfig,
	configManager *files.ConfigManager,
	guildID string,
	session *discordgo.Session,
) (featureWorkspace, error) {
	records := make([]featureRecord, 0, len(featureDefinitions))
	for _, def := range featureDefinitions {
		record, err := buildFeatureRecord(cfg, configManager, guildID, def, session)
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

func buildSingleFeatureRecord(
	cfg files.BotConfig,
	configManager *files.ConfigManager,
	guildID string,
	featureID string,
	session *discordgo.Session,
) (featureRecord, error) {
	def, ok := featureDefinitionsByID[featureID]
	if !ok {
		return featureRecord{}, fmt.Errorf("%w: %s", errUnknownFeatureID, featureID)
	}
	return buildFeatureRecord(cfg, configManager, guildID, def, session)
}

func buildFeatureRecord(
	cfg files.BotConfig,
	configManager *files.ConfigManager,
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
	readiness, blockers := buildFeatureReadiness(&cfg, configManager, guildID, def, effectiveEnabled, session)
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
		SupportsGuildOverride: def.SupportsGuildOverride,
		OverrideState:         featureOverrideState(&cfg, guildID, def.ID),
		EffectiveEnabled:      effectiveEnabled,
		EffectiveSource:       featureEffectiveSource(&cfg, guildID, def.ID),
		Readiness:             readiness,
		Blockers:              blockers,
		Details:               buildFeatureDetails(&cfg, guildID, def),
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

func buildFeatureDetails(cfg *files.BotConfig, guildID string, def featureDefinition) map[string]any {
	out := map[string]any{}

	if def.LogEvent != "" {
		capability, ok := discordlogging.LogEventCapabilities()[def.LogEvent]
		if ok {
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
					if channelID := logFeatureChannelID(&guild, def.LogEvent); channelID != "" {
						out["channel_id"] = channelID
					}
				}
			}
		}
		return out
	}

	rc := cfg.ResolveRuntimeConfig(guildID)
	switch def.ID {
	case "services.automod":
		out["mode"] = "logging_only"
	case "moderation.mute_role":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				out["role_id"] = strings.TrimSpace(guild.Roles.MuteRole)
			}
		}
	case "services.commands":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok && strings.TrimSpace(guild.Channels.Commands) != "" {
				out["channel_id"] = strings.TrimSpace(guild.Channels.Commands)
			}
		}
	case "services.admin_commands":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				out["allowed_role_ids"] = slices.Clone(guild.Roles.Allowed)
				out["allowed_role_count"] = len(guild.Roles.Allowed)
			}
		}
	case "message_cache.cleanup_on_startup":
		out["runtime_enabled"] = rc.MessageCacheCleanup
	case "message_cache.delete_on_log":
		out["runtime_enabled"] = rc.MessageDeleteOnLog
	case "presence_watch.bot":
		out["watch_bot"] = rc.PresenceWatchBot
	case "presence_watch.user":
		out["user_id"] = strings.TrimSpace(rc.PresenceWatchUserID)
	case "safety.bot_role_perm_mirror":
		out["actor_role_id"] = strings.TrimSpace(rc.BotRolePermMirrorActorRoleID)
		out["runtime_disabled"] = rc.DisableBotRolePermMirror
	case "backfill.enabled":
		if guildID == "" {
			out["channel_id"] = strings.TrimSpace(cfg.RuntimeConfig.BackfillChannelID)
		} else if guild, ok := findGuildSettings(*cfg, guildID); ok {
			out["channel_id"] = strings.TrimSpace(guild.Channels.BackfillChannelID())
		}
		out["start_day"] = strings.TrimSpace(rc.BackfillStartDay)
		out["initial_date"] = strings.TrimSpace(rc.BackfillInitialDate)
	case "stats_channels":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				out["config_enabled"] = guild.Stats.Enabled
				out["update_interval_mins"] = guild.Stats.UpdateIntervalMins
				out["configured_channel_count"] = len(guild.Stats.Channels)
				out["channels"] = buildStatsChannelDetails(guild.Stats.Channels)
			}
		}
	case "auto_role_assignment":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				out["config_enabled"] = guild.Roles.AutoAssignment.Enabled
				out["target_role_id"] = strings.TrimSpace(guild.Roles.AutoAssignment.TargetRoleID)
				out["required_role_ids"] = slices.Clone(guild.Roles.AutoAssignment.RequiredRoles)
				out["required_role_count"] = len(guild.Roles.AutoAssignment.RequiredRoles)
				if len(guild.Roles.AutoAssignment.RequiredRoles) > 0 {
					out["level_role_id"] = strings.TrimSpace(guild.Roles.AutoAssignment.RequiredRoles[0])
				}
				out["booster_role_id"] = strings.TrimSpace(guild.Roles.BoosterRole)
			}
		}
	case "user_prune":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				prune := guild.UserPrune
				out["config_enabled"] = prune.Enabled
				out["grace_days"] = prune.GraceDays
				out["scan_interval_mins"] = prune.ScanIntervalMins
				out["initial_delay_secs"] = prune.InitialDelaySecs
				out["kicks_per_second"] = prune.KicksPerSecond
				out["max_kicks_per_run"] = prune.MaxKicksPerRun
				out["exempt_role_ids"] = slices.Clone(prune.ExemptRoleIDs)
				out["exempt_role_count"] = len(prune.ExemptRoleIDs)
				out["dry_run"] = prune.DryRun
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

func buildFeatureReadiness(
	cfg *files.BotConfig,
	configManager *files.ConfigManager,
	guildID string,
	def featureDefinition,
	effectiveEnabled bool,
	session *discordgo.Session,
) (string, []featureBlocker) {
	if !effectiveEnabled {
		return "disabled", nil
	}
	if def.LogEvent != "" {
		return buildLogFeatureReadiness(cfg, configManager, guildID, def.LogEvent, session)
	}

	rc := cfg.ResolveRuntimeConfig(guildID)
	switch def.ID {
	case "moderation.mute_role":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				roleID := strings.TrimSpace(guild.Roles.MuteRole)
				if roleID == "" {
					return "blocked", []featureBlocker{{Code: "missing_role", Message: "Choose the role that should be applied by the mute command.", Field: "role_id"}}
				}
				if roleIndex, err := guildRoleOptionIndex(session, guildID); err == nil {
					if _, ok := roleIndex[roleID]; !ok {
						return "blocked", []featureBlocker{{Code: "invalid_role", Message: "The configured mute role is no longer available in this server.", Field: "role_id"}}
					}
				}
			}
		}
	case "message_cache.cleanup_on_startup":
		if !rc.MessageCacheCleanup {
			return "blocked", []featureBlocker{{Code: "runtime_disabled", Message: "Runtime message cache cleanup is disabled."}}
		}
	case "message_cache.delete_on_log":
		if !rc.MessageDeleteOnLog {
			return "blocked", []featureBlocker{{Code: "runtime_disabled", Message: "Runtime delete-on-log is disabled."}}
		}
	case "presence_watch.bot":
		if !rc.PresenceWatchBot {
			return "blocked", []featureBlocker{{Code: "runtime_disabled", Message: "Runtime bot presence watching is disabled.", Field: "watch_bot"}}
		}
	case "presence_watch.user":
		if strings.TrimSpace(rc.PresenceWatchUserID) == "" {
			return "blocked", []featureBlocker{{Code: "missing_user_id", Message: "Presence watch needs a user ID.", Field: "user_id"}}
		}
	case "safety.bot_role_perm_mirror":
		if rc.DisableBotRolePermMirror {
			return "blocked", []featureBlocker{{Code: "runtime_kill_switch", Message: "Runtime permission mirroring is disabled."}}
		}
		if guildID != "" {
			actorRoleID := strings.TrimSpace(rc.BotRolePermMirrorActorRoleID)
			if actorRoleID != "" {
				if roleIndex, err := guildRoleOptionIndex(session, guildID); err == nil {
					if _, ok := roleIndex[actorRoleID]; !ok {
						return "blocked", []featureBlocker{{Code: "invalid_actor_role", Message: "Permission mirror actor role is no longer available in this server.", Field: "actor_role_id"}}
					}
				}
			}
		}
	case "backfill.enabled":
		channelID := strings.TrimSpace(rc.BackfillChannelID)
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				channelID = strings.TrimSpace(guild.Channels.BackfillChannelID())
			}
		}
		if channelID == "" {
			return "blocked", []featureBlocker{{Code: "missing_channel", Message: "Backfill needs a configured source channel.", Field: "channel_id"}}
		}
		if strings.TrimSpace(rc.BackfillStartDay) == "" && strings.TrimSpace(rc.BackfillInitialDate) == "" {
			return "blocked", []featureBlocker{{Code: "missing_schedule_seed", Message: "Backfill needs start_day or initial_date configured.", Field: "start_day"}}
		}
	case "stats_channels":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				if !guild.Stats.Enabled {
					return "blocked", []featureBlocker{{Code: "config_disabled", Message: "Stats channel config is disabled.", Field: "config_enabled"}}
				}
				if len(guild.Stats.Channels) == 0 {
					return "blocked", []featureBlocker{{Code: "missing_channels", Message: "Stats channels need at least one configured target."}}
				}
			}
		}
	case "auto_role_assignment":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				auto := guild.Roles.AutoAssignment
				if !auto.Enabled {
					return "blocked", []featureBlocker{{Code: "config_disabled", Message: "Auto assignment config is disabled.", Field: "config_enabled"}}
				}
				if strings.TrimSpace(auto.TargetRoleID) == "" {
					return "blocked", []featureBlocker{{Code: "missing_target_role", Message: "Auto assignment needs a target role.", Field: "target_role_id"}}
				}
				if len(auto.RequiredRoles) != 2 {
					return "blocked", []featureBlocker{{Code: "invalid_required_roles", Message: "Auto assignment needs exactly two required roles in order.", Field: "required_role_ids"}}
				}
				if roleIndex, err := guildRoleOptionIndex(session, guildID); err == nil {
					if _, ok := roleIndex[strings.TrimSpace(auto.TargetRoleID)]; !ok {
						return "blocked", []featureBlocker{{Code: "invalid_target_role", Message: "Auto assignment target role is no longer available in this server.", Field: "target_role_id"}}
					}
					for _, roleID := range auto.RequiredRoles {
						if _, ok := roleIndex[strings.TrimSpace(roleID)]; !ok {
							return "blocked", []featureBlocker{{Code: "invalid_required_roles", Message: "Auto assignment required roles are no longer available in this server.", Field: "required_role_ids"}}
						}
					}
				}
			}
		}
	case "user_prune":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok && !guild.UserPrune.Enabled {
				return "blocked", []featureBlocker{{Code: "config_disabled", Message: "User prune config is disabled.", Field: "config_enabled"}}
			}
		}
	}
	return "ready", nil
}

func buildLogFeatureReadiness(
	cfg *files.BotConfig,
	configManager *files.ConfigManager,
	guildID string,
	eventType discordlogging.LogEventType,
	session *discordgo.Session,
) (string, []featureBlocker) {
	if guildID == "" {
		return buildGlobalLogFeatureReadiness(cfg, eventType, session)
	}
	if configManager == nil {
		return "blocked", []featureBlocker{{Code: "config_unavailable", Message: "Config manager is unavailable."}}
	}

	decision := discordlogging.ShouldEmitLogEvent(session, configManager, eventType, guildID)
	if decision.Enabled {
		return "ready", nil
	}
	return logDecisionToReadiness(decision)
}

func buildGlobalLogFeatureReadiness(
	cfg *files.BotConfig,
	eventType discordlogging.LogEventType,
	session *discordgo.Session,
) (string, []featureBlocker) {
	if blocker, ok := globalLogRuntimeBlocker(cfg.ResolveRuntimeConfig(""), eventType); ok {
		return "blocked", []featureBlocker{blocker}
	}

	capability, ok := discordlogging.LogEventCapabilities()[eventType]
	if ok && capability.RequiredIntentsMask != 0 && session != nil {
		currentMask := int(session.Identify.Intents)
		missing := capability.RequiredIntentsMask &^ currentMask
		if missing != 0 {
			return "blocked", []featureBlocker{{
				Code:    "missing_intent",
				Message: fmt.Sprintf("Gateway intents mask %d is missing required bits %d.", currentMask, missing),
			}}
		}
	}
	return "ready", nil
}

func logDecisionToReadiness(decision discordlogging.EmitDecision) (string, []featureBlocker) {
	switch decision.Reason {
	case discordlogging.EmitReasonRuntimeDisableUserLogs,
		discordlogging.EmitReasonRuntimeDisableEntryExitLogs,
		discordlogging.EmitReasonRuntimeDisableMessageLogs,
		discordlogging.EmitReasonRuntimeDisableReactionLogs,
		discordlogging.EmitReasonRuntimeDisableAutomodLogs,
		discordlogging.EmitReasonRuntimeModerationLoggingOff,
		discordlogging.EmitReasonRuntimeDisableCleanLog:
		return "blocked", []featureBlocker{{Code: "runtime_kill_switch", Message: "A runtime kill switch currently disables this feature."}}
	case discordlogging.EmitReasonNoChannelConfigured:
		return "blocked", []featureBlocker{{Code: "missing_channel", Message: "A channel must be configured for this feature.", Field: "channel_id"}}
	case discordlogging.EmitReasonMissingIntent:
		return "blocked", []featureBlocker{{Code: "missing_intent", Message: fmt.Sprintf("Gateway intents are missing required bits %d.", decision.MissingMask)}}
	case discordlogging.EmitReasonChannelInvalid:
		return "blocked", []featureBlocker{{Code: "invalid_channel", Message: "The configured channel failed validation for this feature.", Field: "channel_id"}}
	case discordlogging.EmitReasonGuildConfigMissing:
		return "blocked", []featureBlocker{{Code: "missing_guild_registration", Message: "This guild is not registered in settings yet."}}
	case discordlogging.EmitReasonConfigManagerUnavailable, discordlogging.EmitReasonConfigUnavailable:
		return "blocked", []featureBlocker{{Code: "config_unavailable", Message: "Feature config is unavailable."}}
	default:
		return "blocked", []featureBlocker{{Code: "blocked", Message: string(decision.Reason)}}
	}
}

func globalLogRuntimeBlocker(rc files.RuntimeConfig, eventType discordlogging.LogEventType) (featureBlocker, bool) {
	switch eventType {
	case discordlogging.LogEventAvatarChange, discordlogging.LogEventRoleChange:
		if rc.DisableUserLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime user logging is disabled."}, true
		}
	case discordlogging.LogEventMemberJoin, discordlogging.LogEventMemberLeave:
		if rc.DisableEntryExitLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime entry/exit logging is disabled."}, true
		}
	case discordlogging.LogEventMessageProcess, discordlogging.LogEventMessageEdit, discordlogging.LogEventMessageDelete:
		if rc.DisableMessageLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime message logging is disabled."}, true
		}
	case discordlogging.LogEventReactionMetric:
		if rc.DisableReactionLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime reaction logging is disabled."}, true
		}
	case discordlogging.LogEventAutomodAction:
		if rc.DisableAutomodLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime AutoMod logging is disabled."}, true
		}
	case discordlogging.LogEventModerationCase:
		if !rc.ModerationLoggingEnabled() {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime moderation logging is disabled."}, true
		}
	case discordlogging.LogEventCleanAction:
		if rc.DisableCleanLog {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime clean logging is disabled."}, true
		}
	}
	return featureBlocker{}, false
}

func applyGlobalFeaturePatch(cfg *files.BotConfig, def featureDefinition, payload map[string]json.RawMessage) error {
	remaining := cloneRawPayload(payload)

	if present, enabled, err := consumeNullableBool(remaining, "enabled"); err != nil {
		return err
	} else if present {
		setGlobalFeatureToggle(&cfg.Features, def.ID, enabled)
	}

	switch def.ID {
	case "presence_watch.bot":
		if present, value, err := consumeBool(remaining, "watch_bot"); err != nil {
			return err
		} else if present {
			cfg.RuntimeConfig.PresenceWatchBot = value
		}
	case "presence_watch.user":
		if present, value, err := consumeString(remaining, "user_id"); err != nil {
			return err
		} else if present {
			cfg.RuntimeConfig.PresenceWatchUserID = value
		}
	case "safety.bot_role_perm_mirror":
		if present, value, err := consumeString(remaining, "actor_role_id"); err != nil {
			return err
		} else if present {
			cfg.RuntimeConfig.BotRolePermMirrorActorRoleID = value
		}
	case "backfill.enabled":
		if present, value, err := consumeString(remaining, "channel_id"); err != nil {
			return err
		} else if present {
			cfg.RuntimeConfig.BackfillChannelID = value
		}
		if present, value, err := consumeString(remaining, "start_day"); err != nil {
			return err
		} else if present {
			cfg.RuntimeConfig.BackfillStartDay = value
		}
		if present, value, err := consumeString(remaining, "initial_date"); err != nil {
			return err
		} else if present {
			cfg.RuntimeConfig.BackfillInitialDate = value
		}
	}

	if len(remaining) > 0 {
		return unknownPatchFieldsError(remaining)
	}
	next, err := files.NormalizeRuntimeConfig(cfg.RuntimeConfig)
	if err != nil {
		return err
	}
	cfg.RuntimeConfig = next
	return nil
}

func applyGuildFeaturePatch(cfg *files.BotConfig, guild *files.GuildConfig, def featureDefinition, payload map[string]json.RawMessage) error {
	remaining := cloneRawPayload(payload)

	if present, enabled, err := consumeNullableBool(remaining, "enabled"); err != nil {
		return err
	} else if present {
		setGuildFeatureToggle(guild, def.ID, enabled)
	}

	switch def.ID {
	case "services.commands":
		if present, value, err := consumeString(remaining, "channel_id"); err != nil {
			return err
		} else if present {
			guild.Channels.Commands = value
		}
	case "services.admin_commands":
		if present, value, err := consumeStringSlice(remaining, "allowed_role_ids"); err != nil {
			return err
		} else if present {
			guild.Roles.Allowed = normalizeStringList(value)
		}
	case "moderation.mute_role":
		if present, value, err := consumeString(remaining, "role_id"); err != nil {
			return err
		} else if present {
			guild.Roles.MuteRole = value
		}
	case "presence_watch.bot":
		if present, value, err := consumeBool(remaining, "watch_bot"); err != nil {
			return err
		} else if present {
			guild.RuntimeConfig.PresenceWatchBot = value
		}
	case "presence_watch.user":
		if present, value, err := consumeString(remaining, "user_id"); err != nil {
			return err
		} else if present {
			guild.RuntimeConfig.PresenceWatchUserID = value
		}
	case "safety.bot_role_perm_mirror":
		if present, value, err := consumeString(remaining, "actor_role_id"); err != nil {
			return err
		} else if present {
			guild.RuntimeConfig.BotRolePermMirrorActorRoleID = value
		}
	case "backfill.enabled":
		if present, value, err := consumeString(remaining, "channel_id"); err != nil {
			return err
		} else if present {
			guild.Channels.EntryBackfill = value
		}
		if present, value, err := consumeString(remaining, "start_day"); err != nil {
			return err
		} else if present {
			guild.RuntimeConfig.BackfillStartDay = value
		}
		if present, value, err := consumeString(remaining, "initial_date"); err != nil {
			return err
		} else if present {
			guild.RuntimeConfig.BackfillInitialDate = value
		}
	case "stats_channels":
		if present, value, err := consumeBool(remaining, "config_enabled"); err != nil {
			return err
		} else if present {
			guild.Stats.Enabled = value
		}
		if present, value, err := consumeInt(remaining, "update_interval_mins"); err != nil {
			return err
		} else if present {
			guild.Stats.UpdateIntervalMins = value
		}
	case "auto_role_assignment":
		if present, value, err := consumeBool(remaining, "config_enabled"); err != nil {
			return err
		} else if present {
			guild.Roles.AutoAssignment.Enabled = value
		}
		if present, value, err := consumeString(remaining, "target_role_id"); err != nil {
			return err
		} else if present {
			guild.Roles.AutoAssignment.TargetRoleID = value
		}
		if present, value, err := consumeStringSlice(remaining, "required_role_ids"); err != nil {
			return err
		} else if present {
			guild.Roles.AutoAssignment.RequiredRoles = normalizeStringList(value)
			if len(guild.Roles.AutoAssignment.RequiredRoles) >= 2 {
				guild.Roles.BoosterRole = guild.Roles.AutoAssignment.RequiredRoles[1]
			}
		}
	case "user_prune":
		if present, value, err := consumeBool(remaining, "config_enabled"); err != nil {
			return err
		} else if present {
			guild.UserPrune.Enabled = value
		}
		if present, value, err := consumeInt(remaining, "grace_days"); err != nil {
			return err
		} else if present {
			guild.UserPrune.GraceDays = value
		}
		if present, value, err := consumeInt(remaining, "scan_interval_mins"); err != nil {
			return err
		} else if present {
			guild.UserPrune.ScanIntervalMins = value
		}
		if present, value, err := consumeInt(remaining, "initial_delay_secs"); err != nil {
			return err
		} else if present {
			guild.UserPrune.InitialDelaySecs = value
		}
		if present, value, err := consumeInt(remaining, "kicks_per_second"); err != nil {
			return err
		} else if present {
			guild.UserPrune.KicksPerSecond = value
		}
		if present, value, err := consumeInt(remaining, "max_kicks_per_run"); err != nil {
			return err
		} else if present {
			guild.UserPrune.MaxKicksPerRun = value
		}
		if present, value, err := consumeStringSlice(remaining, "exempt_role_ids"); err != nil {
			return err
		} else if present {
			guild.UserPrune.ExemptRoleIDs = normalizeStringList(value)
		}
		if present, value, err := consumeBool(remaining, "dry_run"); err != nil {
			return err
		} else if present {
			guild.UserPrune.DryRun = value
		}
	default:
		if def.LogEvent != "" {
			if present, value, err := consumeString(remaining, "channel_id"); err != nil {
				return err
			} else if present {
				setLogFeatureChannelID(guild, def.LogEvent, value)
			}
		}
	}

	if len(remaining) > 0 {
		return unknownPatchFieldsError(remaining)
	}
	next, err := files.NormalizeRuntimeConfig(guild.RuntimeConfig)
	if err != nil {
		return err
	}
	guild.RuntimeConfig = next
	_ = cfg
	return nil
}

func featureEditableFields(def featureDefinition, guildID string) []string {
	if guildID == "" {
		return slices.Clone(def.GlobalEditableFields)
	}
	return slices.Clone(def.GuildEditableFields)
}

func featureOverrideState(cfg *files.BotConfig, guildID, featureID string) string {
	if guildID == "" {
		ptr := getGlobalFeatureToggle(cfg.Features, featureID)
		if ptr == nil {
			return "default"
		}
		if *ptr {
			return "enabled"
		}
		return "disabled"
	}

	guild, ok := findGuildSettings(*cfg, guildID)
	if !ok {
		return "inherit"
	}
	ptr := getGuildFeatureToggle(&guild, featureID)
	if ptr == nil {
		return "inherit"
	}
	if *ptr {
		return "enabled"
	}
	return "disabled"
}

func featureEffectiveSource(cfg *files.BotConfig, guildID, featureID string) string {
	if guildID != "" {
		if guild, ok := findGuildSettings(*cfg, guildID); ok && getGuildFeatureToggle(&guild, featureID) != nil {
			return "guild"
		}
	}
	if getGlobalFeatureToggle(cfg.Features, featureID) != nil {
		return "global"
	}
	return "built_in"
}

func resolvedFeatureValue(cfg *files.BotConfig, guildID, featureID string) bool {
	resolved := cfg.ResolveFeatures(guildID)
	switch featureID {
	case "services.monitoring":
		return resolved.Services.Monitoring
	case "services.automod":
		return resolved.Services.Automod
	case "moderation.mute_role":
		return resolved.MuteRole
	case "services.commands":
		return resolved.Services.Commands
	case "services.admin_commands":
		return resolved.Services.AdminCommands
	case "logging.avatar_logging":
		return resolved.Logging.AvatarLogging
	case "logging.role_update":
		return resolved.Logging.RoleUpdate
	case "logging.member_join":
		return resolved.Logging.MemberJoin
	case "logging.member_leave":
		return resolved.Logging.MemberLeave
	case "logging.message_process":
		return resolved.Logging.MessageProcess
	case "logging.message_edit":
		return resolved.Logging.MessageEdit
	case "logging.message_delete":
		return resolved.Logging.MessageDelete
	case "logging.reaction_metric":
		return resolved.Logging.ReactionMetric
	case "logging.automod_action":
		return resolved.Logging.AutomodAction
	case "logging.moderation_case":
		return resolved.Logging.ModerationCase
	case "message_cache.cleanup_on_startup":
		return resolved.MessageCache.CleanupOnStartup
	case "message_cache.delete_on_log":
		return resolved.MessageCache.DeleteOnLog
	case "presence_watch.bot":
		return resolved.PresenceWatch.Bot
	case "presence_watch.user":
		return resolved.PresenceWatch.User
	case "maintenance.db_cleanup":
		return resolved.Maintenance.DBCleanup
	case "safety.bot_role_perm_mirror":
		return resolved.Safety.BotRolePermMirror
	case "backfill.enabled":
		return resolved.Backfill.Enabled
	case "stats_channels":
		return resolved.StatsChannels
	case "auto_role_assignment":
		return resolved.AutoRoleAssign
	case "user_prune":
		return resolved.UserPrune
	default:
		return false
	}
}

func getGlobalFeatureToggle(ft files.FeatureToggles, featureID string) *bool {
	switch featureID {
	case "services.monitoring":
		return ft.Services.Monitoring
	case "services.automod":
		return ft.Services.Automod
	case "moderation.mute_role":
		return ft.MuteRole
	case "services.commands":
		return ft.Services.Commands
	case "services.admin_commands":
		return ft.Services.AdminCommands
	case "logging.avatar_logging":
		return ft.Logging.AvatarLogging
	case "logging.role_update":
		return ft.Logging.RoleUpdate
	case "logging.member_join":
		return ft.Logging.MemberJoin
	case "logging.member_leave":
		return ft.Logging.MemberLeave
	case "logging.message_process":
		return ft.Logging.MessageProcess
	case "logging.message_edit":
		return ft.Logging.MessageEdit
	case "logging.message_delete":
		return ft.Logging.MessageDelete
	case "logging.reaction_metric":
		return ft.Logging.ReactionMetric
	case "logging.automod_action":
		return ft.Logging.AutomodAction
	case "logging.moderation_case":
		return ft.Logging.ModerationCase
	case "message_cache.cleanup_on_startup":
		return ft.MessageCache.CleanupOnStartup
	case "message_cache.delete_on_log":
		return ft.MessageCache.DeleteOnLog
	case "presence_watch.bot":
		return ft.PresenceWatch.Bot
	case "presence_watch.user":
		return ft.PresenceWatch.User
	case "maintenance.db_cleanup":
		return ft.Maintenance.DBCleanup
	case "safety.bot_role_perm_mirror":
		return ft.Safety.BotRolePermMirror
	case "backfill.enabled":
		return ft.Backfill.Enabled
	case "stats_channels":
		return ft.StatsChannels
	case "auto_role_assignment":
		return ft.AutoRoleAssign
	case "user_prune":
		return ft.UserPrune
	default:
		return nil
	}
}

func setGlobalFeatureToggle(ft *files.FeatureToggles, featureID string, value *bool) {
	switch featureID {
	case "services.monitoring":
		ft.Services.Monitoring = cloneBool(value)
	case "services.automod":
		ft.Services.Automod = cloneBool(value)
	case "moderation.mute_role":
		ft.MuteRole = cloneBool(value)
	case "services.commands":
		ft.Services.Commands = cloneBool(value)
	case "services.admin_commands":
		ft.Services.AdminCommands = cloneBool(value)
	case "logging.avatar_logging":
		ft.Logging.AvatarLogging = cloneBool(value)
	case "logging.role_update":
		ft.Logging.RoleUpdate = cloneBool(value)
	case "logging.member_join":
		ft.Logging.MemberJoin = cloneBool(value)
	case "logging.member_leave":
		ft.Logging.MemberLeave = cloneBool(value)
	case "logging.message_process":
		ft.Logging.MessageProcess = cloneBool(value)
	case "logging.message_edit":
		ft.Logging.MessageEdit = cloneBool(value)
	case "logging.message_delete":
		ft.Logging.MessageDelete = cloneBool(value)
	case "logging.reaction_metric":
		ft.Logging.ReactionMetric = cloneBool(value)
	case "logging.automod_action":
		ft.Logging.AutomodAction = cloneBool(value)
	case "logging.moderation_case":
		ft.Logging.ModerationCase = cloneBool(value)
	case "message_cache.cleanup_on_startup":
		ft.MessageCache.CleanupOnStartup = cloneBool(value)
	case "message_cache.delete_on_log":
		ft.MessageCache.DeleteOnLog = cloneBool(value)
	case "presence_watch.bot":
		ft.PresenceWatch.Bot = cloneBool(value)
	case "presence_watch.user":
		ft.PresenceWatch.User = cloneBool(value)
	case "maintenance.db_cleanup":
		ft.Maintenance.DBCleanup = cloneBool(value)
	case "safety.bot_role_perm_mirror":
		ft.Safety.BotRolePermMirror = cloneBool(value)
	case "backfill.enabled":
		ft.Backfill.Enabled = cloneBool(value)
	case "stats_channels":
		ft.StatsChannels = cloneBool(value)
	case "auto_role_assignment":
		ft.AutoRoleAssign = cloneBool(value)
	case "user_prune":
		ft.UserPrune = cloneBool(value)
	}
}

func getGuildFeatureToggle(guild *files.GuildConfig, featureID string) *bool {
	if guild == nil {
		return nil
	}
	return getGlobalFeatureToggle(guild.Features, featureID)
}

func setGuildFeatureToggle(guild *files.GuildConfig, featureID string, value *bool) {
	if guild == nil {
		return
	}
	setGlobalFeatureToggle(&guild.Features, featureID, value)
}

func normalizeFeatureRoutePath(path string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

func splitGlobalFeatureRoute(path string) (string, bool) {
	const prefix = "/v1/features/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" || strings.Contains(trimmed, "/") {
		return "", false
	}
	return trimmed, true
}

func cloneRawPayload(in map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func consumeNullableBool(payload map[string]json.RawMessage, key string) (bool, *bool, error) {
	raw, ok := payload[key]
	if !ok {
		return false, nil, nil
	}
	delete(payload, key)
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true, nil, nil
	}
	var parsed bool
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, nil, featurePatchBadRequestError{message: fmt.Sprintf("%s must be a boolean or null: %v", key, err)}
	}
	return true, &parsed, nil
}

func consumeBool(payload map[string]json.RawMessage, key string) (bool, bool, error) {
	raw, ok := payload[key]
	if !ok {
		return false, false, nil
	}
	delete(payload, key)
	var parsed bool
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, false, featurePatchBadRequestError{message: fmt.Sprintf("%s must be a boolean: %v", key, err)}
	}
	return true, parsed, nil
}

func consumeString(payload map[string]json.RawMessage, key string) (bool, string, error) {
	raw, ok := payload[key]
	if !ok {
		return false, "", nil
	}
	delete(payload, key)
	var parsed string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, "", featurePatchBadRequestError{message: fmt.Sprintf("%s must be a string: %v", key, err)}
	}
	return true, strings.TrimSpace(parsed), nil
}

func consumeStringSlice(payload map[string]json.RawMessage, key string) (bool, []string, error) {
	raw, ok := payload[key]
	if !ok {
		return false, nil, nil
	}
	delete(payload, key)
	var parsed []string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, nil, featurePatchBadRequestError{message: fmt.Sprintf("%s must be a string array: %v", key, err)}
	}
	return true, parsed, nil
}

func consumeInt(payload map[string]json.RawMessage, key string) (bool, int, error) {
	raw, ok := payload[key]
	if !ok {
		return false, 0, nil
	}
	delete(payload, key)
	var parsed int
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, 0, featurePatchBadRequestError{message: fmt.Sprintf("%s must be an integer: %v", key, err)}
	}
	return true, parsed, nil
}

func unknownPatchFieldsError(payload map[string]json.RawMessage) error {
	fields := make([]string, 0, len(payload))
	for key := range payload {
		fields = append(fields, key)
	}
	slices.Sort(fields)
	return featurePatchBadRequestError{message: fmt.Sprintf("unsupported patch field(s): %s", strings.Join(fields, ", "))}
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func cloneBool(in *bool) *bool {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func logFeatureChannelID(guild *files.GuildConfig, eventType discordlogging.LogEventType) string {
	if guild == nil {
		return ""
	}
	switch eventType {
	case discordlogging.LogEventAvatarChange:
		return strings.TrimSpace(guild.Channels.AvatarLogging)
	case discordlogging.LogEventRoleChange:
		return strings.TrimSpace(guild.Channels.RoleUpdate)
	case discordlogging.LogEventMemberJoin:
		return strings.TrimSpace(guild.Channels.MemberJoin)
	case discordlogging.LogEventMemberLeave:
		return strings.TrimSpace(guild.Channels.MemberLeave)
	case discordlogging.LogEventMessageEdit:
		return strings.TrimSpace(guild.Channels.MessageEdit)
	case discordlogging.LogEventMessageDelete:
		return strings.TrimSpace(guild.Channels.MessageDelete)
	case discordlogging.LogEventAutomodAction:
		return strings.TrimSpace(guild.Channels.AutomodAction)
	case discordlogging.LogEventModerationCase:
		return strings.TrimSpace(guild.Channels.ModerationCase)
	case discordlogging.LogEventCleanAction:
		return strings.TrimSpace(guild.Channels.CleanAction)
	default:
		return ""
	}
}

func setLogFeatureChannelID(guild *files.GuildConfig, eventType discordlogging.LogEventType, channelID string) {
	if guild == nil {
		return
	}
	switch eventType {
	case discordlogging.LogEventAvatarChange:
		guild.Channels.AvatarLogging = channelID
	case discordlogging.LogEventRoleChange:
		guild.Channels.RoleUpdate = channelID
	case discordlogging.LogEventMemberJoin:
		guild.Channels.MemberJoin = channelID
	case discordlogging.LogEventMemberLeave:
		guild.Channels.MemberLeave = channelID
	case discordlogging.LogEventMessageEdit:
		guild.Channels.MessageEdit = channelID
	case discordlogging.LogEventMessageDelete:
		guild.Channels.MessageDelete = channelID
	case discordlogging.LogEventAutomodAction:
		guild.Channels.AutomodAction = channelID
	case discordlogging.LogEventModerationCase:
		guild.Channels.ModerationCase = channelID
	case discordlogging.LogEventCleanAction:
		guild.Channels.CleanAction = channelID
	}
}

func (s *Server) currentDiscordSession() (*discordgo.Session, error) {
	return s.discordSessionForGuild("")
}

func (s *Server) discordSessionForGuild(guildID string) (*discordgo.Session, error) {
	if s == nil || s.discordSession == nil {
		return nil, nil
	}
	return s.discordSession(guildID)
}
