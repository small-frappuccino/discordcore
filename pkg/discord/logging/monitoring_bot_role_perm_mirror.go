package logging

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

func (ms *MonitoringService) botRolePermSnapshotKey(guildID, roleID string) string {
	if guildID == "" || roleID == "" {
		return ""
	}
	return persistentCacheKeyPrefixBotRolePermSnapshot + guildID + ":" + roleID
}

func (ms *MonitoringService) botPermMirrorEnabled(guildID string) bool {
	if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
		cfg := scopedCfg
		rc := cfg.ResolveRuntimeConfig(guildID)
		features := cfg.ResolveFeatures(guildID)
		if !features.Safety.BotRolePermMirror {
			return false
		}
		return !rc.DisableBotRolePermMirror
	}
	return true
}

func (ms *MonitoringService) botPermMirrorActorRoleID(guildID string) string {
	if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
		rc := scopedCfg.ResolveRuntimeConfig(guildID)
		v := strings.TrimSpace(rc.BotRolePermMirrorActorRoleID)
		if v != "" {
			return v
		}
	}
	return defaultBotPermMirrorActorRoleID
}

func (ms *MonitoringService) findGuildRole(guildID string, match func(*discordgo.Role) bool) (*discordgo.Role, bool) {
	if guildID == "" || ms.session == nil || match == nil {
		return nil, false
	}
	roles, err := ms.session.GuildRoles(guildID)
	if err != nil {
		return nil, false
	}
	for _, role := range roles {
		if role == nil {
			continue
		}
		if match(role) {
			return role, true
		}
	}
	return nil, false
}

func (ms *MonitoringService) isBotManagedRole(guildID, roleID string) bool {
	if roleID == "" {
		return false
	}
	role, ok := ms.findGuildRole(guildID, func(r *discordgo.Role) bool {
		return r.ID == roleID
	})
	return ok && role.Managed
}

func (ms *MonitoringService) getRoleByID(guildID, roleID string) (*discordgo.Role, bool) {
	if roleID == "" {
		return nil, false
	}
	return ms.findGuildRole(guildID, func(r *discordgo.Role) bool {
		return r.ID == roleID
	})
}

func (ms *MonitoringService) findBotManagedRole(guildID string) (*discordgo.Role, bool) {
	return ms.findGuildRole(guildID, func(r *discordgo.Role) bool {
		return r.Managed
	})
}

func (ms *MonitoringService) saveBotRolePermSnapshot(guildID, roleID string, prevPerm int64, actorUserID string) {
	if ms.store == nil || guildID == "" || roleID == "" {
		return
	}
	snap := botRolePermSnapshot{
		GuildID:         guildID,
		RoleID:          roleID,
		PrevPermissions: prevPerm,
		SavedAt:         time.Now().UTC(),
		SavedByUserID:   actorUserID,
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return
	}
	expiresAt := time.Now().UTC().Add(365 * 24 * time.Hour)
	_ = ms.store.UpsertCacheEntry(ms.botRolePermSnapshotKey(guildID, roleID), persistentCacheTypeBotRolePermSnapshot, string(b), expiresAt)
}

func (ms *MonitoringService) getBotRolePermSnapshot(guildID, roleID string) (*botRolePermSnapshot, bool) {
	if ms.store == nil {
		return nil, false
	}
	key := ms.botRolePermSnapshotKey(guildID, roleID)
	if key == "" {
		return nil, false
	}
	tp, data, _, ok, err := ms.store.GetCacheEntry(key)
	if err != nil || !ok || tp != persistentCacheTypeBotRolePermSnapshot || strings.TrimSpace(data) == "" {
		return nil, false
	}
	var snap botRolePermSnapshot
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		return nil, false
	}
	if snap.GuildID == "" || snap.RoleID == "" {
		return nil, false
	}
	return &snap, true
}

func (ms *MonitoringService) maybeRestoreBotRolePermissions(guildID, roleID string, newPerm int64) {
	snap, ok := ms.getBotRolePermSnapshot(guildID, roleID)
	if !ok || snap == nil {
		return
	}
	if newPerm == snap.PrevPermissions {
		return
	}

	if !ms.isBotManagedRole(guildID, roleID) {
		return
	}
	if newPerm > snap.PrevPermissions {
		return
	}

	if ms.session == nil {
		return
	}
	perm := snap.PrevPermissions
	if _, err := ms.session.GuildRoleEdit(guildID, roleID, &discordgo.RoleParams{
		Permissions: &perm,
	}); err != nil {
		log.ErrorLoggerRaw().Error(
			"Failed to restore bot managed role permissions from snapshot",
			"guildID", guildID,
			"roleID", roleID,
			"targetPermissions", perm,
			"err", err,
		)
	}
}

func (ms *MonitoringService) handleRoleCreateForBotPermMirroring(s *discordgo.Session, e *discordgo.GuildRoleCreate) {
	if e == nil || e.Role == nil || e.GuildID == "" {
		return
	}
	if !ms.handlesGuild(e.GuildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_role_create",
		slog.String("guildID", e.GuildID),
		slog.String("roleID", e.Role.ID),
		slog.Bool("managed", e.Role.Managed),
	)
	defer done()

	if !ms.botPermMirrorEnabled(e.GuildID) {
		return
	}
	if !e.Role.Managed {
		return
	}
	ms.maybeRestoreBotRolePermissions(e.GuildID, e.Role.ID, e.Role.Permissions)
}

