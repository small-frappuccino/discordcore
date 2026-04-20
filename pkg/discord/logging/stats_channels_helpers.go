package logging

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	defaultStatsInterval          = 30 * time.Minute
	defaultStatsReconcileInterval = 6 * time.Hour
	maxStatsReconcileInterval     = 24 * time.Hour
	statsSeedMetadataPrefix       = "stats_channels.seeded:"
)

func statsEnabled(cfg files.StatsConfig) bool {
	if !cfg.Enabled {
		return false
	}
	return len(cfg.Channels) > 0
}

func statsInterval(cfg files.StatsConfig) time.Duration {
	if cfg.UpdateIntervalMins <= 0 {
		return defaultStatsInterval
	}
	return time.Duration(cfg.UpdateIntervalMins) * time.Minute
}

func statsReconcileInterval(cfg files.StatsConfig) time.Duration {
	interval := statsInterval(cfg) * 12
	if interval < defaultStatsReconcileInterval {
		return defaultStatsReconcileInterval
	}
	if interval > maxStatsReconcileInterval {
		return maxStatsReconcileInterval
	}
	return interval
}

func statsStoreFreshnessLimit(cfg files.StatsConfig) time.Duration {
	limit := statsInterval(cfg)
	minimum := 2 * heartbeatInterval
	if limit < minimum {
		return minimum
	}
	return limit
}

func statsSeedMetadataKey(guildID string) string {
	return statsSeedMetadataPrefix + strings.TrimSpace(guildID)
}

func statsRequiresBotClassification(channels []files.StatsChannelConfig) bool {
	for _, channel := range channels {
		switch normalizeMemberType(channel.MemberType) {
		case "bots", "humans":
			return true
		}
	}
	return false
}

func filterTrackedRoles(roles []string, trackedRoles map[string]struct{}) []string {
	if len(roles) == 0 || len(trackedRoles) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(roles))
	filtered := make([]string, 0, len(roles))
	for _, roleID := range roles {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		if _, ok := trackedRoles[roleID]; !ok {
			continue
		}
		if _, duplicate := seen[roleID]; duplicate {
			continue
		}
		seen[roleID] = struct{}{}
		filtered = append(filtered, roleID)
	}
	sort.Strings(filtered)
	return filtered
}

func memberTypeMatches(memberType string, isBot bool) bool {
	switch normalizeMemberType(memberType) {
	case "bots":
		return isBot
	case "humans":
		return !isBot
	default:
		return true
	}
}

func normalizeMemberType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "bots", "bot":
		return "bots"
	case "humans", "human":
		return "humans"
	default:
		return "all"
	}
}

func (ms *MonitoringService) resolveChannel(ctx context.Context, channelID string) (*discordgo.Channel, error) {
	if ms.session == nil || channelID == "" {
		return nil, fmt.Errorf("session not available or channel id empty")
	}
	if ms.session.State != nil {
		if ch, err := ms.session.State.Channel(channelID); err == nil && ch != nil {
			return ch, nil
		}
	}
	return monitoringRunWithTimeout(ctx, monitoringDependencyTimeout, func() (*discordgo.Channel, error) {
		return ms.session.Channel(channelID)
	})
}
