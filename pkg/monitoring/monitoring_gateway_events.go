package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

// setupEventHandlers registra handlers do Discord.
func (ms *MonitoringService) setupEventHandlers() {
	rc := files.RuntimeConfig{}
	if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
		rc = scopedCfg.RuntimeConfig
	}
	ms.setupEventHandlersFromRuntimeConfig(rc)
}

// setupEventHandlersFromRuntimeConfig registers handlers based on the provided runtime config.
// This is used both at startup and for hot-apply.
func (ms *MonitoringService) setupEventHandlersFromRuntimeConfig(rc files.RuntimeConfig) {
	state := ms.workloadState(rc)

	if state.presenceHandler {
		ms.eventHandlers = append(ms.eventHandlers, ms.session.AddHandler(ms.handlePresenceUpdate))
	}
	if state.memberUpdateHandler {
		ms.eventHandlers = append(ms.eventHandlers, ms.session.AddHandler(ms.handleMemberUpdate))
	}

	if state.userUpdateHandler {
		ms.eventHandlers = append(ms.eventHandlers, ms.session.AddHandler(ms.handleUserUpdate))
	}
	ms.eventHandlers = append(ms.eventHandlers,
		ms.session.AddHandler(ms.handleGuildCreate),
		ms.session.AddHandler(ms.handleGuildUpdate),
	)
	if !state.presenceHandler && !state.memberUpdateHandler && !state.userUpdateHandler {
		ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "🛑 User and presence handlers are disabled by effective runtime/features")
	}
	if state.botPermMirrorHandlers {
		ms.eventHandlers = append(ms.eventHandlers,
			ms.session.AddHandler(ms.handleRoleUpdateForBotPermMirroring),
			ms.session.AddHandler(ms.handleRoleCreateForBotPermMirroring),
		)
	}
}

// removeEventHandlers removes all registered event handlers
func (ms *MonitoringService) removeEventHandlers() {
	for _, h := range ms.eventHandlers {
		if h != nil {
			h()
		}
	}
	ms.eventHandlers = nil
}

// ensureGuildsListed adds minimal guild entries to discordcore.json
// for all guilds present in the session but missing from the configuration.
func (ms *MonitoringService) ensureGuildsListed() {
	if ms.session == nil || ms.session.State == nil {
		return
	}

	for _, g := range ms.session.State.Guilds {
		if g == nil || g.ID == "" {
			continue
		}
		if ms.configManager.GuildConfig(g.ID) == nil {
			if err := ms.configManager.EnsureMinimalGuildConfig(g.ID); err != nil {
				ms.logger.LogAttrs(context.Background(), slog.LevelError, "Error adding minimal dormant guild entry", slog.String("guildID", g.ID), slog.Any("err", err))
			} else {
				ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "📘 Guild listed in config with disabled defaults", slog.String("guildID", g.ID))
			}
		}
	}
}

func (ms *MonitoringService) handleGuildCreate(s *discordgo.Session, e *discordgo.GuildCreate) {
	if e == nil {
		return
	}
	guildID := e.ID
	if guildID == "" {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_create",
		slog.String("guildID", guildID),
	)
	defer done()

	if ms.configManager.GuildConfig(guildID) == nil {
		if err := ms.configManager.EnsureMinimalGuildConfig(guildID); err != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelError, "Error adding dormant guild entry for new guild", slog.String("guildID", guildID), slog.Any("err", err))
			return
		}
		ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "🆕 New guild listed in config with disabled defaults", slog.String("guildID", guildID))
		ms.initializeGuildCache(guildID)
	}
}

