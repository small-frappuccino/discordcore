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
		return false
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
	out := make([]GuildConfig, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if guild.BelongsToBotInstance(target) {
			out = append(out, guild)
		}
	}
	return out
}
