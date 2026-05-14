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
