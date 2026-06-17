package control

import (
	"slices"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func featureEditableFields(def featureDefinition, guildID string) []string {
	if guildID == "" {
		return slices.Clone(def.GlobalEditableFields)
	}
	return slices.Clone(def.GuildEditableFields)
}

func featureOverrideState(cfg *files.BotConfig, guildID, featureID string) string {
	if featureID == "stats_channels" || featureID == "auto_role_assignment" || featureID == "user_prune" {
		return "inherit" // No explicit override state for configuration-based features
	}

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
	if featureID == "stats_channels" || featureID == "auto_role_assignment" || featureID == "user_prune" {
		return "built_in" // Driven by presence
	}

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
	switch featureID {
	case "stats_channels":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				return len(guild.Stats.Channels) > 0
			}
		}
		return false
	case "auto_role_assignment":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				return guild.Roles.AutoAssignment.Enabled
			}
		}
		return false
	case "user_prune":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				return guild.UserPrune.Enabled
			}
		}
		return false
	}

	resolved := cfg.ResolveFeatures(guildID)
	enabled, _ := resolved.Lookup(featureID)
	return enabled
}

func getGlobalFeatureToggle(ft files.FeatureToggles, featureID string) *bool {
	return ft.LookupToggle(featureID)
}

func setGlobalFeatureToggle(ft *files.FeatureToggles, featureID string, value *bool) {
	ft.SetToggle(featureID, value)
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
