package files

import "strings"

// NormalizeBotInstanceID trims a persisted bot instance identifier.
func NormalizeBotInstanceID(botInstanceID string) string {
	return strings.TrimSpace(botInstanceID)
}

// BelongsToBotInstance reports whether the guild should be handled by the
// provided runtime, which is true if the guild has a configured token for it.
func (gc GuildConfig) BelongsToBotInstance(botInstanceID string) bool {
	botInstanceID = NormalizeBotInstanceID(botInstanceID)
	if botInstanceID == "" {
		return true
	}
	token, ok := gc.BotInstanceTokens[botInstanceID]
	return ok && len(token) > 0
}

// GuildsForBotInstance returns the guild subset assigned to the provided bot instance,
// preserving config order.
func (cfg *BotConfig) GuildsForBotInstance(botInstanceID string) []GuildConfig {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}

	target := NormalizeBotInstanceID(botInstanceID)
	if target == "" {
		out := make([]GuildConfig, len(cfg.Guilds))
		copy(out, cfg.Guilds)
		return out
	}

	out := make([]GuildConfig, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if guild.BelongsToBotInstance(target) {
			out = append(out, guild)
		}
	}
	return out
}

// GuildsForBotInstanceFeature returns the guild subset assigned to the provided bot instance for a specific feature,
// preserving config order.
func (cfg *BotConfig) GuildsForBotInstanceFeature(botInstanceID string, feature string, fallbackID string) []GuildConfig {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}

	target := NormalizeBotInstanceID(botInstanceID)
	if target == "" {
		out := make([]GuildConfig, len(cfg.Guilds))
		copy(out, cfg.Guilds)
		return out
	}

	out := make([]GuildConfig, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if !guild.BelongsToBotInstance(target) {
			continue
		}
		resolvedID, _ := guild.ResolveFeatureBotInstanceID(feature, fallbackID)
		if resolvedID == target {
			out = append(out, guild)
		}
	}
	return out
}

// ResolveFeatureBotInstanceID returns the designated bot instance for a given feature.
// It explicitly parses FeatureRouting and falls back to fallbackID.
// It returns the resolved instance ID and a boolean fallbackFlag
// indicating if the designated bot token was revoked, invalid, or missing, necessitating
// a degradation to the fallback bot.
func (gc GuildConfig) ResolveFeatureBotInstanceID(feature string, fallbackID string) (resolvedID string, fallback bool) {
	route := gc.FeatureRouting[feature]
	if route == "" {
		route = fallbackID
	}

	if route == "" {
		return "", false
	}

	token, ok := gc.BotInstanceTokens[route]
	if !ok || len(token) == 0 {
		return fallbackID, true
	}
	return route, false
}
