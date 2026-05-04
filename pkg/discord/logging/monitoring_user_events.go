package logging

import (
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

type auditRolePartial struct {
	ID   string
	Name string
}

func diffStringIDs(prev, cur []string) (added []string, removed []string) {
	curSet := make(map[string]struct{}, len(cur))
	for _, roleID := range cur {
		if roleID != "" {
			curSet[roleID] = struct{}{}
		}
	}
	prevSet := make(map[string]struct{}, len(prev))
	for _, roleID := range prev {
		if roleID != "" {
			prevSet[roleID] = struct{}{}
		}
	}
	for roleID := range curSet {
		if _, ok := prevSet[roleID]; !ok {
			added = append(added, roleID)
		}
	}
	for roleID := range prevSet {
		if _, ok := curSet[roleID]; !ok {
			removed = append(removed, roleID)
		}
	}
	return added, removed
}

func (ms *MonitoringService) computeMemberRoleDiff(guildID, userID string, proposed []string) (cur []string, added []string, removed []string, known bool) {
	switch {
	case proposed != nil:
		cur = append([]string(nil), proposed...)
		known = true
	default:
		member, err := ms.getGuildMember(guildID, userID)
		if err == nil && member != nil {
			cur = append([]string(nil), member.Roles...)
			known = true
		}
	}
	if !known {
		return nil, nil, nil, false
	}

	var prev []string
	if p, ok := ms.cacheRolesGet(guildID, userID); ok {
		atomic.AddUint64(&ms.cacheRolesMemoryHits, 1)
		prev = p
	} else if ms.store != nil {
		if r, err := ms.store.GetMemberRoles(guildID, userID); err == nil {
			atomic.AddUint64(&ms.cacheRolesStoreHits, 1)
			prev = r
		}
	}

	added, removed = diffStringIDs(prev, cur)
	return cur, added, removed, true
}

func (ms *MonitoringService) persistMemberRoleSnapshot(guildID, userID string, roles []string) error {
	if ms.store != nil {
		if err := ms.store.UpsertMemberRoles(guildID, userID, roles, time.Now()); err != nil {
			return err
		}
	}
	ms.cacheRolesSet(guildID, userID, roles)
	return nil
}

func (ms *MonitoringService) getRoleUpdateAuditEntries(guildID string, forceRefresh bool) ([]*discordgo.AuditLogEntry, bool, error) {
	now := time.Now()

	ms.roleUpdateAuditMu.Lock()
	ms.ensureRoleUpdateAuditStateLocked()
	if !forceRefresh {
		if entry, ok := ms.roleUpdateAuditCache[guildID]; ok && now.Sub(entry.fetchedAt) < monitoringRoleAuditCacheTTL {
			entries := append([]*discordgo.AuditLogEntry(nil), entry.entries...)
			atomic.AddUint64(&ms.cacheRoleAuditHits, 1)
			ms.roleUpdateAuditMu.Unlock()
			return entries, true, nil
		}
	}
	ms.roleUpdateAuditMu.Unlock()

	audit, err := ms.session.GuildAuditLog(guildID, "", "", int(discordgo.AuditLogActionMemberRoleUpdate), 10)
	atomic.AddUint64(&ms.apiAuditLogCalls, 1)
	if err != nil {
		return nil, false, err
	}
	if audit == nil {
		return nil, false, nil
	}

	entries := make([]*discordgo.AuditLogEntry, 0, len(audit.AuditLogEntries))
	for _, entry := range audit.AuditLogEntries {
		if entry == nil || entry.ActionType == nil || *entry.ActionType != discordgo.AuditLogActionMemberRoleUpdate {
			continue
		}
		entries = append(entries, entry)
	}

	ms.roleUpdateAuditMu.Lock()
	ms.roleUpdateAuditCache[guildID] = cachedRoleUpdateAudit{
		fetchedAt: now,
		entries:   append([]*discordgo.AuditLogEntry(nil), entries...),
	}
	if len(ms.roleUpdateAuditCache) > 100 {
		for key, entry := range ms.roleUpdateAuditCache {
			if now.Sub(entry.fetchedAt) > 5*time.Minute {
				delete(ms.roleUpdateAuditCache, key)
			}
		}
	}
	ms.roleUpdateAuditMu.Unlock()

	return entries, false, nil
}

func (ms *MonitoringService) shouldDebounceRoleUpdateAuditRefresh(guildID, userID string) bool {
	now := time.Now()
	key := guildID + ":" + userID

	ms.roleUpdateAuditMu.Lock()
	defer ms.roleUpdateAuditMu.Unlock()
	ms.ensureRoleUpdateAuditStateLocked()

	if last, ok := ms.roleUpdateAuditDebounce[key]; ok && now.Sub(last) < monitoringRoleAuditDebounceTTL {
		return true
	}
	ms.roleUpdateAuditDebounce[key] = now
	if len(ms.roleUpdateAuditDebounce) > 200 {
		for debounceKey, last := range ms.roleUpdateAuditDebounce {
			if now.Sub(last) > 5*time.Minute {
				delete(ms.roleUpdateAuditDebounce, debounceKey)
			}
		}
	}
	return false
}

func isRecentRoleUpdateAuditEntry(entry *discordgo.AuditLogEntry) bool {
	if entry == nil || entry.ID == "" {
		return true
	}
	entryTime, ok := snowflakeTimestamp(entry.ID)
	if !ok {
		return true
	}
	return time.Since(entryTime) <= monitoringRoleAuditEntryMaxAge
}

func extractAuditRolePartials(v interface{}) []auditRolePartial {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]auditRolePartial, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role := auditRolePartial{}
		if value, ok := obj["id"].(string); ok {
			role.ID = value
		}
		if value, ok := obj["name"].(string); ok {
			role.Name = value
		}
		if role.ID != "" || role.Name != "" {
			out = append(out, role)
		}
	}
	return out
}

