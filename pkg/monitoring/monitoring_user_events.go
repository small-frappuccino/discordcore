package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordgo"
	log_slog "log/slog"

	"github.com/small-frappuccino/discordcore/pkg/notifications"

	"strconv"
)

// UserWatcher contains the specific logic for processing user changes.
type UserWatcher struct {
	session       *discordgo.Session
	arikawaState  *state.State
	logger        *log_slog.Logger
	configManager *files.ConfigManager
	store         *storage.Store
	notifier      *notifications.NotificationSender
	cache         *cache.UnifiedCache
}

// NewUserWatcher news user watcher.
func NewUserWatcher(session *discordgo.Session, arikawaState *state.State, configManager *files.ConfigManager, store *storage.Store, notifier *notifications.NotificationSender, unifiedCache *cache.UnifiedCache, logger *log_slog.Logger) *UserWatcher {
	return &UserWatcher{
		session:       session,
		arikawaState:  arikawaState,
		configManager: configManager,
		store:         store,
		notifier:      notifier,
		cache:         unifiedCache,
		logger:        logger,
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

func (ms *MonitoringService) computeMemberRoleDiff(guildID, userID string, proposed []discord.RoleID) (cur []string, added []string, removed []string, known bool) {
	switch {
	case proposed != nil:
		for _, r := range proposed {
			cur = append(cur, r.String())
		}
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

func (ms *MonitoringService) getRoleUpdateAuditEntries(guildID string, forceRefresh bool) ([]discord.AuditLogEntry, bool, error) {
	now := time.Now()

	if !forceRefresh {
		if entries, ok := ms.rolesCacheService.AuditCachedEntries(guildID, now); ok {
			ms.observability().RecordRolesAuditCacheHit()
			return entries, true, nil
		}
	}

	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, false, err
	}

	audit, err := ms.arikawaState.Client.AuditLog(discord.GuildID(gID), api.AuditLogData{
		ActionType: discord.MemberRoleUpdate,
		Limit:      10,
	})
	ms.observability().RecordAuditLogCall()
	if err != nil {
		return nil, false, fmt.Errorf("MonitoringService.getRoleUpdateAuditEntries: %w", err)
	}
	if audit == nil {
		return nil, false, nil
	}

	entries := make([]discord.AuditLogEntry, 0, len(audit.Entries))
	for _, entry := range audit.Entries {
		if entry.ActionType != discord.MemberRoleUpdate {
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

func isRecentRoleUpdateAuditEntry(entry discord.AuditLogEntry) bool {
	if !entry.ID.IsValid() {
		return true
	}
	return time.Since(entry.ID.Time()) <= monitoringRoleAuditEntryMaxAge
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

func extractAuditRoleDelta(entry discord.AuditLogEntry) (added []auditRolePartial, removed []auditRolePartial) {
	for _, change := range entry.Changes {
		switch change.Key {
		case discord.AuditGuildRoleAdd:
			added = append(added, extractAuditRolePartials(change.NewValue)...)
			added = append(added, extractAuditRolePartials(change.OldValue)...)
		case discord.AuditGuildRoleRemove:
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

func (ms *MonitoringService) sendRoleUpdateNotification(guildID string, channelID string, user *discord.User, actorID string, added string, removed string, source string) error {
	if user == nil {
		return fmt.Errorf("role update user is nil")
	}
	if ms.eventLogger == nil {
		return fmt.Errorf("eventLogger is nil")
	}

	ariUser := *user

	// We extract RoleIDs from the added/removed strings for the Sink
	var addedRoles, removedRoles []discord.RoleID
	extractRoles := func(r string) []discord.RoleID {
		var roles []discord.RoleID
		for _, part := range strings.Split(r, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "<@&") && strings.HasSuffix(part, ">") {
				id, _ := discord.ParseSnowflake(part[3 : len(part)-1])
				roles = append(roles, discord.RoleID(id))
			} else if strings.HasPrefix(part, "`") && strings.HasSuffix(part, "`") {
				id, _ := discord.ParseSnowflake(part[1 : len(part)-1])
				roles = append(roles, discord.RoleID(id))
			}
		}
		return roles
	}

	if added != "None" {
		addedRoles = extractRoles(added)
	}
	if removed != "None" {
		removedRoles = extractRoles(removed)
	}

	ms.eventLogger.OnRoleUpdate(context.Background(), guildID, ariUser, addedRoles, removedRoles)
	return nil
}

// handleMemberUpdate processes member updates.
func (ms *MonitoringService) handleMemberUpdate(e *gateway.GuildMemberUpdateEvent) {
	if e.User.ID == 0 {
		return
	}
	guildID := e.GuildID.String()
	userID := e.User.ID.String()
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

	ms.checkAvatarChange(guildID, userID, string(e.User.Avatar), e.User.Username)

	emit := logpolicy.ShouldEmitLogEvent(ms.session, ms.configManager, logpolicy.LogEventRoleChange, guildID)
	if !emit.Enabled {
		ms.logger.LogAttrs(context.Background(), log_slog.LevelDebug, "Role update notification suppressed by policy", log_slog.String("guildID", guildID), log_slog.String("userID", userID), log_slog.String("reason", string(emit.Reason)))
		return
	}
	channelID := emit.ChannelID

	curRoles, verifiedAdded, verifiedRemoved, known := ms.computeMemberRoleDiff(guildID, userID, e.RoleIDs)
	if !known {
		ms.logger.LogAttrs(context.Background(), log_slog.LevelDebug,
			"Role update skipped because current role state could not be resolved",
			log_slog.String("guildID", guildID),
			log_slog.String("userID", userID),
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
		ms.logger.LogAttrs(context.Background(), log_slog.LevelWarn, "Failed to fetch audit logs for role update", log_slog.String("guildID", guildID), log_slog.String("userID", userID), log_slog.Any("err", err))
	} else if ms.tryNotifyRoleUpdateFromAudit(e, channelID, verifiedAdded, verifiedRemoved, curRoles, entries) {
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
			ms.logger.LogAttrs(context.Background(), log_slog.LevelWarn, "Failed to refresh audit logs for role update", log_slog.String("guildID", guildID), log_slog.String("userID", userID), log_slog.Any("err", refreshErr))
		} else if ms.tryNotifyRoleUpdateFromAudit(e, channelID, verifiedAdded, verifiedRemoved, curRoles, refreshedEntries) {
			return
		}
	}

	ms.sendFallbackRoleUpdateNotification(e, channelID, verifiedAdded, verifiedRemoved, curRoles)
}

// persistRoleSnapshotOrWarn persists the member's current role set, logging warnMsg on
// failure without interrupting the handler.
func (ms *MonitoringService) persistRoleSnapshotOrWarn(guildID, userID string, curRoles []string, warnMsg string) {
	if err := ms.persistMemberRoleSnapshot(guildID, userID, curRoles); err != nil {
		ms.logger.LogAttrs(context.Background(), log_slog.LevelWarn, warnMsg, log_slog.String("guildID", guildID), log_slog.String("userID", userID), log_slog.Int("roleCount", len(curRoles)), log_slog.Any("err", err))
	}
}

// tryNotifyRoleUpdateFromAudit scans audit-log entries for a recent role change targeting
// the updated member, cross-checks it against the locally verified delta, and on a match
// sends the role-update notification (or persists a snapshot when verification empties the
// delta). It reports whether a matching entry was handled.
func (ms *MonitoringService) tryNotifyRoleUpdateFromAudit(e *gateway.GuildMemberUpdateEvent, channelID string, verifiedAdded, verifiedRemoved, curRoles []string, entries []discord.AuditLogEntry) bool {
	verifiedAddedSet := toIDSet(verifiedAdded)
	verifiedRemovedSet := toIDSet(verifiedRemoved)

	for _, entry := range entries {
		if entry.TargetID.String() != e.User.ID.String() || !isRecentRoleUpdateAuditEntry(entry) {
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
			ms.logger.LogAttrs(context.Background(), log_slog.LevelDebug,
				"Role update skipped after verification produced empty delta",
				log_slog.String("guildID", e.GuildID.String()),
				log_slog.String("userID", e.User.ID.String()),
				log_slog.Int("auditAddedCount", len(auditAdded)),
				log_slog.Int("auditRemovedCount", len(auditRemoved)),
				log_slog.Int("verifiedAddedCount", len(verifiedAdded)),
				log_slog.Int("verifiedRemovedCount", len(verifiedRemoved)),
			)
			ms.persistRoleSnapshotOrWarn(e.GuildID.String(), e.User.ID.String(), curRoles, "Failed to persist role snapshot after verification skip")
			return true
		}

		if err := ms.sendRoleUpdateNotification(
			e.GuildID.String(),
			channelID,
			&e.User,
			entry.UserID.String(),
			buildAuditRoleList(filteredAdded),
			buildAuditRoleList(filteredRemoved),
			"Source: Audit Log",
		); err != nil {
			ms.logger.LogAttrs(context.Background(), log_slog.LevelError, "Failed to send role update notification", log_slog.String("guildID", e.GuildID.String()), log_slog.String("userID", e.User.ID.String()), log_slog.String("channelID", channelID), log_slog.Any("err", err))
		} else {
			ms.logger.LogAttrs(context.Background(), log_slog.LevelInfo, "Role update notification sent successfully", log_slog.String("guildID", e.GuildID.String()), log_slog.String("userID", e.User.ID.String()), log_slog.String("channelID", channelID))
			ms.persistRoleSnapshotOrWarn(e.GuildID.String(), e.User.ID.String(), curRoles, "Failed to persist role snapshot after role update notification")
		}
		return true
	}
	return false
}

// sendFallbackRoleUpdateNotification sends the role-update notification derived purely from
// the locally verified diff when no audit-log entry confirmed the change, persisting the
// role snapshot on success.
func (ms *MonitoringService) sendFallbackRoleUpdateNotification(e *gateway.GuildMemberUpdateEvent, channelID string, verifiedAdded, verifiedRemoved, curRoles []string) {
	if err := ms.sendRoleUpdateNotification(
		e.GuildID.String(),
		channelID,
		&e.User,
		"",
		buildRoleIDList(verifiedAdded),
		buildRoleIDList(verifiedRemoved),
		"Source: Role Diff",
	); err != nil {
		ms.logger.LogAttrs(context.Background(), log_slog.LevelError, "Failed to send fallback role update notification", log_slog.String("guildID", e.GuildID.String()), log_slog.String("userID", e.User.ID.String()), log_slog.String("channelID", channelID), log_slog.Any("err", err))
		return
	}

	ms.logger.LogAttrs(context.Background(), log_slog.LevelInfo, "Fallback role update notification sent successfully", log_slog.String("guildID", e.GuildID.String()), log_slog.String("userID", e.User.ID.String()), log_slog.String("channelID", channelID))
	ms.persistRoleSnapshotOrWarn(e.GuildID.String(), e.User.ID.String(), curRoles, "Failed to persist role snapshot after fallback role update notification")
}

// handleUserUpdate processes user updates across all configured guilds.
func (ms *MonitoringService) handleUserUpdate(e *gateway.UserUpdateEvent) {
	if e == nil {
		return
	}

	done := perf.StartGatewayEvent(
		"user_update",
		slog.String("userID", e.ID.String()),
	)
	defer done()

	cfg := ms.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 {
		return
	}
	for _, gcfg := range cfg.Guilds {
		gID, err := discord.ParseSnowflake(gcfg.GuildID)
		guildID := discord.GuildID(gID)
		if err != nil {
			continue
		}
		member, err := ms.arikawaState.Member(guildID, e.ID)
		if err != nil {
			continue
		}
		ms.checkAvatarChange(gcfg.GuildID, member.User.ID.String(), string(member.User.Avatar), member.User.Username)
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
	if ms.arikawaState != nil {
		gID, errG := discord.ParseSnowflake(guildID)
		uID, errU := discord.ParseSnowflake(userID)
		if errG == nil && errU == nil {
			if member, err := ms.arikawaState.Member(discord.GuildID(gID), discord.UserID(uID)); err == nil {
				if string(member.User.Avatar) == currentAvatar {
					changed = false
				}
			}
		}
	}

	if changed {
		ms.changeDebounce.record(changeKey, 100, 5*time.Minute)

		if ms.adapters != nil {
			if err := ms.adapters.EnqueueProcessAvatarChange(guildID, userID, username, currentAvatar); err != nil {
				if errors.Is(err, task.ErrDuplicateTask) {
					ms.logger.LogAttrs(context.Background(), log_slog.LevelInfo, "Avatar change task already enqueued (idempotency)", log_slog.String("guildID", guildID), log_slog.String("userID", userID))
				} else {
					ms.logger.LogAttrs(context.Background(), log_slog.LevelError, "Failed to enqueue avatar change task; falling back to synchronous processing", log_slog.String("guildID", guildID), log_slog.String("userID", userID), log_slog.Any("err", err))
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
		aw.logger.LogAttrs(context.Background(), log_slog.LevelError, "Failed to fetch current avatar from store", log_slog.String("guildID", guildID), log_slog.String("userID", userID), log_slog.Any("err", err))
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

	aw.logger.LogAttrs(context.Background(), log_slog.LevelInfo, "Avatar change detected and processing", log_slog.String("userID", userID), log_slog.String("guildID", guildID), log_slog.String("old_avatar", oldAvatar), log_slog.String("new_avatar", currentAvatar))

	emit := logpolicy.ShouldEmitLogEvent(aw.session, aw.configManager, logpolicy.LogEventAvatarChange, guildID)
	if !emit.Enabled {
		if emit.Reason == logpolicy.EmitReasonNoChannelConfigured {
			aw.logger.LogAttrs(context.Background(), log_slog.LevelError, "User activity log channel not configured; notification not sent", log_slog.String("guildID", guildID))
		} else {
			aw.logger.LogAttrs(context.Background(), log_slog.LevelDebug, "Avatar notification suppressed by policy", log_slog.String("guildID", guildID), log_slog.String("userID", userID), log_slog.String("reason", string(emit.Reason)))
		}
	} else {
		if err := aw.notifier.SendAvatarChangeNotification(emit.ChannelID, change); err != nil {
			aw.logger.LogAttrs(context.Background(), log_slog.LevelError, "Error sending notification", log_slog.String("channelID", emit.ChannelID), log_slog.String("userID", userID), log_slog.String("guildID", guildID), log_slog.Any("err", err))
		} else {
			aw.logger.LogAttrs(context.Background(), log_slog.LevelInfo, "Avatar notification sent successfully", log_slog.String("channelID", emit.ChannelID), log_slog.String("userID", userID), log_slog.String("guildID", guildID))
		}
	}

	if _, _, err := aw.store.UpsertAvatar(guildID, userID, currentAvatar, time.Now()); err != nil {
		aw.logger.LogAttrs(context.Background(), log_slog.LevelError, "Error saving avatar in store for guild", log_slog.String("guildID", guildID), log_slog.Any("err", err))
	}
}

func (aw *UserWatcher) getUsernameForNotification(guildID, userID string) string {
	if aw.arikawaState != nil {
		gID, errG := discord.ParseSnowflake(guildID)
		uID, errU := discord.ParseSnowflake(userID)
		if errG == nil && errU == nil {
			if member, err := aw.arikawaState.Member(discord.GuildID(gID), discord.UserID(uID)); err == nil {
				if member.Nick != "" {
					return member.Nick
				}
				if member.User.Username != "" {
					return member.User.Username
				}
			}
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