// handleGuildUpdate updates the OwnerID cache when the server ownership changes.
func (ms *MonitoringService) handleGuildUpdate(s *discordgo.Session, e *discordgo.GuildUpdate) {
	if e == nil || e.Guild == nil || e.Guild.ID == "" {
		return
	}
	if !ms.handlesGuild(e.Guild.ID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_update",
		slog.String("guildID", e.Guild.ID),
	)
	defer done()

	if ms.store != nil {
		prev, ok, err := ms.store.GetGuildOwnerID(e.Guild.ID)
		if err != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelError,
				"Failed to read guild owner cache during guild update",
				slog.String("operation", "monitoring.handle_guild_update.get_owner"),
				slog.String("guildID", e.Guild.ID),
				slog.Any("err", err),
			)
		} else if ok && prev != e.Guild.OwnerID {
			ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "Guild owner changed", slog.String("guildID", e.Guild.ID), slog.String("from", prev), slog.String("to", e.Guild.OwnerID))
		}
		if err := ms.store.SetGuildOwnerID(e.Guild.ID, e.Guild.OwnerID); err != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelError,
				"Failed to persist guild owner cache during guild update",
				slog.String("operation", "monitoring.handle_guild_update.set_owner"),
				slog.String("guildID", e.Guild.ID),
				slog.String("ownerID", e.Guild.OwnerID),
				slog.Any("err", err),
			)
		}
	}
}

// handlePresenceUpdate processes presence updates (includes avatar).
func (ms *MonitoringService) handlePresenceUpdate(s *discordgo.Session, m *discordgo.PresenceUpdate) {
	if m.User == nil {
		return
	}
	if !ms.handlesGuild(m.GuildID) {
		return
	}
	if !ms.isFeatureBot(m.GuildID, "moderation") {
		return
	}
	if m.User.Username == "" {
		ms.logger.LogAttrs(context.Background(), slog.LevelDebug, "PresenceUpdate ignored (empty username)", slog.String("userID", m.User.ID), slog.String("guildID", m.GuildID))
		ms.handlePresenceWatch(m)
		return
	}

	done := perf.StartGatewayEvent(
		"presence_update",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.User.ID),
	)
	defer done()

	if runCtx := ms.currentRunCtx(); runCtx != nil {
		ms.markEvent(runCtx)
	}
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)
	ms.handlePresenceWatch(m)
}

func (ms *MonitoringService) handlePresenceWatch(m *discordgo.PresenceUpdate) {
	if m == nil || m.User == nil || ms.configManager == nil {
		return
	}
	cfg := ms.scopedConfig()
	if cfg == nil {
		return
	}
	rc := cfg.ResolveRuntimeConfig(m.GuildID)
	features := cfg.ResolveFeatures(m.GuildID)
	watchUserID := strings.TrimSpace(rc.PresenceWatchUserID)
	watchBot := rc.PresenceWatchBot
	if !features.PresenceWatch.User {
		watchUserID = ""
	}
	if !features.PresenceWatch.Bot {
		watchBot = false
	}
	if watchUserID == "" && !watchBot {
		return
	}

	userID := strings.TrimSpace(m.User.ID)
	if userID == "" {
		return
	}

	botID := ""
	if ms.session != nil && ms.session.State != nil && ms.session.State.User != nil {
		botID = ms.session.State.User.ID
	}
	isBotTarget := watchBot && botID != "" && userID == botID
	isUserTarget := watchUserID != "" && userID == watchUserID
	if !isBotTarget && !isUserTarget {
		return
	}

	snap := presenceSnapshot{
		Status:       normalizeStatus(m.Status),
		ClientStatus: normalizeClientStatus(m.ClientStatus),
	}

	prev, hasPrev, changed := ms.presence.observe(userID, snap)
	if !changed {
		return
	}

	statusChange := ""
	if hasPrev {
		if normalizeStatus(prev.Status) != normalizeStatus(snap.Status) {
			statusChange = fmt.Sprintf("%s -> %s", statusDisplay(prev.Status), statusDisplay(snap.Status))
		}
	} else {
		statusChange = statusDisplay(snap.Status)
	}

	deviceChanges := deviceStatusChanges(prev.ClientStatus, snap.ClientStatus)

	username := strings.TrimSpace(m.User.Username)
	if username == "" {
		username = userID
	}

	target := "user"
	if isBotTarget {
		target = "bot"
	}

	fields := []slog.Attr{
		slog.String("target", target),
		slog.String("userID", userID),
		slog.String("username", username),
		slog.String("status", presenceStatusLabel(snap.Status, snap.ClientStatus)),
		slog.String("devices", clientStatusSummary(snap.ClientStatus)),
	}
	if m.GuildID != "" {
		fields = append(fields, slog.String("guildID", m.GuildID))
	}
	if statusChange != "" {
		fields = append(fields, slog.String("status_change", statusChange))
	}
	if len(deviceChanges) > 0 {
		fields = append(fields, slog.String("device_changes", strings.Join(deviceChanges, "; ")))
	}

	ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "Presence watch update", fields...)
}