func extractAuditRoleDelta(entry *discordgo.AuditLogEntry) (added []auditRolePartial, removed []auditRolePartial) {
	if entry == nil {
		return nil, nil
	}
	for _, change := range entry.Changes {
		if change == nil || change.Key == nil {
			continue
		}
		switch *change.Key {
		case discordgo.AuditLogChangeKeyRoleAdd:
			added = append(added, extractAuditRolePartials(change.NewValue)...)
			added = append(added, extractAuditRolePartials(change.OldValue)...)
		case discordgo.AuditLogChangeKeyRoleRemove:
			removed = append(removed, extractAuditRolePartials(change.NewValue)...)
			removed = append(removed, extractAuditRolePartials(change.OldValue)...)
		}
	}
	return added, removed
}

func toIDSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			set[id] = struct{}{}
		}
	}
	return set
}

func buildAuditRoleList(list []auditRolePartial) string {
	if len(list) == 0 {
		return "None"
	}
	out := ""
	for i, role := range list {
		display := ""
		if role.ID != "" {
			display = "<@&" + role.ID + ">"
		}
		if display == "" && role.Name != "" {
			display = "`" + role.Name + "`"
		}
		if display == "" && role.ID != "" {
			display = "`" + role.ID + "`"
		}
		if i > 0 {
			out += ", "
		}
		out += display
	}
	return out
}

func buildRoleIDList(list []string) string {
	if len(list) == 0 {
		return "None"
	}
	out := ""
	for i, id := range list {
		if i > 0 {
			out += ", "
		}
		out += "<@&" + id + ">"
	}
	return out
}

func (ms *MonitoringService) sendRoleUpdateNotification(channelID string, user *discordgo.User, actorID string, added string, removed string, source string) error {
	if user == nil {
		return fmt.Errorf("role update user is nil")
	}
	targetLabel := formatUserLabel(user.Username, user.ID)
	actorLabel := formatUserRef(actorID)
	desc := fmt.Sprintf("**Target:** %s\n**Actor:** %s", targetLabel, actorLabel)
	embed := &discordgo.MessageEmbed{
		Title:       "Roles Updated",
		Color:       theme.MemberRoleUpdate(),
		Description: desc,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Added", Value: added, Inline: true},
			{Name: "Removed", Value: removed, Inline: true},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: source,
		},
	}

	atomic.AddUint64(&ms.apiMessagesSent, 1)
	_, err := ms.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// handleMemberUpdate processes member updates.
