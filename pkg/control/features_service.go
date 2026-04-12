package control

import (
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

type featureControlService struct {
	configManager   *files.ConfigManager
	discordSessions discordSessionResolver
}

func newFeatureControlService(
	configManager *files.ConfigManager,
	discordSessions discordSessionResolver,
) *featureControlService {
	return &featureControlService{
		configManager:   configManager,
		discordSessions: discordSessions,
	}
}

func (s *Server) featureControl() *featureControlService {
	if s == nil {
		return nil
	}
	return newFeatureControlService(s.configManager, s.discordSession)
}

func (svc *featureControlService) catalog() []featureCatalogEntry {
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
	return items
}

func (svc *featureControlService) workspace(guildID string) (featureWorkspace, error) {
	if svc == nil || svc.configManager == nil {
		return featureWorkspace{}, fmt.Errorf("config manager unavailable")
	}

	cfg := svc.configManager.SnapshotConfig()
	if guildID != "" {
		if _, ok := findGuildSettings(cfg, guildID); !ok {
			return featureWorkspace{}, fmt.Errorf("%w: guild_id=%s", files.ErrGuildConfigNotFound, guildID)
		}
	}

	session, err := svc.discordSessionForGuild(guildID)
	if err != nil {
		return featureWorkspace{}, err
	}
	return svc.buildWorkspace(cfg, guildID, session)
}

func (svc *featureControlService) feature(guildID, featureID string) (featureRecord, error) {
	if svc == nil || svc.configManager == nil {
		return featureRecord{}, fmt.Errorf("config manager unavailable")
	}

	cfg := svc.configManager.SnapshotConfig()
	if guildID != "" {
		if _, ok := findGuildSettings(cfg, guildID); !ok {
			return featureRecord{}, fmt.Errorf("%w: guild_id=%s", files.ErrGuildConfigNotFound, guildID)
		}
	}

	session, err := svc.discordSessionForGuild(guildID)
	if err != nil {
		return featureRecord{}, err
	}
	return svc.buildSingleFeatureRecord(cfg, guildID, featureID, session)
}

func (svc *featureControlService) patch(r *http.Request, guildID, featureID string) (featureRecord, error) {
	updated, err := svc.applyPatch(r, guildID, featureID)
	if err != nil {
		return featureRecord{}, err
	}

	session, err := svc.discordSessionForGuild(guildID)
	if err != nil {
		return featureRecord{}, err
	}
	return svc.buildSingleFeatureRecord(updated, guildID, featureID, session)
}

func (svc *featureControlService) applyPatch(r *http.Request, guildID, featureID string) (files.BotConfig, error) {
	if svc == nil || svc.configManager == nil {
		return files.BotConfig{}, fmt.Errorf("config manager unavailable")
	}

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

	updated, err := svc.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
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

func (svc *featureControlService) currentDiscordSession() (*discordgo.Session, error) {
	return svc.discordSessionForGuild("")
}

func (svc *featureControlService) discordSessionForGuild(guildID string) (*discordgo.Session, error) {
	if svc == nil || svc.discordSessions == nil {
		return nil, nil
	}
	return svc.discordSessions(guildID)
}

func (svc *featureControlService) buildWorkspace(
	cfg files.BotConfig,
	guildID string,
	session *discordgo.Session,
) (featureWorkspace, error) {
	records := make([]featureRecord, 0, len(featureDefinitions))
	for _, def := range featureDefinitions {
		record, err := svc.buildFeatureRecord(cfg, guildID, def, session)
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

func (svc *featureControlService) buildSingleFeatureRecord(
	cfg files.BotConfig,
	guildID string,
	featureID string,
	session *discordgo.Session,
) (featureRecord, error) {
	def, ok := featureDefinitionsByID[featureID]
	if !ok {
		return featureRecord{}, fmt.Errorf("%w: %s", errUnknownFeatureID, featureID)
	}
	return svc.buildFeatureRecord(cfg, guildID, def, session)
}

func (svc *featureControlService) buildFeatureRecord(
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
	readiness, blockers := buildFeatureReadiness(&cfg, svc.configManager, guildID, def, effectiveEnabled, session)
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
		Details:               svc.buildFeatureDetails(&cfg, guildID, def),
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

func (svc *featureControlService) buildFeatureDetails(
	cfg *files.BotConfig,
	guildID string,
	def featureDefinition,
) map[string]any {
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

func (s *Server) handleFeatureCatalogGet(w http.ResponseWriter) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"catalog": svc.catalog(),
	})
}

func (s *Server) handleGlobalFeaturePatch(w http.ResponseWriter, r *http.Request, featureID string) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	record, err := svc.patch(r, "", featureID)
	if err != nil {
		status := statusForFeatureMutationError(err)
		http.Error(w, fmt.Sprintf("failed to update feature: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"feature": record,
	})
}

func (s *Server) handleGuildFeaturePatch(w http.ResponseWriter, r *http.Request, guildID, featureID string) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	record, err := svc.patch(r, guildID, featureID)
	if err != nil {
		status := statusForFeatureMutationError(err)
		http.Error(w, fmt.Sprintf("failed to update guild feature: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"feature":  record,
	})
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
