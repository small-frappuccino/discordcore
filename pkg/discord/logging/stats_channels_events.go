package logging

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

func (ms *MonitoringService) handleStatsMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m == nil || m.Member == nil || m.Member.User == nil {
		return
	}
	if !ms.handlesGuild(m.GuildID) {
		return
	}
	ms.applyStatsMemberAdd(m.Member)
}

func (ms *MonitoringService) handleStatsMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m == nil || m.User == nil {
		return
	}
	if !ms.handlesGuild(m.GuildID) {
		return
	}
	ms.applyStatsMemberRemove(m.GuildID, m.User.ID)
}

func (ms *MonitoringService) applyStatsMemberAdd(member *discordgo.Member) {
	if member == nil || member.User == nil {
		return
	}
	guildID := strings.TrimSpace(member.GuildID)
	userID := strings.TrimSpace(member.User.ID)
	if guildID == "" || userID == "" {
		return
	}

	_, trackedRoles, trackedRolesKey, enabled := ms.statsGuildConfig(guildID)
	if !enabled {
		return
	}
	ms.persistStatsMemberActive(guildID, userID, member.JoinedAt, member.User.Bot, member.Roles)
	snapshot := statsMemberSnapshot{
		isBot:        member.User.Bot,
		trackedRoles: filterTrackedRoles(member.Roles, trackedRoles),
	}

	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()

	state := ms.ensureStatsGuildStateLocked(guildID)
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyAdd(userID, snapshot) {
		state.dirty = true
	}
}

func (ms *MonitoringService) applyStatsMemberUpdate(guildID, userID string, isBot bool, roles []string) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	_, trackedRoles, trackedRolesKey, enabled := ms.statsGuildConfig(guildID)
	if !enabled {
		return
	}
	ms.persistStatsMemberActive(guildID, userID, time.Time{}, isBot, roles)
	snapshot := statsMemberSnapshot{
		isBot:        isBot,
		trackedRoles: filterTrackedRoles(roles, trackedRoles),
	}

	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()

	state := ms.ensureStatsGuildStateLocked(guildID)
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyUpdate(userID, snapshot) {
		state.dirty = true
	}
}

func (ms *MonitoringService) applyStatsMemberRemove(guildID, userID string) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	_, _, trackedRolesKey, enabled := ms.statsGuildConfig(guildID)
	if !enabled {
		return
	}
	ms.persistStatsMemberLeft(guildID, userID)

	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()

	state := ms.ensureStatsGuildStateLocked(guildID)
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyRemove(userID) {
		state.dirty = true
	}
}

func (ms *MonitoringService) persistStatsMemberActive(guildID, userID string, joinedAt time.Time, isBot bool, roles []string) {
	if ms == nil || ms.store == nil {
		return
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	err := monitoringRunErrWithTimeoutContext(context.Background(), monitoringPersistenceTimeout, func(runCtx context.Context) error {
		if err := ms.store.UpsertMemberPresenceContext(runCtx, guildID, userID, joinedAt, time.Now().UTC(), isBot); err != nil {
			return fmt.Errorf("upsert member presence: %w", err)
		}
		if err := ms.store.UpsertMemberRoles(guildID, userID, roles, time.Now().UTC()); err != nil {
			return fmt.Errorf("upsert member roles: %w", err)
		}
		return nil
	})
	if err != nil {
		log.ApplicationLogger().Warn(
			"Failed to persist stats member state",
			"operation", "monitoring.stats.persist_member_active",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
	}
}

func (ms *MonitoringService) persistStatsMemberLeft(guildID, userID string) {
	if ms == nil || ms.store == nil {
		return
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	err := monitoringRunErrWithTimeoutContext(context.Background(), monitoringPersistenceTimeout, func(runCtx context.Context) error {
		return ms.store.MarkMemberLeftContext(runCtx, guildID, userID, time.Now().UTC())
	})
	if err != nil {
		log.ApplicationLogger().Warn(
			"Failed to persist stats member leave",
			"operation", "monitoring.stats.persist_member_left",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
	}
}