func (ms *MonitoringService) handleMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.User == nil {
		return
	}
	if !ms.handlesGuild(m.GuildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_update.monitoring",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.User.ID),
	)
	defer done()

	gcfg := ms.configManager.GuildConfig(m.GuildID)
	if gcfg == nil {
		return
	}

	ms.applyStatsMemberUpdate(m.GuildID, m.User.ID, m.User.Bot, m.Roles)
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)

	emit := ShouldEmitLogEvent(ms.session, ms.configManager, LogEventRoleChange, m.GuildID)
	if !emit.Enabled {
		log.ApplicationLogger().Debug("Role update notification suppressed by policy", "guildID", m.GuildID, "userID", m.User.ID, "reason", emit.Reason)
		return
	}
	channelID := emit.ChannelID

	curRoles, verifiedAdded, verifiedRemoved, known := ms.computeMemberRoleDiff(m.GuildID, m.User.ID, m.Roles)
	if !known {
		log.ApplicationLogger().Debug(
			"Role update skipped because current role state could not be resolved",
			"guildID", m.GuildID,
			"userID", m.User.ID,
		)
		return
	}

	if len(verifiedAdded) == 0 && len(verifiedRemoved) == 0 {
		if err := ms.persistMemberRoleSnapshot(m.GuildID, m.User.ID, curRoles); err != nil {
			log.ApplicationLogger().Warn(
				"Failed to persist role snapshot after empty local diff",
				"guildID", m.GuildID,
				"userID", m.User.ID,
				"roleCount", len(curRoles),
				"err", err,
			)
		}
		return
	}

	tryNotifyFromEntries := func(entries []*discordgo.AuditLogEntry) bool {
		verifiedAddedSet := toIDSet(verifiedAdded)
		verifiedRemovedSet := toIDSet(verifiedRemoved)

		for _, entry := range entries {
			if entry == nil || entry.TargetID != m.User.ID || !isRecentRoleUpdateAuditEntry(entry) {
				continue
			}

			auditAdded, auditRemoved := extractAuditRoleDelta(entry)
			if len(auditAdded) == 0 && len(auditRemoved) == 0 {
				continue
			}

			filteredAdded := make([]auditRolePartial, 0, len(auditAdded))
			for _, role := range auditAdded {
				if _, ok := verifiedAddedSet[role.ID]; ok {
					filteredAdded = append(filteredAdded, role)
				}
			}
			filteredRemoved := make([]auditRolePartial, 0, len(auditRemoved))
			for _, role := range auditRemoved {
				if _, ok := verifiedRemovedSet[role.ID]; ok {
					filteredRemoved = append(filteredRemoved, role)
				}
			}

			if len(filteredAdded) == 0 && len(filteredRemoved) == 0 {
				log.ApplicationLogger().Debug(
					"Role update skipped after verification produced empty delta",
					"guildID", m.GuildID,
					"userID", m.User.ID,
					"auditAddedCount", len(auditAdded),
					"auditRemovedCount", len(auditRemoved),
					"verifiedAddedCount", len(verifiedAdded),
					"verifiedRemovedCount", len(verifiedRemoved),
				)
				if err := ms.persistMemberRoleSnapshot(m.GuildID, m.User.ID, curRoles); err != nil {
					log.ApplicationLogger().Warn(
						"Failed to persist role snapshot after verification skip",
						"guildID", m.GuildID,
						"userID", m.User.ID,
						"roleCount", len(curRoles),
						"err", err,
					)
				}
				return true
			}

			if err := ms.sendRoleUpdateNotification(
				channelID,
				m.User,
				entry.UserID,
				buildAuditRoleList(filteredAdded),
				buildAuditRoleList(filteredRemoved),
				"Source: Audit Log",
			); err != nil {
				log.ErrorLoggerRaw().Error("Failed to send role update notification", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID, "err", err)
			} else {
				log.ApplicationLogger().Info("Role update notification sent successfully", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID)
				if err := ms.persistMemberRoleSnapshot(m.GuildID, m.User.ID, curRoles); err != nil {
					log.ApplicationLogger().Warn(
						"Failed to persist role snapshot after role update notification",
						"guildID", m.GuildID,
						"userID", m.User.ID,
						"roleCount", len(curRoles),
						"err", err,
					)
				}
			}
			return true
		}
		return false
	}

	auditLookupDebounced := ms.shouldDebounceRoleUpdateAuditRefresh(m.GuildID, m.User.ID)
	entries, fromCache, err := ms.getRoleUpdateAuditEntries(m.GuildID, false)
	if err != nil {
		log.ApplicationLogger().Warn("Failed to fetch audit logs for role update", "guildID", m.GuildID, "userID", m.User.ID, "err", err)
	} else if tryNotifyFromEntries(entries) {
		return
	}

	if fromCache && !auditLookupDebounced {
		time.Sleep(monitoringRoleAuditRetryDelay)
		refreshedEntries, _, refreshErr := ms.getRoleUpdateAuditEntries(m.GuildID, true)
		if refreshErr != nil {
			log.ApplicationLogger().Warn("Failed to refresh audit logs for role update", "guildID", m.GuildID, "userID", m.User.ID, "err", refreshErr)
		} else if tryNotifyFromEntries(refreshedEntries) {
			return
		}
	}

	if err := ms.sendRoleUpdateNotification(
		channelID,
		m.User,
		"",
		buildRoleIDList(verifiedAdded),
		buildRoleIDList(verifiedRemoved),
		"Source: Role Diff",
	); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send fallback role update notification", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID, "err", err)
		return
	}

	log.ApplicationLogger().Info("Fallback role update notification sent successfully", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID)
	if err := ms.persistMemberRoleSnapshot(m.GuildID, m.User.ID, curRoles); err != nil {
		log.ApplicationLogger().Warn(
			"Failed to persist role snapshot after fallback role update notification",
			"guildID", m.GuildID,
			"userID", m.User.ID,
			"roleCount", len(curRoles),
			"err", err,
		)
	}
}

