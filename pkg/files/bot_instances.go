package files

import "strings"

const BotDomainQOTD = "qotd"

// NormalizeBotInstanceID trims a persisted bot instance identifier.
func NormalizeBotInstanceID(botInstanceID string) string {
	return strings.TrimSpace(botInstanceID)
}

// NormalizeBotDomain canonicalizes a persisted or requested domain key.
func NormalizeBotDomain(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}

// EffectiveBotInstanceID resolves the guild bot binding, falling back to the
// provided default instance ID when the config is unset.
func (gc GuildConfig) EffectiveBotInstanceID(defaultBotInstanceID string) string {
	if botInstanceID := NormalizeBotInstanceID(gc.BotInstanceID); botInstanceID != "" {
		return botInstanceID
	}
	return NormalizeBotInstanceID(defaultBotInstanceID)
}

// BotInstanceIDOverrideForDomain returns the explicit bot-instance override for
// the provided domain, or an empty string when the domain falls back to the
// guild-wide bot binding.
func (gc GuildConfig) BotInstanceIDOverrideForDomain(domain string) string {
	domain = NormalizeBotDomain(domain)
	if domain == "" || len(gc.DomainBotInstanceIDs) == 0 {
		return ""
	}

	for configuredDomain, botInstanceID := range gc.DomainBotInstanceIDs {
		if NormalizeBotDomain(configuredDomain) != domain {
			continue
		}
		if normalizedBotInstanceID := NormalizeBotInstanceID(botInstanceID); normalizedBotInstanceID != "" {
			return normalizedBotInstanceID
		}
	}

	return ""
}

// EffectiveBotInstanceIDForDomain resolves the bot binding for a specialized
// domain, falling back to the guild-wide binding and then the runtime default.
func (gc GuildConfig) EffectiveBotInstanceIDForDomain(domain, defaultBotInstanceID string) string {
	if botInstanceID := gc.BotInstanceIDOverrideForDomain(domain); botInstanceID != "" {
		return botInstanceID
	}
	return gc.EffectiveBotInstanceID(defaultBotInstanceID)
}

// BelongsToBotInstance reports whether the guild should be handled by the
// provided runtime, considering the default binding for legacy configs.
func (gc GuildConfig) BelongsToBotInstance(botInstanceID, defaultBotInstanceID string) bool {
	return gc.BelongsToBotInstanceForDomain("", botInstanceID, defaultBotInstanceID)
}

// BelongsToBotInstanceForDomain reports whether the provided runtime should own
// the requested domain for the guild, considering explicit domain overrides and
// legacy guild-wide bindings.
func (gc GuildConfig) BelongsToBotInstanceForDomain(domain, botInstanceID, defaultBotInstanceID string) bool {
	return gc.EffectiveBotInstanceIDForDomain(domain, defaultBotInstanceID) == NormalizeBotInstanceID(botInstanceID)
}

// GuildsForBotInstance returns the guild subset assigned to the provided bot
// instance, preserving config order.
func (cfg *BotConfig) GuildsForBotInstance(botInstanceID, defaultBotInstanceID string) []GuildConfig {
	return cfg.GuildsForBotInstanceForDomain("", botInstanceID, defaultBotInstanceID)
}

// GuildsForBotInstanceForDomain returns the guild subset whose requested
// domain resolves to the provided bot instance, preserving config order.
func (cfg *BotConfig) GuildsForBotInstanceForDomain(domain, botInstanceID, defaultBotInstanceID string) []GuildConfig {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}

	domain = NormalizeBotDomain(domain)
	target := NormalizeBotInstanceID(botInstanceID)
	out := make([]GuildConfig, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if guild.BelongsToBotInstanceForDomain(domain, target, defaultBotInstanceID) {
			out = append(out, guild)
		}
	}
	return out
}
