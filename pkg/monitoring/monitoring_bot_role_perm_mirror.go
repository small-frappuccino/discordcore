package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

const (
	// persistent_cache types
	persistentCacheTypeBotRolePermSnapshot = "bot_role_perm_snapshot"

	// persistent_cache key prefix
	persistentCacheKeyPrefixBotRolePermSnapshot = "bot_role_perm_snapshot:"
)

type botRolePermSnapshot struct {
	GuildID         string    `json:"guild_id"`
	RoleID          string    `json:"role_id"`
	PrevPermissions int64     `json:"prev_permissions"`
	SavedAt         time.Time `json:"saved_at"`
	SavedByUserID   string    `json:"saved_by_user_id"`
}

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

// botPermMirrorActorRoleID returns the configured actor role ID for the guild
// or "" when no role is configured. An empty result intentionally disables the
// mirror trigger at the call site, matching features.Safety.BotRolePermMirror=false
// as the disable path.
func (ms *MonitoringService) botPermMirrorActorRoleID(guildID string) string {
	scopedCfg := ms.scopedConfig()
	if scopedCfg == nil {
		return ""
	}
	rc := scopedCfg.ResolveRuntimeConfig(guildID)
	return strings.TrimSpace(rc.BotRolePermMirrorActorRoleID)
}

func (ms *MonitoringService) findGuildRole(guildID string, match func(*Role) bool) (*Role, bool) {
	if guildID == "" || ms.dataProvider == nil || match == nil {
		return nil, false
	}
	ctx := context.Background()
	if runCtx := ms.currentRunCtx(); runCtx != nil {
		ctx = runCtx
	}
	roles, err := ms.dataProvider.GetGuildRoles(ctx, guildID)
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
	role, ok := ms.findGuildRole(guildID, func(r *Role) bool {
		return r.ID == roleID
	})
	return ok && role.Managed
}

func (ms *MonitoringService) getRoleByID(guildID, roleID string) (*Role, bool) {
	if roleID == "" {
		return nil, false
	}
	return ms.findGuildRole(guildID, func(r *Role) bool {
		return r.ID == roleID
	})
}

func (ms *MonitoringService) findBotManagedRole(guildID string) (*Role, bool) {
	return ms.findGuildRole(guildID, func(r *Role) bool {
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

func (ms *MonitoringService) checkAndMirrorUserRolePermissions(guildID, roleID string) {
	requiredRoleID := ms.botPermMirrorActorRoleID(guildID)
	if requiredRoleID != roleID {
		return
	}

	role, ok := ms.getRoleByID(guildID, roleID)
	if !ok || role == nil {
		return
	}

	botRole, ok := ms.findBotManagedRole(guildID)
	if !ok || botRole == nil {
		return
	}

	if botRole.Permissions == role.Permissions {
		return
	}

	newPerm := role.Permissions
	if ms.dataProvider == nil {
		return
	}
	ctx := context.Background()
	if runCtx := ms.currentRunCtx(); runCtx != nil {
		ctx = runCtx
	}
	if err := ms.dataProvider.EditGuildRolePermissions(ctx, guildID, botRole.ID, newPerm); err != nil {
		log.ErrorLoggerRaw().Error(
			"Failed to mirror role permissions to bot role",
			"guildID", guildID,
			"targetRoleID", botRole.ID,
			"sourceRoleID", roleID,
			"targetPermissions", newPerm,
			"err", err,
		)
	} else {
		log.ApplicationLogger().Info(
			"Mirrored role permissions to bot role",
			"guildID", guildID,
			"targetRoleID", botRole.ID,
			"sourceRoleID", roleID,
			"permissions", newPerm,
		)
	}
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

	if ms.dataProvider == nil {
		return
	}
	ctx := context.Background()
	if runCtx := ms.currentRunCtx(); runCtx != nil {
		ctx = runCtx
	}
	if err := ms.dataProvider.EditGuildRolePermissions(ctx, guildID, roleID, snap.PrevPermissions); err != nil {
		log.ErrorLoggerRaw().Error(
			"Failed to restore bot managed role permissions from snapshot",
			"guildID", guildID,
			"roleID", roleID,
			"targetPermissions", snap.PrevPermissions,
			"err", err,
		)
	}
}

func (ms *MonitoringService) HandleRoleCreateForBotPermMirroring(guildID string, roleID string, managed bool) {
	if roleID == "" || guildID == "" {
		return
	}
	if !ms.handlesGuild(guildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_role_create",
		slog.String("guildID", guildID),
		slog.String("roleID", roleID),
		slog.Bool("managed", managed),
	)
	defer done()

	if !ms.botPermMirrorEnabled(guildID) {
		return
	}
	if !managed {
		return
	}
	ms.maybeRestoreBotRolePermissions(guildID, roleID, 0)
}

func (ms *MonitoringService) HandleRoleUpdateForBotPermMirroring(guildID string, roleID string, managed bool) {
	if roleID == "" || guildID == "" {
		return
	}
	if !ms.handlesGuild(guildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_role_update.mirror",
		slog.String("guildID", guildID),
		slog.String("roleID", roleID),
		slog.Bool("managed", managed),
	)
	defer done()

	if !ms.botPermMirrorEnabled(guildID) {
		return
	}

	if managed {
		ms.maybeRestoreBotRolePermissions(guildID, roleID, 0)
	} else {
		ms.checkAndMirrorUserRolePermissions(guildID, roleID)
	}
}

// Helper methods for cached API calls

// getGuildMember retrieves a member using the DataProvider
func (ms *MonitoringService) getGuildMember(guildID, userID string) (*Member, error) {
	return ms.getGuildMemberContext(context.Background(), guildID, userID)
}

func (ms *MonitoringService) getGuildMemberContext(ctx context.Context, guildID, userID string) (*Member, error) {
	if ms.dataProvider == nil {
		return nil, fmt.Errorf("data provider is nil")
	}
	return ms.dataProvider.GetMember(ctx, guildID, userID)
}

// getGuild is not fully needed if we decouple, but stubbed out to avoid breaking other files yet.
// If the domain needs guild properties, they should be fetched via data provider.
func (ms *MonitoringService) getGuild(guildID string) (any, error) {
	return ms.getGuildContext(context.Background(), guildID)
}

func (ms *MonitoringService) getGuildContext(ctx context.Context, guildID string) (any, error) {
	// If domain needs guild, we should add GetGuild to DataProvider.
	// For now, return nil as it's likely unused or we'll find out in compilation.
	return nil, nil
}
