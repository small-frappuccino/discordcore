package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
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
		ms.eventHandlers = append(ms.eventHandlers, ms.arikawaState.AddHandler(ms.handlePresenceUpdate))
	}
	if state.memberUpdateHandler {
		ms.eventHandlers = append(ms.eventHandlers, ms.arikawaState.AddHandler(ms.handleMemberUpdate))
	}

	if state.userUpdateHandler {
		ms.eventHandlers = append(ms.eventHandlers, ms.arikawaState.AddHandler(ms.handleUserUpdate))
	}
	ms.eventHandlers = append(ms.eventHandlers,
		ms.arikawaState.AddHandler(ms.handleGuildCreate),
		ms.arikawaState.AddHandler(ms.handleGuildUpdate),
	)
	if !state.presenceHandler && !state.memberUpdateHandler && !state.userUpdateHandler {
		ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "🛑 User and presence handlers are disabled by effective runtime/features")
	}
	if state.botPermMirrorHandlers {
		ms.eventHandlers = append(ms.eventHandlers,
			ms.arikawaState.AddHandler(ms.handleRoleUpdateForBotPermMirroring),
			ms.arikawaState.AddHandler(ms.handleRoleCreateForBotPermMirroring),
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

func (ms *MonitoringService) handleGuildCreate(e *gateway.GuildCreateEvent) {
	if e == nil {
		return
	}
	guildID := e.ID.String()
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
func (ms *MonitoringService) handleGuildUpdate(e *gateway.GuildUpdateEvent) {
	if e == nil || e.ID == 0 {
		return
	}
	guildID := e.ID.String()
	if !ms.handlesGuild(guildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_update",
		slog.String("guildID", guildID),
	)
	defer done()

	ownerID := e.OwnerID.String()
	if ms.store != nil {
		prev, ok, err := ms.store.GetGuildOwnerID(guildID)
		if err != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelError,
				"Failed to read guild owner cache during guild update",
				slog.String("operation", "monitoring.handle_guild_update.get_owner"),
				slog.String("guildID", guildID),
				slog.Any("err", err),
			)
		} else if ok && prev != ownerID {
			ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "Guild owner changed", slog.String("guildID", guildID), slog.String("from", prev), slog.String("to", ownerID))
		}
		if err := ms.store.SetGuildOwnerID(guildID, ownerID); err != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelError,
				"Failed to persist guild owner cache during guild update",
				slog.String("operation", "monitoring.handle_guild_update.set_owner"),
				slog.String("guildID", guildID),
				slog.String("ownerID", ownerID),
				slog.Any("err", err),
			)
		}
	}
}

// handlePresenceUpdate processes presence updates (includes avatar).
func (ms *MonitoringService) handlePresenceUpdate(e *gateway.PresenceUpdateEvent) {
	if e == nil {
		return
	}
	guildID := e.GuildID.String()
	if !ms.handlesGuild(guildID) {
		return
	}
	if !ms.isFeatureBot(guildID, "moderation") {
		return
	}
	if e.User.Username == "" {
		ms.logger.LogAttrs(context.Background(), slog.LevelDebug, "PresenceUpdate ignored (empty username)", slog.String("userID", e.User.ID.String()), slog.String("guildID", guildID))
		ms.handlePresenceWatch(e)
		return
	}

	done := perf.StartGatewayEvent(
		"presence_update",
		slog.String("guildID", guildID),
		slog.String("userID", e.User.ID.String()),
	)
	defer done()

	if runCtx := ms.currentRunCtx(); runCtx != nil {
		ms.markEvent(runCtx)
	}
	ms.checkAvatarChange(guildID, e.User.ID.String(), string(e.User.Avatar), e.User.Username)
	ms.handlePresenceWatch(e)
}

func (ms *MonitoringService) handlePresenceWatch(e *gateway.PresenceUpdateEvent) {
	if e == nil || ms.configManager == nil {
		return
	}
	guildID := e.GuildID.String()
	cfg := ms.scopedConfig()
	if cfg == nil {
		return
	}
	rc := cfg.ResolveRuntimeConfig(guildID)
	features := cfg.ResolveFeatures(guildID)
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

	userID := e.User.ID.String()
	if userID == "" {
		return
	}

	botID := ""
	if ms.arikawaState != nil {
		if me, err := ms.arikawaState.Me(); err == nil {
			botID = me.ID.String()
		}
	}
	isBotTarget := watchBot && botID != "" && userID == botID
	isUserTarget := watchUserID != "" && userID == watchUserID
	if !isBotTarget && !isUserTarget {
		return
	}

	snap := presenceSnapshot{
		Status:       normalizeStatus(string(e.Status)),
		ClientStatus: normalizeClientStatusArikawa(e.ClientStatus),
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

	username := strings.TrimSpace(e.User.Username)
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
	if guildID != "" {
		fields = append(fields, slog.String("guildID", guildID))
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

func normalizeStatus(status string) string {
	if strings.TrimSpace(status) == "" {
		return "offline"
	}
	return status
}

type clientStatus struct {
	Desktop string
	Mobile  string
	Web     string
}

func normalizeClientStatusArikawa(cs discord.ClientStatus) clientStatus {
	return clientStatus{
		Desktop: normalizeStatus(string(cs.Desktop)),
		Mobile:  normalizeStatus(string(cs.Mobile)),
		Web:     normalizeStatus(string(cs.Web)),
	}
}

func normalizeClientStatus(cs clientStatus) clientStatus {
	cs.Desktop = normalizeStatus(cs.Desktop)
	cs.Mobile = normalizeStatus(cs.Mobile)
	cs.Web = normalizeStatus(cs.Web)
	return cs
}

func clientStatusEqual(a, b clientStatus) bool {
	a = normalizeClientStatus(a)
	b = normalizeClientStatus(b)
	return a.Desktop == b.Desktop && a.Mobile == b.Mobile && a.Web == b.Web
}

func isActiveStatus(status string) bool {
	switch normalizeStatus(status) {
	case "online", "idle", "dnd":
		return true
	default:
		return false
	}
}

func statusDisplay(status string) string {
	switch normalizeStatus(status) {
	case "online":
		return "online"
	case "idle":
		return "idle (away)"
	case "dnd":
		return "dnd"
	case "invisible":
		return "invisible"
	case "offline":
		return "offline"
	default:
		return status
	}
}

func presenceStatusLabel(status string, client clientStatus) string {
	label := statusDisplay(status)
	if isActiveStatus(client.Mobile) {
		label += " (mobile)"
	}
	return label
}

func clientStatusSummary(cs clientStatus) string {
	cs = normalizeClientStatus(cs)
	return fmt.Sprintf("desktop=%s mobile=%s web=%s", statusDisplay(cs.Desktop), statusDisplay(cs.Mobile), statusDisplay(cs.Web))
}

func deviceStatusChanges(prev, cur clientStatus) []string {
	prev = normalizeClientStatus(prev)
	cur = normalizeClientStatus(cur)
	changes := []string{}
	addChange := func(label string, prevStatus, curStatus string) {
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
