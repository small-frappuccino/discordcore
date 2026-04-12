package control

import (
	"encoding/json"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

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
	return nil
}
