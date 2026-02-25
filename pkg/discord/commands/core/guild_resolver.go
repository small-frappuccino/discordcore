package core

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

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
