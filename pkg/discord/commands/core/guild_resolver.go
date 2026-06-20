package core

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	arikawadiscord "github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

// PermissionChecker manages user permission checks
type PermissionChecker struct {
	session *discordgo.Session
	config  *files.ConfigManager
	store   *storage.Store
	cache   *cache.UnifiedCache
}

// NewPermissionChecker news permission checker.
func NewPermissionChecker(session *discordgo.Session, config *files.ConfigManager) *PermissionChecker {
	return &PermissionChecker{session: session, config: config}
}

// SetStore sets store.
func (pc *PermissionChecker) SetStore(store *storage.Store) {
	pc.store = store
}

// SetCache sets cache.
func (pc *PermissionChecker) SetCache(unifiedCache *cache.UnifiedCache) {
	pc.cache = unifiedCache
}

// HasPermission checks whether the user has permission to use commands
func (pc *PermissionChecker) HasPermission(guildID, userID string) bool {
	if guildID == "" {
		return false
	}
	if pc.hasAdministrativeAccess(guildID, userID) {
		return true
	}
	guildConfig := pc.config.GuildConfig(guildID)
	if guildConfig == nil || len(guildConfig.Roles.Allowed) == 0 {
		return false
	}

	member, memberFound, err := pc.ResolveMember(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild member",
			"operation", "commands.permission.has_permission.resolve_member",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	if !memberFound || member == nil {
		return false
	}

	for _, userRole := range member.Roles {
		if slices.Contains(guildConfig.Roles.Allowed, userRole) {
			return true
		}
	}
	return false
}

func (pc *PermissionChecker) hasAdministrativeAccess(guildID, userID string) bool {
	member, memberFound, err := pc.ResolveMember(guildID, userID)
	if err != nil || !memberFound {
		fmt.Printf("DEBUG hasAdministrativeAccess: member not found for %s/%s err=%v\n", guildID, userID, err)
		return false
	}

	for _, roleID := range member.Roles {
		if roleID == guildID {
			continue // skip @everyone
		}
		hasAdmin, err := pc.roleHasAdmin(guildID, roleID)
		if err == nil && hasAdmin {
			fmt.Printf("DEBUG hasAdministrativeAccess: role %s has admin for %s/%s\n", roleID, guildID, userID)
			return true
		}
		fmt.Printf("DEBUG hasAdministrativeAccess: role %s NO admin for %s/%s (err=%v hasAdmin=%v)\n", roleID, guildID, userID, err, hasAdmin)
	}
	fmt.Printf("DEBUG hasAdministrativeAccess: no admin roles for %s/%s\n", guildID, userID)
	return false
}

func (pc *PermissionChecker) roleHasAdmin(guildID, roleID string) (bool, error) {
	roles, err := pc.ResolveRoles(guildID)
	if err == nil {
		for _, r := range roles {
			if r.ID == roleID {
				return r.Permissions&discordgo.PermissionAdministrator != 0 || r.Permissions&discordgo.PermissionManageGuild != 0, nil
			}
		}
	}
	return false, nil
}

func memberPermissionBits(member *discordgo.Member, roles []*discordgo.Role, guildID string) int64 {
	if member == nil {
		return 0
	}
	rolesByID := make(map[string]*discordgo.Role, len(roles))
	for _, role := range roles {
		if role == nil {
			continue
		}
		rolesByID[role.ID] = role
	}

	var permissions int64
	if role, ok := rolesByID[guildID]; ok && role != nil {
		permissions |= role.Permissions
	}
	for _, roleID := range member.Roles {
		if role, ok := rolesByID[roleID]; ok && role != nil {
			permissions |= role.Permissions
		}
	}
	return permissions
}

// HasRole checks whether the user has a specific role
func (pc *PermissionChecker) HasRole(guildID, userID, roleID string) bool {
	member, ok, err := pc.ResolveMember(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild member for role check",
			"operation", "commands.permission.has_role.resolve_member",
			"guildID", guildID,
			"userID", userID,
			"roleID", roleID,
			"err", err,
		)
		return false
	}
	if !ok || member == nil {
		return false
	}
	return slices.Contains(member.Roles, roleID)
}

