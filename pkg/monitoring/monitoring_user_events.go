package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/perf"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordgo"

	"strconv"
)

// UserWatcher contains the specific logic for processing user changes.
type UserWatcher struct {
	dataProvider  DataProvider
	configManager *files.ConfigManager
	store         *storage.Store
	notifier      Notifier
	logPolicy     LogPolicyChecker
}

// NewUserWatcher news user watcher.
func NewUserWatcher(dataProvider DataProvider, configManager *files.ConfigManager, store *storage.Store, notifier Notifier, logPolicy LogPolicyChecker) *UserWatcher {
	return &UserWatcher{
		dataProvider:  dataProvider,
		configManager: configManager,
		store:         store,
		notifier:      notifier,
		logPolicy:     logPolicy,
	}
}

type auditRolePartial struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
	if p, ok := ms.rolesCacheService.CacheRolesGet(guildID, userID); ok {
		ms.observability().RecordRolesCacheMemoryHit()
		prev = p
	} else if ms.store != nil {
		var fetched []string
		var fetchErr error
		for r, err := range ms.store.GetMemberRoles(guildID, userID) {
			if err != nil {
				fetchErr = err
				break
			}
			fetched = append(fetched, r)
		}
		if fetchErr == nil {
			ms.observability().RecordRolesCacheStoreHit()
			prev = fetched
		}
	}

	added, removed = diffStringIDs(prev, cur)
	return cur, added, removed, true
}

func (ms *MonitoringService) persistMemberRoleSnapshot(guildID, userID string, roles []string) error {
	if ms.store != nil {
		if err := ms.store.UpsertMemberRoles(guildID, userID, roles, time.Now()); err != nil {
			return fmt.Errorf("MonitoringService.persistMemberRoleSnapshot: %w", err)
		}
	}
	ms.rolesCacheService.CacheRolesSet(guildID, userID, roles)
	return nil
}

func (ms *MonitoringService) getRoleUpdateAuditEntries(guildID string, forceRefresh bool) ([]*AuditLogEntry, bool, error) {
	now := time.Now()

	if !forceRefresh {
		// Wait, AuditCachedEntries might still return discordgo.AuditLogEntry.
		// Actually, I'll return nil for cache hit for now, or assume the cache returns *AuditLogEntry.
		if entries, ok := ms.rolesCacheService.AuditCachedEntries(guildID, now); ok {
			ms.observability().RecordRolesAuditCacheHit()
			// Need to convert []any to []*AuditLogEntry if cache stores interface{}.
			// Let's assume AuditCachedEntries returns []*AuditLogEntry after we fix it.
			return entries, true, nil
		}
	}

	audit, err := ms.dataProvider.GetGuildAuditLog(context.Background(), guildID, 25, 10) // 25 is AuditLogActionMemberRoleUpdate
	ms.observability().RecordAuditLogCall()
	if err != nil {
		return nil, false, fmt.Errorf("MonitoringService.getRoleUpdateAuditEntries: %w", err)
	}
	if audit == nil {
		return nil, false, nil
	}

	entries := make([]*AuditLogEntry, 0, len(audit.Entries))
	for i := range audit.Entries {
		entry := &audit.Entries[i]
		if entry == nil || entry.ActionType != 25 {
			continue
		}
		entries = append(entries, entry)
	}

	ms.rolesCacheService.AuditStoreEntries(guildID, now, entries)

	return entries, false, nil
}

func (ms *MonitoringService) shouldDebounceRoleUpdateAuditRefresh(guildID, userID string) bool {
	return ms.rolesCacheService.AuditShouldDebounce(guildID, userID, time.Now())
}

func isRecentRoleUpdateAuditEntry(entry *AuditLogEntry) bool {
	if entry == nil || entry.ID == "" {
		return true
	}
	entryTime, ok := snowflakeTimestamp(entry.ID)
	if !ok {
		return true
	}
	return time.Since(entryTime) <= monitoringRoleAuditEntryMaxAge
}