// handleUserUpdate processes user updates across all configured guilds.
func (ms *MonitoringService) handleUserUpdate(s *discordgo.Session, m *discordgo.UserUpdate) {
	if m == nil || m.User == nil {
		return
	}

	done := perf.StartGatewayEvent(
		"user_update",
		slog.String("userID", m.User.ID),
	)
	defer done()

	cfg := ms.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 {
		return
	}
	for _, gcfg := range cfg.Guilds {
		var member *discordgo.Member
		if m2, err := ms.getGuildMember(gcfg.GuildID, m.User.ID); err == nil {
			member = m2
		}
		if member == nil || member.User == nil {
			continue
		}
		ms.checkAvatarChange(gcfg.GuildID, member.User.ID, member.User.Avatar, member.User.Username)
	}
}

// checkAvatarChange aplica debounce e delega processamento ao UserWatcher.
func (ms *MonitoringService) checkAvatarChange(guildID, userID, currentAvatar, username string) {
	if currentAvatar == "" {
		currentAvatar = "default"
	}
	changeKey := fmt.Sprintf("%s:%s:%s", guildID, userID, currentAvatar)
	ms.changesMutex.RLock()
	if lastChange, exists := ms.recentChanges[changeKey]; exists {
		if time.Since(lastChange) < 65*time.Second {
			ms.changesMutex.RUnlock()
			return
		}
	}
	ms.changesMutex.RUnlock()

	changed := true
	if ms.unifiedCache != nil {
		if member, ok := ms.unifiedCache.GetMember(guildID, userID); ok {
			if member.User != nil && member.User.Avatar == currentAvatar {
				changed = false
			}
		}
	}

	if changed {
		ms.changesMutex.Lock()
		ms.recentChanges[changeKey] = time.Now()
		if len(ms.recentChanges) > 100 {
			for key, timestamp := range ms.recentChanges {
				if time.Since(timestamp) > 5*time.Minute {
					delete(ms.recentChanges, key)
				}
			}
		}
		ms.changesMutex.Unlock()

		if ms.adapters != nil {
			if err := ms.adapters.EnqueueProcessAvatarChange(guildID, userID, username, currentAvatar); err != nil {
				if errors.Is(err, task.ErrDuplicateTask) {
					log.ApplicationLogger().Info("Avatar change task already enqueued (idempotency)", "guildID", guildID, "userID", userID)
				} else {
					log.ErrorLoggerRaw().Error("Failed to enqueue avatar change task; falling back to synchronous processing", "guildID", guildID, "userID", userID, "err", err)
					ms.userWatcher.ProcessChange(guildID, userID, currentAvatar, username)
				}
			}
		} else {
			ms.userWatcher.ProcessChange(guildID, userID, currentAvatar, username)
		}
	}
}

