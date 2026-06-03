package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
)

type featureMutationApplier struct {
	configManager *files.ConfigManager
}

func newFeatureMutationApplier(configManager *files.ConfigManager) *featureMutationApplier {
	return &featureMutationApplier{configManager: configManager}
}

func (applier *featureMutationApplier) ApplyPatch(
	r *http.Request,
	guildID string,
	featureID string,
) (files.BotConfig, error) {
	if applier == nil || applier.configManager == nil {
		return files.BotConfig{}, fmt.Errorf("config manager unavailable")
	}

	def, ok := featureDefinitionsByID[featureID]
	if !ok {
		return files.BotConfig{}, fmt.Errorf("%w: %s", errUnknownFeatureID, featureID)
	}

	payload, err := decodeFeaturePatchPayload(r)
	if err != nil {
		return files.BotConfig{}, fmt.Errorf("featureMutationApplier.ApplyPatch: %w", err)
	}
	if len(payload) == 0 {
		return files.BotConfig{}, featurePatchBadRequestError{message: "payload must contain at least one field"}
	}

	updated, err := applier.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		if guildID == "" {
			return applier.applyGlobalPatch(cfg, def, payload)
		}
		guild, ok := findGuildSettingsMutable(cfg, guildID)
		if !ok {
			return fmt.Errorf("%w: register this guild first (guild_id=%s)", errGuildRegistrationRequired, guildID)
		}
		return applier.applyGuildPatch(cfg, guild, def, payload)
	})
	if err != nil {
		return files.BotConfig{}, fmt.Errorf("featureMutationApplier.ApplyPatch: %w", err)
	}
	return updated, nil
}

func (applier *featureMutationApplier) applyGlobalPatch(
	cfg *files.BotConfig,
	def featureDefinition,
	payload map[string]json.RawMessage,
) error {
	remaining := cloneRawPayload(payload)

	if present, enabled, err := consumeNullableBool(remaining, "enabled"); err != nil {
		return fmt.Errorf("featureMutationApplier.applyGlobalPatch: %w", err)
	} else if present {
		setGlobalFeatureToggle(&cfg.Features, def.ID, enabled)
	}

	if handler, ok := globalFeaturePatchHandlers[def.ID]; ok {
		if err := handler(cfg, remaining); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGlobalPatch: %w", err)
		}
	}

	if len(remaining) > 0 {
		return unknownPatchFieldsError(remaining)
	}
	next, err := files.NormalizeRuntimeConfig(cfg.RuntimeConfig)
	if err != nil {
		return fmt.Errorf("featureMutationApplier.applyGlobalPatch: %w", err)
	}
	cfg.RuntimeConfig = next
	return nil
}

func (applier *featureMutationApplier) applyGuildPatch(
	cfg *files.BotConfig,
	guild *files.GuildConfig,
	def featureDefinition,
	payload map[string]json.RawMessage,
) error {
	remaining := cloneRawPayload(payload)

	if present, enabled, err := consumeNullableBool(remaining, "enabled"); err != nil {
		return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
	} else if present {
		setGuildFeatureToggle(guild, def.ID, enabled)
	}

	if handler, ok := guildFeaturePatchHandlers[def.ID]; ok {
		if err := handler(cfg, guild, remaining); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		}
	} else if def.LogEvent != "" {
		if present, value, err := consumeString(remaining, "channel_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			setLogFeatureChannelID(guild, def.LogEvent, value)
		}
	}

	if len(remaining) > 0 {
		return unknownPatchFieldsError(remaining)
	}
	next, err := files.NormalizeRuntimeConfig(guild.RuntimeConfig)
	if err != nil {
		return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
	}
	guild.RuntimeConfig = next
	return nil
}

type globalFeaturePatchHandler func(*files.BotConfig, map[string]json.RawMessage) error
type guildFeaturePatchHandler func(*files.BotConfig, *files.GuildConfig, map[string]json.RawMessage) error

