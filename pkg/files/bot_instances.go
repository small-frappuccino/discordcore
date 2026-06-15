package files

import (
	"log/slog"
	"strings"
)

// NormalizeBotInstanceID trims a persisted bot instance identifier.
func NormalizeBotInstanceID(botInstanceID string) string {
	return strings.TrimSpace(botInstanceID)
}

// BelongsToBotInstance reports whether the guild should be handled by the
// provided runtime, which is true if the guild has a configured token for it.
func (gc GuildConfig) BelongsToBotInstance(botInstanceID string) bool {
	botInstanceID = NormalizeBotInstanceID(botInstanceID)

	// If the guild has gracefully fallen back due to having NO bot tokens,
	// the magic blank instance handles it.
	if len(gc.BotInstanceTokens) == 0 {
		slog.Debug("Transient state inspection: Conditional evaluation on empty token vector resulting in instance fallback",
			slog.String("guild_id", gc.GuildID),
		)
		return botInstanceID == ""
	}

	token, ok := gc.BotInstanceTokens[botInstanceID]
	match := ok && len(token) > 0

	slog.Debug("Transient state inspection: Membership resolution computed in guild tree",
		slog.String("guild_id", gc.GuildID),
		slog.String("bot_instance_id", botInstanceID),
		slog.Bool("match", match),
	)

	return match
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

	slog.Debug("Conditional branch tracking: Guild vector filtering by instance allocation completed",
		slog.String("target_instance", target),
		slog.Int("total_guilds", len(cfg.Guilds)),
		slog.Int("matched_guilds", len(out)),
	)

	return out
}

// GuildsForBotInstanceFeature returns the guild subset assigned to the provided bot instance for a specific feature,
// preserving config order.
func (cfg *BotConfig) GuildsForBotInstanceFeature(botInstanceID string, feature string) []GuildConfig {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}

	target := NormalizeBotInstanceID(botInstanceID)

	out := make([]GuildConfig, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if !guild.BelongsToBotInstance(target) {
			continue
		}
		resolvedID, _ := guild.ResolveFeatureBotInstanceID(feature)
		if resolvedID == target {
			out = append(out, guild)
		}
	}

	slog.Debug("Conditional branch tracking: Segmented vector filtering by feature routing completed",
		slog.String("target_instance", target),
		slog.String("feature", feature),
		slog.Int("total_guilds", len(cfg.Guilds)),
		slog.Int("matched_guilds", len(out)),
	)

	return out
}

// ResolveFeatureBotInstanceID returns the designated bot instance for a given feature.
// It explicitly parses FeatureRouting and falls back to "".
// It returns the resolved instance ID and a boolean fallbackFlag
// indicating if the designated bot token was revoked, invalid, or missing, necessitating
// a degradation to the default fallback bot.
func (gc GuildConfig) ResolveFeatureBotInstanceID(feature string) (resolvedID string, fallback bool) {
	// If the guild has gracefully fallen back due to having NO bot tokens,
	// the magic blank instance handles ALL features.
	if len(gc.BotInstanceTokens) == 0 {
		return "", false
	}

	route, exists := gc.FeatureRouting[feature]
	if !exists || route == "" {
		// If unrouted, return a sentinel so it does not accidentally match
		// the magic blank instance ("").
		return "<unrouted>", false
	}

	token, ok := gc.BotInstanceTokens[route]
	if !ok || len(token) == 0 {
		slog.Warn("Mitigated service degradation: Structural feature routing pointed to revoked or non-existent token. Triggering fallback without main flow interruption.",
			slog.String("guild_id", gc.GuildID),
			slog.String("feature", feature),
			slog.String("invalid_route", route),
		)
		return "<unrouted>", true
	}

	slog.Debug("Transient state inspection: Feature route resolved nominally against token dictionary",
		slog.String("guild_id", gc.GuildID),
		slog.String("feature", feature),
		slog.String("resolved_route", route),
	)

	return route, false
}