func (ms *MonitoringService) handleRoleUpdateForBotPermMirroring(s *discordgo.Session, e *discordgo.GuildRoleUpdate) {
	if e == nil || e.Role == nil || e.GuildID == "" {
		return
	}
	if !ms.handlesGuild(e.GuildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_role_update",
		slog.String("guildID", e.GuildID),
		slog.String("roleID", e.Role.ID),
		slog.Bool("managed", e.Role.Managed),
	)
	defer done()

	if !ms.botPermMirrorEnabled(e.GuildID) {
		return
	}
	if !e.Role.Managed {
		return
	}

	actionType := int(discordgo.AuditLogActionRoleUpdate)
	audit, err := ms.session.GuildAuditLog(e.GuildID, "", "", actionType, 10)
	atomic.AddUint64(&ms.apiAuditLogCalls, 1)
	if err == nil && audit != nil {
		for _, entry := range audit.AuditLogEntries {
			if entry == nil || entry.ActionType == nil {
				continue
			}
			if *entry.ActionType != discordgo.AuditLogActionRoleUpdate {
				continue
			}
			if entry.TargetID != e.Role.ID {
				continue
			}

			actorID := entry.UserID
			if strings.TrimSpace(actorID) == "" {
				break
			}

			actor, err := ms.getGuildMember(e.GuildID, actorID)
			if err != nil || actor == nil {
				break
			}
			hasActorRole := false
			requiredRoleID := ms.botPermMirrorActorRoleID(e.GuildID)
			if requiredRoleID != "" {
				for _, rid := range actor.Roles {
					if rid == requiredRoleID {
						hasActorRole = true
						break
					}
				}
			}
			if !hasActorRole {
				break
			}

			var oldPerm *int64
			for _, ch := range entry.Changes {
				if ch == nil || ch.Key == nil {
					continue
				}
				if *ch.Key != "permissions" {
					continue
				}
				switch v := ch.OldValue.(type) {
				case string:
					if p, err := strconv.ParseInt(v, 10, 64); err == nil {
						oldPerm = &p
					}
				case float64:
					p := int64(v)
					oldPerm = &p
				case int64:
					p := v
					oldPerm = &p
				case int:
					p := int64(v)
					oldPerm = &p
				}
			}
			if oldPerm != nil {
				ms.saveBotRolePermSnapshot(e.GuildID, e.Role.ID, *oldPerm, actorID)
			}
			break
		}
	}

	ms.maybeRestoreBotRolePermissions(e.GuildID, e.Role.ID, e.Role.Permissions)
}

// Helper methods for cached API calls

// getGuildMember retrieves a member using unified cache -> state -> API fallback
func (ms *MonitoringService) getGuildMember(guildID, userID string) (*discordgo.Member, error) {
	return ms.getGuildMemberContext(context.Background(), guildID, userID)
}

func (ms *MonitoringService) getGuildMemberContext(ctx context.Context, guildID, userID string) (*discordgo.Member, error) {
	if ms.unifiedCache != nil {
		if member, ok := ms.unifiedCache.GetMember(guildID, userID); ok {
			return member, nil
		}
	}

	if ms.session != nil && ms.session.State != nil {
		if member, err := ms.session.State.Member(guildID, userID); err == nil && member != nil {
			atomic.AddUint64(&ms.cacheStateMemberHits, 1)
			if ms.unifiedCache != nil {
				ms.unifiedCache.SetMember(guildID, userID, member)
			}
			return member, nil
		}
	}

	atomic.AddUint64(&ms.apiGuildMemberCalls, 1)
	member, err := monitoringRunWithTimeout(ctx, monitoringDependencyTimeout, func() (*discordgo.Member, error) {
		return ms.session.GuildMember(guildID, userID)
	})
	if err != nil {
		return nil, err
	}

	if ms.unifiedCache != nil {
		ms.unifiedCache.SetMember(guildID, userID, member)
	}
	return member, nil
}

// getGuild retrieves a guild using unified cache -> state -> API fallback
func (ms *MonitoringService) getGuild(guildID string) (*discordgo.Guild, error) {
	return ms.getGuildContext(context.Background(), guildID)
}

func (ms *MonitoringService) getGuildContext(ctx context.Context, guildID string) (*discordgo.Guild, error) {
	if ms.unifiedCache != nil {
		if guild, ok := ms.unifiedCache.GetGuild(guildID); ok {
			return guild, nil
		}
	}

	if ms.session != nil && ms.session.State != nil {
		if guild, err := ms.session.State.Guild(guildID); err == nil && guild != nil {
			if ms.unifiedCache != nil {
				ms.unifiedCache.SetGuild(guildID, guild)
			}
			return guild, nil
		}
	}

	guild, err := monitoringRunWithTimeout(ctx, monitoringDependencyTimeout, func() (*discordgo.Guild, error) {
		return ms.session.Guild(guildID)
	})
	if err != nil {
		return nil, err
	}

	if ms.unifiedCache != nil {
		ms.unifiedCache.SetGuild(guildID, guild)
	}
	return guild, nil
}