var globalFeaturePatchHandlers = map[string]globalFeaturePatchHandler{
	"presence_watch.bot": func(cfg *files.BotConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeBool(remaining, "watch_bot"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			cfg.RuntimeConfig.PresenceWatchBot = value
		}
		return nil
	},
	"presence_watch.user": func(cfg *files.BotConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "user_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			cfg.RuntimeConfig.PresenceWatchUserID = value
		}
		return nil
	},
	"safety.bot_role_perm_mirror": func(cfg *files.BotConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "actor_role_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			cfg.RuntimeConfig.BotRolePermMirrorActorRoleID = value
		}
		return nil
	},
	"backfill.enabled": func(cfg *files.BotConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "channel_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			cfg.RuntimeConfig.BackfillChannelID = value
		}
		if present, value, err := consumeString(remaining, "start_day"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			cfg.RuntimeConfig.BackfillStartDay = value
		}
		if present, value, err := consumeString(remaining, "initial_date"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			cfg.RuntimeConfig.BackfillInitialDate = value
		}
		return nil
	},
}

var guildFeaturePatchHandlers = map[string]guildFeaturePatchHandler{
	"services.commands": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "channel_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.Channels.Commands = value
		}
		return nil
	},
	"services.admin_commands": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeStringSlice(remaining, "allowed_role_ids"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.Roles.Allowed = normalizeStringList(value)
		}
		return nil
	},
	"moderation.mute_role": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "role_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.Roles.MuteRole = value
		}
		return nil
	},
	"presence_watch.bot": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeBool(remaining, "watch_bot"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.RuntimeConfig.PresenceWatchBot = value
		}
		return nil
	},
	"presence_watch.user": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "user_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.RuntimeConfig.PresenceWatchUserID = value
		}
		return nil
	},
	"safety.bot_role_perm_mirror": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "actor_role_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.RuntimeConfig.BotRolePermMirrorActorRoleID = value
		}
		return nil
	},
	"backfill.enabled": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "channel_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.Channels.EntryBackfill = value
		}
		if present, value, err := consumeString(remaining, "start_day"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.RuntimeConfig.BackfillStartDay = value
		}
		if present, value, err := consumeString(remaining, "initial_date"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.RuntimeConfig.BackfillInitialDate = value
		}
		return nil
	},
	"stats_channels": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeBool(remaining, "config_enabled"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.Stats.Enabled = value
		}
		if present, value, err := consumeInt(remaining, "update_interval_mins"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.Stats.UpdateIntervalMins = value
		}
		return nil
	},
	"auto_role_assignment": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeBool(remaining, "config_enabled"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.Roles.AutoAssignment.Enabled = value
		}
		if present, value, err := consumeString(remaining, "target_role_id"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.Roles.AutoAssignment.TargetRoleID = value
		}
		if present, value, err := consumeStringSlice(remaining, "required_role_ids"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.Roles.AutoAssignment.RequiredRoles = normalizeStringList(value)
			if len(guild.Roles.AutoAssignment.RequiredRoles) >= 2 {
				guild.Roles.BoosterRole = guild.Roles.AutoAssignment.RequiredRoles[1]
			}
		}
		return nil
	},
	"user_prune": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeBool(remaining, "config_enabled"); err != nil {
			return fmt.Errorf("featureMutationApplier.applyGuildPatch: %w", err)
		} else if present {
			guild.UserPrune.Enabled = value
		}
		return nil
	},
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

func setLogFeatureChannelID(guild *files.GuildConfig, eventType logpolicy.LogEventType, channelID string) {
	if guild == nil {
		return
	}
	switch eventType {
	case logpolicy.LogEventAvatarChange:
		guild.Channels.AvatarLogging = channelID
	case logpolicy.LogEventRoleChange:
		guild.Channels.RoleUpdate = channelID
	case logpolicy.LogEventMemberJoin:
		guild.Channels.MemberJoin = channelID
	case logpolicy.LogEventMemberLeave:
		guild.Channels.MemberLeave = channelID
	case logpolicy.LogEventMessageEdit:
		guild.Channels.MessageEdit = channelID
	case logpolicy.LogEventMessageDelete:
		guild.Channels.MessageDelete = channelID
	case logpolicy.LogEventAutomodAction:
		guild.Channels.AutomodAction = channelID
	case logpolicy.LogEventModerationCase:
		guild.Channels.ModerationCase = channelID
	case logpolicy.LogEventCleanAction:
		guild.Channels.CleanAction = channelID
	}
}
