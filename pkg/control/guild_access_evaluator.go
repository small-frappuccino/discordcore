package control

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type guildAccessEvaluator struct {
	configManager   *files.ConfigManager
	discordSessions discordSessionResolver
}

func newGuildAccessEvaluator(
	configManager *files.ConfigManager,
	discordSessions discordSessionResolver,
) *guildAccessEvaluator {
	return &guildAccessEvaluator{
		configManager:   configManager,
		discordSessions: discordSessions,
	}
}

func (evaluator *guildAccessEvaluator) ResolveGuildAccessLevel(
	guild discordOAuthGuild,
	userID string,
) (guildAccessLevel, bool) {
	if isGuildManageableByUser(guild) {
		return guildAccessLevelWrite, true
	}
	if evaluator == nil || evaluator.configManager == nil {
		return "", false
	}

	guildID := strings.TrimSpace(guild.ID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return "", false
	}

	guildConfig := evaluator.configManager.GuildConfig(guildID)
	if guildConfig == nil {
		return "", false
	}
	if len(guildConfig.Roles.DashboardRead) == 0 && len(guildConfig.Roles.DashboardWrite) == 0 {
		return "", false
	}
	if evaluator.discordSessions == nil {
		return "", false
	}

	session, err := evaluator.discordSessions(guildID)
	if err != nil || session == nil {
		return "", false
	}

	member, err := resolveGuildMemberFromDiscordSession(session, guildID, userID)
	if err != nil || member == nil {
		return "", false
	}

	memberRoleSet := make(map[string]struct{}, len(member.Roles))
	for _, roleID := range member.Roles {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		memberRoleSet[roleID] = struct{}{}
	}

	if matchesAnyRole(memberRoleSet, guildConfig.Roles.DashboardWrite) {
		return guildAccessLevelWrite, true
	}
	if matchesAnyRole(memberRoleSet, guildConfig.Roles.DashboardRead) {
		return guildAccessLevelRead, true
	}

	return "", false
}

func matchesAnyRole(memberRoleSet map[string]struct{}, roleIDs []string) bool {
	if len(memberRoleSet) == 0 || len(roleIDs) == 0 {
		return false
	}
	for _, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		if _, ok := memberRoleSet[roleID]; ok {
			return true
		}
	}
	return false
}

func isGuildManageableByUser(guild discordOAuthGuild) bool {
	if guild.Owner {
		return true
	}
	if guild.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		return true
	}
	if guild.Permissions&discordgo.PermissionManageGuild == discordgo.PermissionManageGuild {
		return true
	}
	return false
}
