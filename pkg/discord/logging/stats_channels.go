package logging

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

const defaultStatsInterval = 30 * time.Minute

func (ms *MonitoringService) updateStatsChannels() {
	if ms.session == nil || ms.configManager == nil {
		return
	}
	cfg := ms.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		return
	}

	for _, gcfg := range cfg.Guilds {
		if !statsEnabled(gcfg.Stats) {
			continue
		}
		interval := statsInterval(gcfg.Stats)
		if !ms.shouldRunStatsUpdate(gcfg.GuildID, interval) {
			continue
		}
		if err := ms.updateStatsForGuild(gcfg); err != nil {
			log.ErrorLoggerRaw().Error("Failed to update stats channels", "guildID", gcfg.GuildID, "err", err)
		}
	}
}

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

func (ms *MonitoringService) shouldRunStatsUpdate(guildID string, interval time.Duration) bool {
	if guildID == "" {
		return false
	}
	if interval <= 0 {
		interval = defaultStatsInterval
	}
	now := time.Now()
	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()
	last, ok := ms.statsLastRun[guildID]
	if ok && now.Sub(last) < interval {
		return false
	}
	ms.statsLastRun[guildID] = now
	return true
}

func (ms *MonitoringService) updateStatsForGuild(gcfg files.GuildConfig) error {
	if gcfg.GuildID == "" {
		return fmt.Errorf("guild id is empty")
	}
	if len(gcfg.Stats.Channels) == 0 {
		return nil
	}

	members, err := ms.fetchAllGuildMembers(gcfg.GuildID)
	if err != nil {
		return fmt.Errorf("fetch guild members: %w", err)
	}

	targets := make([]statsTarget, 0, len(gcfg.Stats.Channels))
	for _, sc := range gcfg.Stats.Channels {
		targets = append(targets, statsTarget{cfg: sc})
	}

	for _, member := range members {
		if member == nil || member.User == nil {
			continue
		}
		isBot := member.User.Bot
		for i := range targets {
			cfg := targets[i].cfg
			if !memberTypeMatches(cfg.MemberType, isBot) {
				continue
			}
			if cfg.RoleID != "" {
				if memberHasRole(member, cfg.RoleID) {
					targets[i].count++
				}
				continue
			}
			targets[i].count++
		}
	}

	for _, t := range targets {
		if err := ms.updateStatsChannelName(gcfg.GuildID, t.cfg, t.count); err != nil {
			log.ErrorLoggerRaw().Error("Failed to update stats channel name", "guildID", gcfg.GuildID, "channelID", t.cfg.ChannelID, "err", err)
		}
	}

	return nil
}

type statsTarget struct {
	cfg   files.StatsChannelConfig
	count int
}

func memberHasRole(member *discordgo.Member, roleID string) bool {
	if member == nil || roleID == "" {
		return false
	}
	for _, rid := range member.Roles {
		if rid == roleID {
			return true
		}
	}
	return false
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

func (ms *MonitoringService) updateStatsChannelName(guildID string, cfg files.StatsChannelConfig, count int) error {
	channelID := strings.TrimSpace(cfg.ChannelID)
	if channelID == "" {
		return nil
	}

	channel, err := ms.resolveChannel(channelID)
	if err != nil {
		return fmt.Errorf("resolve channel: %w", err)
	}
	if channel == nil {
		return fmt.Errorf("channel not found")
	}
	if guildID != "" && channel.GuildID != "" && channel.GuildID != guildID {
		return fmt.Errorf("channel guild mismatch: %s", channel.GuildID)
	}

	label := strings.TrimSpace(cfg.Label)
	if label == "" {
		label = strings.TrimSpace(channel.Name)
	}
	newName := renderStatsChannelName(label, cfg.NameTemplate, count)
	if newName == "" {
		return nil
	}
	if len(newName) > 100 {
		newName = newName[:100]
	}
	if channel.Name == newName {
		return nil
	}

	if _, err := ms.session.ChannelEdit(channelID, &discordgo.ChannelEdit{Name: newName}); err != nil {
		return fmt.Errorf("channel edit: %w", err)
	}
	log.ApplicationLogger().Info("Updated stats channel name", "guildID", guildID, "channelID", channelID, "count", count, "name", newName)
	return nil
}

func (ms *MonitoringService) resolveChannel(channelID string) (*discordgo.Channel, error) {
	if ms.session == nil || channelID == "" {
		return nil, fmt.Errorf("session not available or channel id empty")
	}
	if ms.session.State != nil {
		if ch, err := ms.session.State.Channel(channelID); err == nil && ch != nil {
			return ch, nil
		}
	}
	return ms.session.Channel(channelID)
}

func renderStatsChannelName(label, template string, count int) string {
	label = strings.TrimSpace(label)
	tmpl := strings.TrimSpace(template)
	if tmpl == "" {
		if label == "" {
			return fmt.Sprintf("%d", count)
		}
		return fmt.Sprintf("%s: %d", label, count)
	}
	out := strings.ReplaceAll(tmpl, "{count}", fmt.Sprintf("%d", count))
	out = strings.ReplaceAll(out, "{label}", label)
	return strings.TrimSpace(out)
}
