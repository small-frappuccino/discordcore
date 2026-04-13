package control

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/files"
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
		return files.BotConfig{}, err
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
		return files.BotConfig{}, err
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
		return err
	} else if present {
		setGlobalFeatureToggle(&cfg.Features, def.ID, enabled)
	}

	if handler, ok := globalFeaturePatchHandlers[def.ID]; ok {
		if err := handler(cfg, remaining); err != nil {
			return err
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

func (applier *featureMutationApplier) applyGuildPatch(
	cfg *files.BotConfig,
	guild *files.GuildConfig,
	def featureDefinition,
	payload map[string]json.RawMessage,
) error {
	remaining := cloneRawPayload(payload)

	if present, enabled, err := consumeNullableBool(remaining, "enabled"); err != nil {
		return err
	} else if present {
		setGuildFeatureToggle(guild, def.ID, enabled)
	}

	if handler, ok := guildFeaturePatchHandlers[def.ID]; ok {
		if err := handler(cfg, guild, remaining); err != nil {
			return err
		}
	} else if def.LogEvent != "" {
		if present, value, err := consumeString(remaining, "channel_id"); err != nil {
			return err
		} else if present {
			setLogFeatureChannelID(guild, def.LogEvent, value)
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
	return nil
}

type globalFeaturePatchHandler func(*files.BotConfig, map[string]json.RawMessage) error
type guildFeaturePatchHandler func(*files.BotConfig, *files.GuildConfig, map[string]json.RawMessage) error

var globalFeaturePatchHandlers = map[string]globalFeaturePatchHandler{
	"presence_watch.bot": func(cfg *files.BotConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeBool(remaining, "watch_bot"); err != nil {
			return err
		} else if present {
			cfg.RuntimeConfig.PresenceWatchBot = value
		}
		return nil
	},
	"presence_watch.user": func(cfg *files.BotConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "user_id"); err != nil {
			return err
		} else if present {
			cfg.RuntimeConfig.PresenceWatchUserID = value
		}
		return nil
	},
	"safety.bot_role_perm_mirror": func(cfg *files.BotConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "actor_role_id"); err != nil {
			return err
		} else if present {
			cfg.RuntimeConfig.BotRolePermMirrorActorRoleID = value
		}
		return nil
	},
	"backfill.enabled": func(cfg *files.BotConfig, remaining map[string]json.RawMessage) error {
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
		return nil
	},
}

var guildFeaturePatchHandlers = map[string]guildFeaturePatchHandler{
	"services.commands": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "channel_id"); err != nil {
			return err
		} else if present {
			guild.Channels.Commands = value
		}
		return nil
	},
	"services.admin_commands": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeStringSlice(remaining, "allowed_role_ids"); err != nil {
			return err
		} else if present {
			guild.Roles.Allowed = normalizeStringList(value)
		}
		return nil
	},
	"moderation.mute_role": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "role_id"); err != nil {
			return err
		} else if present {
			guild.Roles.MuteRole = value
		}
		return nil
	},
	"presence_watch.bot": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeBool(remaining, "watch_bot"); err != nil {
			return err
		} else if present {
			guild.RuntimeConfig.PresenceWatchBot = value
		}
		return nil
	},
	"presence_watch.user": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "user_id"); err != nil {
			return err
		} else if present {
			guild.RuntimeConfig.PresenceWatchUserID = value
		}
		return nil
	},
	"safety.bot_role_perm_mirror": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
		if present, value, err := consumeString(remaining, "actor_role_id"); err != nil {
			return err
		} else if present {
			guild.RuntimeConfig.BotRolePermMirrorActorRoleID = value
		}
		return nil
	},
	"backfill.enabled": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
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
		return nil
	},
	"stats_channels": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
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
		return nil
	},
	"auto_role_assignment": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
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
		return nil
	},
	"user_prune": func(_ *files.BotConfig, guild *files.GuildConfig, remaining map[string]json.RawMessage) error {
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