// ResolveMember resolves a guild member using cache -> state -> REST.
// It returns (member, true, nil) when found, (nil, false, nil) when not found,
// and a non-nil error when resolution fails in a terminal path.
func (pc *PermissionChecker) ResolveMember(guildID, userID string) (*discordgo.Member, bool, error) {
	if pc == nil || guildID == "" || userID == "" {
		return nil, false, nil
	}

	if pc.cache != nil {
		if member, ok := pc.cache.GetMember(guildID, userID); ok && member != nil {
			return toDiscordgoMember(member), true, nil
		}
	}

	if pc.session != nil && pc.session.State != nil {
		if member, _ := pc.session.State.Member(guildID, userID); member != nil {
			if pc.cache != nil {
				pc.cache.SetMember(guildID, userID, toArikawaMember(member))
			}
			return member, true, nil
		}
	}

	if pc.session == nil {
		return nil, false, fmt.Errorf("resolve member for guild %s user %s: session not ready", guildID, userID)
	}

	member, err := pc.session.GuildMember(guildID, userID)
	if err != nil {
		if isNotFoundRESTError(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("resolve member via rest for guild %s user %s: %w", guildID, userID, err)
	}
	if member == nil {
		return nil, false, nil
	}

	if pc.cache != nil {
		pc.cache.SetMember(guildID, userID, toArikawaMember(member))
	}
	return member, true, nil
}

// ResolveRoles resolves guild roles using cache -> state -> REST.
func (pc *PermissionChecker) ResolveRoles(guildID string) ([]*discordgo.Role, error) {
	if pc == nil || guildID == "" {
		return nil, nil
	}

	if pc.cache != nil {
		if roles, ok := pc.cache.GetRoles(guildID); ok && roles != nil && len(*roles) > 0 {
			return toDiscordgoRoles(roles), nil
		}
	}

	if pc.session != nil && pc.session.State != nil {
		if g, _ := pc.session.State.Guild(guildID); g != nil && len(g.Roles) > 0 {
			if pc.cache != nil {
				pc.cache.SetRoles(guildID, toArikawaRoles(g.Roles))
				pc.cache.SetGuild(guildID, toArikawaGuild(g))
			}
			return g.Roles, nil
		}
	}

	if pc.session == nil {
		return nil, fmt.Errorf("resolve roles for guild %s: session not ready", guildID)
	}

	roles, err := pc.session.GuildRoles(guildID)
	if err != nil {
		if isNotFoundRESTError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve roles via rest for guild %s: %w", guildID, err)
	}
	if pc.cache != nil && len(roles) > 0 {
		pc.cache.SetRoles(guildID, toArikawaRoles(roles))
	}
	return roles, nil
}

func toArikawaMember(m *discordgo.Member) *arikawadiscord.Member {
	if m == nil {
		return nil
	}
	roles := make([]arikawadiscord.RoleID, len(m.Roles))
	for i, r := range m.Roles {
		rid, _ := arikawadiscord.ParseSnowflake(r)
		roles[i] = arikawadiscord.RoleID(rid)
	}
	var uid arikawadiscord.Snowflake
	if m.User != nil {
		uid, _ = arikawadiscord.ParseSnowflake(m.User.ID)
	}
	return &arikawadiscord.Member{
		User:    arikawadiscord.User{ID: arikawadiscord.UserID(uid)},
		RoleIDs: roles,
	}
}

func toDiscordgoMember(m *arikawadiscord.Member) *discordgo.Member {
	if m == nil {
		return nil
	}
	roles := make([]string, len(m.RoleIDs))
	for i, r := range m.RoleIDs {
		roles[i] = r.String()
	}
	return &discordgo.Member{
		User:  &discordgo.User{ID: m.User.ID.String()},
		Roles: roles,
	}
}

func toArikawaRoles(roles []*discordgo.Role) *[]arikawadiscord.Role {
	if roles == nil {
		return nil
	}
	res := make([]arikawadiscord.Role, len(roles))
	for i, r := range roles {
		rid, _ := arikawadiscord.ParseSnowflake(r.ID)
		res[i] = arikawadiscord.Role{
			ID:          arikawadiscord.RoleID(rid),
			Permissions: arikawadiscord.Permissions(r.Permissions),
		}
	}
	return &res
}

func toDiscordgoRoles(roles *[]arikawadiscord.Role) []*discordgo.Role {
	if roles == nil {
		return nil
	}
	res := make([]*discordgo.Role, len(*roles))
	for i, r := range *roles {
		res[i] = &discordgo.Role{
			ID:          r.ID.String(),
			Permissions: int64(r.Permissions),
		}
	}
	return res
}

func toArikawaGuild(g *discordgo.Guild) *arikawadiscord.Guild {
	if g == nil {
		return nil
	}
	gid, _ := arikawadiscord.ParseSnowflake(g.ID)
	return &arikawadiscord.Guild{
		ID: arikawadiscord.GuildID(gid),
	}
}

func isNotFoundRESTError(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr == nil || restErr.Response == nil {
		return false
	}
	return restErr.Response.StatusCode == http.StatusNotFound
}
