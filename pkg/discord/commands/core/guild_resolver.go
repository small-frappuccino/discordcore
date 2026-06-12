package core

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

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
	ownerID, ownerFound, err := pc.ResolveOwnerID(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild owner",
			"operation", "commands.permission.has_permission.resolve_owner",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
	}
	if err == nil && ownerFound && ownerID == userID {
		return true
	}

	member, memberFound, err := pc.ResolveMember(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild member for admin access",
			"operation", "commands.permission.has_permission.resolve_member_admin",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	if !memberFound || member == nil {
		return false
	}

	roles, err := pc.ResolveRoles(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild roles for admin access",
			"operation", "commands.permission.has_permission.resolve_roles_admin",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	permissions := memberPermissionBits(member, roles, guildID)
	if permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		return true
	}
	if permissions&discordgo.PermissionManageGuild == discordgo.PermissionManageGuild {
		return true
	}
	return false
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

// IsOwner checks whether the user is the server owner
func (pc *PermissionChecker) IsOwner(guildID, userID string) bool {
	if guildID == "" {
		return false
	}
	ownerID, ok, err := pc.ResolveOwnerID(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild owner for owner check",
			"operation", "commands.permission.is_owner.resolve_owner",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	if !ok {
		return false
	}
	return ownerID == userID
}

// ResolveOwnerID resolves a guild owner ID using cache -> state -> store -> REST.
// It returns (ownerID, true, nil) when found, ("", false, nil) when not found,
// and a non-nil error when resolution fails in a terminal path.
func (pc *PermissionChecker) ResolveOwnerID(guildID string) (string, bool, error) {
	if pc == nil || guildID == "" {
		return "", false, nil
	}

	if pc.cache != nil {
		if g, ok := pc.cache.GetGuild(guildID); ok && g != nil && g.OwnerID != "" {
			return g.OwnerID, true, nil
		}
	}

	if pc.session != nil && pc.session.State != nil {
		if g, _ := pc.session.State.Guild(guildID); g != nil && g.OwnerID != "" {
			if pc.cache != nil {
				pc.cache.SetGuild(guildID, g)
			}
			if pc.store != nil {
				if err := pc.store.SetGuildOwnerID(guildID, g.OwnerID); err != nil {
					log.ErrorLoggerRaw().Error(
						"Guild resolver failed to persist owner from state",
						"operation", "commands.guild_resolver.resolve_owner.store_write",
						"guildID", guildID,
						"ownerID", g.OwnerID,
						"source", "state",
						"err", err,
					)
				}
			}
			return g.OwnerID, true, nil
		}
	}

	if pc.store != nil {
		ownerID, ok, err := pc.store.GetGuildOwnerID(guildID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Guild resolver failed to read owner from store",
				"operation", "commands.guild_resolver.resolve_owner.store_read",
				"guildID", guildID,
				"err", err,
			)
		} else if ok && ownerID != "" {
			return ownerID, true, nil
		}
	}

	if pc.session == nil {
		return "", false, fmt.Errorf("resolve owner id for guild %s: session not ready", guildID)
	}

	guild, err := pc.session.Guild(guildID)
	if err != nil {
		if isNotFoundRESTError(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("resolve owner id via rest for guild %s: %w", guildID, err)
	}
	if guild == nil || guild.OwnerID == "" {
		return "", false, nil
	}

	if pc.cache != nil {
		pc.cache.SetGuild(guildID, guild)
	}
	if pc.store != nil {
		if err := pc.store.SetGuildOwnerID(guildID, guild.OwnerID); err != nil {
			log.ErrorLoggerRaw().Error(
				"Guild resolver failed to persist owner from rest",
				"operation", "commands.guild_resolver.resolve_owner.store_write",
				"guildID", guildID,
				"ownerID", guild.OwnerID,
				"source", "rest",
				"err", err,
			)
		}
	}

	return guild.OwnerID, true, nil
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
			return member, true, nil
		}
	}

	if pc.session != nil && pc.session.State != nil {
		if member, _ := pc.session.State.Member(guildID, userID); member != nil {
			if pc.cache != nil {
				pc.cache.SetMember(guildID, userID, member)
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
		pc.cache.SetMember(guildID, userID, member)
	}
	return member, true, nil
}

// ResolveRoles resolves guild roles using cache -> state -> REST.
func (pc *PermissionChecker) ResolveRoles(guildID string) ([]*discordgo.Role, error) {
	if pc == nil || guildID == "" {
		return nil, nil
	}

	if pc.cache != nil {
		if roles, ok := pc.cache.GetRoles(guildID); ok && len(roles) > 0 {
			return roles, nil
		}
	}

	if pc.session != nil && pc.session.State != nil {
		if g, _ := pc.session.State.Guild(guildID); g != nil && len(g.Roles) > 0 {
			if pc.cache != nil {
				pc.cache.SetRoles(guildID, g.Roles)
				pc.cache.SetGuild(guildID, g)
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
		pc.cache.SetRoles(guildID, roles)
	}
	return roles, nil
}

func isNotFoundRESTError(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr == nil || restErr.Response == nil {
		return false
	}
	return restErr.Response.StatusCode == http.StatusNotFound
}