func presenceSnapshotEqual(a, b presenceSnapshot) bool {
	if normalizeStatus(a.Status) != normalizeStatus(b.Status) {
		return false
	}
	return clientStatusEqual(a.ClientStatus, b.ClientStatus)
}

func normalizeStatus(status discordgo.Status) discordgo.Status {
	if strings.TrimSpace(string(status)) == "" {
		return discordgo.StatusOffline
	}
	return status
}

func normalizeClientStatus(cs discordgo.ClientStatus) discordgo.ClientStatus {
	cs.Desktop = normalizeStatus(cs.Desktop)
	cs.Mobile = normalizeStatus(cs.Mobile)
	cs.Web = normalizeStatus(cs.Web)
	return cs
}

func clientStatusEqual(a, b discordgo.ClientStatus) bool {
	a = normalizeClientStatus(a)
	b = normalizeClientStatus(b)
	return a.Desktop == b.Desktop && a.Mobile == b.Mobile && a.Web == b.Web
}

func isActiveStatus(status discordgo.Status) bool {
	switch normalizeStatus(status) {
	case discordgo.StatusOnline, discordgo.StatusIdle, discordgo.StatusDoNotDisturb:
		return true
	default:
		return false
	}
}

func statusDisplay(status discordgo.Status) string {
	switch normalizeStatus(status) {
	case discordgo.StatusOnline:
		return "online"
	case discordgo.StatusIdle:
		return "idle (away)"
	case discordgo.StatusDoNotDisturb:
		return "dnd"
	case discordgo.StatusInvisible:
		return "invisible"
	case discordgo.StatusOffline:
		return "offline"
	default:
		return string(status)
	}
}

func presenceStatusLabel(status discordgo.Status, client discordgo.ClientStatus) string {
	label := statusDisplay(status)
	if isActiveStatus(client.Mobile) {
		label += " (mobile)"
	}
	return label
}

func clientStatusSummary(cs discordgo.ClientStatus) string {
	cs = normalizeClientStatus(cs)
	return fmt.Sprintf("desktop=%s mobile=%s web=%s", statusDisplay(cs.Desktop), statusDisplay(cs.Mobile), statusDisplay(cs.Web))
}

func deviceStatusChanges(prev, cur discordgo.ClientStatus) []string {
	prev = normalizeClientStatus(prev)
	cur = normalizeClientStatus(cur)
	changes := []string{}
	addChange := func(label string, prevStatus, curStatus discordgo.Status) {
		prevActive := isActiveStatus(prevStatus)
		curActive := isActiveStatus(curStatus)
		if prevActive != curActive {
			if curActive {
				changes = append(changes, fmt.Sprintf("%s entered (%s)", label, statusDisplay(curStatus)))
			} else {
				changes = append(changes, fmt.Sprintf("%s left", label))
			}
			return
		}
		if prevStatus != curStatus {
			changes = append(changes, fmt.Sprintf("%s status %s -> %s", label, statusDisplay(prevStatus), statusDisplay(curStatus)))
		}
	}

	addChange("desktop", prev.Desktop, cur.Desktop)
	addChange("mobile", prev.Mobile, cur.Mobile)
	addChange("web", prev.Web, cur.Web)
	return changes
}