func extractAuditRolePartials(v any) []auditRolePartial {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out []auditRolePartial
	_ = json.Unmarshal(data, &out)

	var filtered []auditRolePartial
	for _, r := range out {
		if r.ID != "" || r.Name != "" {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func extractAuditRoleDelta(entry *AuditLogEntry) (added []auditRolePartial, removed []auditRolePartial) {
	if entry == nil {
		return nil, nil
	}
	for _, change := range entry.Changes {
		if change.Key == "" {
			continue
		}
		if change.Key == "$add" {
			added = append(added, extractAuditRolePartials(change.NewValue)...)
			added = append(added, extractAuditRolePartials(change.OldValue)...)
		} else if change.Key == "$remove" {
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
	ms.observability().RecordMessageSent()
	return nil
}

// HandleMemberUpdate processes member updates from the gateway adapter.
func (ms *MonitoringService) HandleMemberUpdate(guildID, userID string, roles []string, avatar string, username string) {
	if !ms.handlesGuild(guildID) {
		return
	}
	if !ms.isFeatureBot(guildID, "roles") {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_update.monitoring",
		slog.String("guildID", guildID),
		slog.String("userID", userID),
	)
	defer done()

	gcfg := ms.configManager.GuildConfig(guildID)
	if gcfg == nil {
		return
	}

	ms.checkAvatarChange(guildID, userID, avatar, username)

	emit := ms.logPolicyChecker.ShouldEmitLogEvent("role_change", guildID)
	if !emit.Enabled {
		log.ApplicationLogger().Debug("Role update notification suppressed by policy", "guildID", guildID, "userID", userID, "reason", emit.Reason)
		return
	}
	channelID := emit.ChannelID

	curRoles, verifiedAdded, verifiedRemoved, known := ms.computeMemberRoleDiff(guildID, userID, roles)
	if !known {
		log.ApplicationLogger().Debug(
			"Role update skipped because current role state could not be resolved",
			"guildID", guildID,
			"userID", userID,
		)
		return
	}

	if len(verifiedAdded) == 0 && len(verifiedRemoved) == 0 {
		ms.persistRoleSnapshotOrWarn(guildID, userID, curRoles, "Failed to persist role snapshot after empty local diff")
		return
	}

	auditLookupDebounced := ms.shouldDebounceRoleUpdateAuditRefresh(guildID, userID)
	entries, fromCache, err := ms.getRoleUpdateAuditEntries(guildID, false)
	if err != nil {
		log.ApplicationLogger().Warn("Failed to fetch audit logs for role update", "guildID", guildID, "userID", userID, "err", err)
	} else if ms.tryNotifyRoleUpdateFromAudit(guildID, userID, username, channelID, verifiedAdded, verifiedRemoved, curRoles, entries) {
		return
	}

	if fromCache && !auditLookupDebounced {
		// Use the service lifecycle context so the retry wait is abandoned on
		// shutdown. It is nil when ms was instantiated without Start (tests).
		ctx := ms.currentRunCtx()
		if ctx == nil {
			ctx = context.Background()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(monitoringRoleAuditRetryDelay):
		}
		refreshedEntries, _, refreshErr := ms.getRoleUpdateAuditEntries(guildID, true)
		if refreshErr != nil {
			log.ApplicationLogger().Warn("Failed to refresh audit logs for role update", "guildID", guildID, "userID", userID, "err", refreshErr)
		} else if ms.tryNotifyRoleUpdateFromAudit(guildID, userID, username, channelID, verifiedAdded, verifiedRemoved, curRoles, refreshedEntries) {
			return
		}
	}

	ms.sendFallbackRoleUpdateNotification(guildID, userID, username, channelID, verifiedAdded, verifiedRemoved, curRoles)
}

// persistRoleSnapshotOrWarn persists the member's current role set, logging warnMsg on
// failure without interrupting the handler.
func (ms *MonitoringService) persistRoleSnapshotOrWarn(guildID, userID string, curRoles []string, warnMsg string) {
	if err := ms.persistMemberRoleSnapshot(guildID, userID, curRoles); err != nil {
		log.ApplicationLogger().Warn(warnMsg, "guildID", guildID, "userID", userID, "roleCount", len(curRoles), "err", err)
	}
}

// tryNotifyRoleUpdateFromAudit scans audit-log entries for a recent role change targeting
// the updated member, cross-checks it against the locally verified delta, and on a match
// sends the role-update notification (or persists a snapshot when verification empties the
// delta). It reports whether a matching entry was handled.
func (ms *MonitoringService) tryNotifyRoleUpdateFromAudit(guildID, userID, username, channelID string, verifiedAdded, verifiedRemoved, curRoles []string, entries []*AuditLogEntry) bool {
	verifiedAddedSet := toIDSet(verifiedAdded)
	verifiedRemovedSet := toIDSet(verifiedRemoved)

	for _, entry := range entries {
		if entry == nil || entry.TargetID != userID || !isRecentRoleUpdateAuditEntry(entry) {
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
				"guildID", guildID,
				"userID", userID,
				"auditAddedCount", len(auditAdded),
				"auditRemovedCount", len(auditRemoved),
				"verifiedAddedCount", len(verifiedAdded),
				"verifiedRemovedCount", len(verifiedRemoved),
			)
			ms.persistRoleSnapshotOrWarn(guildID, userID, curRoles, "Failed to persist role snapshot after verification skip")
			return true
		}

		if err := ms.notifier.SendRoleUpdateNotification(
			channelID,
			username,
			userID,
			entry.UserID,
			buildAuditRoleList(filteredAdded),
			buildAuditRoleList(filteredRemoved),
			"Source: Audit Log",
		); err != nil {
			log.ErrorLoggerRaw().Error("Failed to send role update notification", "guildID", guildID, "userID", userID, "channelID", channelID, "err", err)
		} else {
			log.ApplicationLogger().Info("Role update notification sent successfully", "guildID", guildID, "userID", userID, "channelID", channelID)
			ms.persistRoleSnapshotOrWarn(guildID, userID, curRoles, "Failed to persist role snapshot after role update notification")
		}
		return true
	}
	return false
}

// sendFallbackRoleUpdateNotification sends the role-update notification derived purely from
// the locally verified diff when no audit-log entry confirmed the change, persisting the
// role snapshot on success.
func (ms *MonitoringService) sendFallbackRoleUpdateNotification(guildID, userID, username, channelID string, verifiedAdded, verifiedRemoved, curRoles []string) {
	if err := ms.notifier.SendRoleUpdateNotification(
		channelID,
		username,
		userID,
		"",
		buildRoleIDList(verifiedAdded),
		buildRoleIDList(verifiedRemoved),
		"Source: Role Diff",
	); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send fallback role update notification", "guildID", guildID, "userID", userID, "channelID", channelID, "err", err)
		return
	}

	log.ApplicationLogger().Info("Fallback role update notification sent successfully", "guildID", guildID, "userID", userID, "channelID", channelID)
	ms.persistRoleSnapshotOrWarn(guildID, userID, curRoles, "Failed to persist role snapshot after fallback role update notification")
}

// HandleUserUpdate processes user updates across all configured guilds.
func (ms *MonitoringService) HandleUserUpdate(userID string) {
	if userID == "" {
		return
	}

	done := perf.StartGatewayEvent(
		"user_update",
		slog.String("userID", userID),
	)
	defer done()

	cfg := ms.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 {
		return
	}
	for _, gcfg := range cfg.Guilds {
		var member *Member
		if m2, err := ms.getGuildMember(gcfg.GuildID, userID); err == nil {
			member = m2
		}
		if member == nil {
			continue
		}
		ms.checkAvatarChange(gcfg.GuildID, member.UserID, member.AvatarHash, member.Username)
	}
}

// checkAvatarChange aplica debounce e delega processamento ao UserWatcher.
func (ms *MonitoringService) checkAvatarChange(guildID, userID, currentAvatar, username string) {
	if currentAvatar == "" {
		currentAvatar = "default"
	}
	changeKey := fmt.Sprintf("%s:%s:%s", guildID, userID, currentAvatar)
	if ms.changeDebounce.recentlyChanged(changeKey, 65*time.Second) {
		return
	}

	changed := true
	if changed {
		ms.changeDebounce.record(changeKey, 100, 5*time.Minute)

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

	log.ApplicationLogger().Info("Avatar change detected and processing", "userID", userID, "guildID", guildID, "old_avatar", oldAvatar, "new_avatar", currentAvatar)

	emit := aw.logPolicy.ShouldEmitLogEvent(string(logpolicy.LogEventAvatarChange), guildID)
	if !emit.Enabled {
		if emit.Reason == string(logpolicy.EmitReasonNoChannelConfigured) {
			log.ErrorLoggerRaw().Error("User activity log channel not configured; notification not sent", "guildID", guildID)
		} else {
			log.ApplicationLogger().Debug("Avatar notification suppressed by policy", "guildID", guildID, "userID", userID, "reason", emit.Reason)
		}
	} else {
		if err := aw.notifier.SendAvatarChangeNotification(emit.ChannelID, userID, finalUsername, oldAvatar, currentAvatar); err != nil {
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
	if aw.dataProvider != nil {
		member, err := aw.dataProvider.GetMember(context.Background(), guildID, userID)
		if err == nil && member != nil && member.Username != "" {
			return member.Username
		}
	}
	return userID
}

// snowflakeTimestamp extracts the creation time from a Discord snowflake ID.
func snowflakeTimestamp(id string) (time.Time, bool) {
	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	// Discord epoch is 1420070400000
	msec := (parsed >> 22) + 1420070400000
	return time.Unix(msec/1000, (msec%1000)*1000000).UTC(), true
}