// ProcessChange performs avatar-specific logic: notification and persistence.
func (aw *UserWatcher) ProcessChange(guildID, userID, currentAvatar, username string) {
	oldAvatar, _, ok, err := aw.store.GetAvatar(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to fetch current avatar from store", "guildID", guildID, "userID", userID, "err", err)
	}

	if ok && oldAvatar == currentAvatar {
		return
	}

	finalUsername := username
	if finalUsername == "" {
		finalUsername = aw.getUsernameForNotification(guildID, userID)
	}

	change := files.AvatarChange{
		UserID:    userID,
		Username:  finalUsername,
		OldAvatar: oldAvatar,
		NewAvatar: currentAvatar,
		Timestamp: time.Now(),
	}

	log.ApplicationLogger().Info("Avatar change detected and processing", "userID", userID, "guildID", guildID, "old_avatar", oldAvatar, "new_avatar", currentAvatar)

	emit := ShouldEmitLogEvent(aw.session, aw.configManager, LogEventAvatarChange, guildID)
	if !emit.Enabled {
		if emit.Reason == EmitReasonNoChannelConfigured {
			log.ErrorLoggerRaw().Error("User activity log channel not configured; notification not sent", "guildID", guildID)
		} else {
			log.ApplicationLogger().Debug("Avatar notification suppressed by policy", "guildID", guildID, "userID", userID, "reason", emit.Reason)
		}
	} else {
		if err := aw.notifier.SendAvatarChangeNotification(emit.ChannelID, change); err != nil {
			log.ErrorLoggerRaw().Error("Error sending notification", "channelID", emit.ChannelID, "userID", userID, "guildID", guildID, "err", err)
		} else {
			log.ApplicationLogger().Info("Avatar notification sent successfully", "channelID", emit.ChannelID, "userID", userID, "guildID", guildID)
		}
	}

	if _, _, err := aw.store.UpsertAvatar(guildID, userID, currentAvatar, time.Now()); err != nil {
		log.ErrorLoggerRaw().Error("Error saving avatar in store for guild", "guildID", guildID, "err", err)
	}
}

func (aw *UserWatcher) getUsernameForNotification(guildID, userID string) string {
	if aw.cache != nil {
		if member, ok := aw.cache.GetMember(guildID, userID); ok {
			if member.Nick != "" {
				return member.Nick
			}
			if member.User != nil && member.User.Username != "" {
				return member.User.Username
			}
		}
	}

	if aw.session != nil && aw.session.State != nil {
		if m, _ := aw.session.State.Member(guildID, userID); m != nil {
			if aw.cache != nil {
				aw.cache.SetMember(guildID, userID, m)
			}
			if m.Nick != "" {
				return m.Nick
			}
			if m.User != nil && m.User.Username != "" {
				return m.User.Username
			}
		}
	}

	member, err := aw.session.GuildMember(guildID, userID)
	if err != nil || member == nil {
		log.ApplicationLogger().Info("Error getting member for username; using ID", "userID", userID, "guildID", guildID, "err", err)
		return userID
	}

	if aw.cache != nil {
		aw.cache.SetMember(guildID, userID, member)
	}

	if member.Nick != "" {
		return member.Nick
	}
	if member.User != nil && member.User.Username != "" {
		return member.User.Username
	}
	return userID
}
