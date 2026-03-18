package files

import "strings"

// NormalizeBotInstanceID trims a persisted bot instance identifier.
func NormalizeBotInstanceID(botInstanceID string) string {
	return strings.TrimSpace(botInstanceID)
}

// EffectiveBotInstanceID resolves the guild bot binding, falling back to the
// provided default instance ID when the config is unset.
func (gc GuildConfig) EffectiveBotInstanceID(defaultBotInstanceID string) string {
	if botInstanceID := NormalizeBotInstanceID(gc.BotInstanceID); botInstanceID != "" {
		return botInstanceID
	}
	return NormalizeBotInstanceID(defaultBotInstanceID)
}

// BelongsToBotInstance reports whether the guild should be handled by the
// provided runtime, considering the default binding for legacy configs.
func (gc GuildConfig) BelongsToBotInstance(botInstanceID, defaultBotInstanceID string) bool {
	return gc.EffectiveBotInstanceID(defaultBotInstanceID) == NormalizeBotInstanceID(botInstanceID)
}

// GuildsForBotInstance returns the guild subset assigned to the provided bot
// instance, preserving config order.
func (cfg *BotConfig) GuildsForBotInstance(botInstanceID, defaultBotInstanceID string) []GuildConfig {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}

	target := NormalizeBotInstanceID(botInstanceID)
	out := make([]GuildConfig, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if guild.BelongsToBotInstance(target, defaultBotInstanceID) {
			out = append(out, guild)
		}
	}
	return out
}
