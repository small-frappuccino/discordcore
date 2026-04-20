package logging

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func statsCountForChannel(snapshot statsGuildSnapshot, cfg files.StatsChannelConfig) int {
	roleID := strings.TrimSpace(cfg.RoleID)
	if roleID == "" {
		return snapshot.totals.total(cfg.MemberType)
	}
	return snapshot.roleTotals[roleID].total(cfg.MemberType)
}

func renderStatsChannelName(label, template string, count int) string {
	label = strings.TrimSpace(label)
	tmpl := strings.TrimSpace(template)
	if tmpl == "" {
		if label == "" {
			return fmt.Sprintf("☆  ☆ : %d", count)
		}
		return fmt.Sprintf("☆ %s ☆ : %d", strings.ToLower(label), count)
	}
	out := strings.ReplaceAll(tmpl, "{count}", fmt.Sprintf("%d", count))
	out = strings.ReplaceAll(out, "{label}", label)
	return strings.TrimSpace(out)
}

func statsSnapshotFromMember(member *discordgo.Member, trackedRoles map[string]struct{}) (string, statsMemberSnapshot, bool) {
	if member == nil || member.User == nil {
		return "", statsMemberSnapshot{}, false
	}
	userID := strings.TrimSpace(member.User.ID)
	if userID == "" {
		return "", statsMemberSnapshot{}, false
	}
	return userID, statsMemberSnapshot{
		isBot:        member.User.Bot,
		trackedRoles: filterTrackedRoles(member.Roles, trackedRoles),
	}, true
}

func statsSnapshotFromStoredState(member storage.GuildMemberCurrentState, trackedRoles map[string]struct{}) (string, statsMemberSnapshot, bool) {
	userID := strings.TrimSpace(member.UserID)
	if userID == "" || !member.Active {
		return "", statsMemberSnapshot{}, false
	}
	return userID, statsMemberSnapshot{
		isBot:        member.IsBot,
		trackedRoles: filterTrackedRoles(member.Roles, trackedRoles),
	}, true
}

func statsTrackedRoles(channels []files.StatsChannelConfig) (map[string]struct{}, string) {
	roleIDs := make([]string, 0, len(channels))
	seen := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		roleID := strings.TrimSpace(channel.RoleID)
		if roleID == "" {
			continue
		}
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		roleIDs = append(roleIDs, roleID)
	}
	sort.Strings(roleIDs)

	tracked := make(map[string]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		tracked[roleID] = struct{}{}
	}
	return tracked, strings.Join(roleIDs, ",")
}
