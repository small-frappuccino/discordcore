package monitoring

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// ensureGuildsListed adds minimal guild entries to discordcore.json
// for all guilds present in the session but missing from the configuration.
// This should be called by the adapter during startup.
func (ms *MonitoringService) EnsureGuildsListed(guildIDs []string) {
	for _, guildID := range guildIDs {
		if guildID == "" {
			continue
		}
		if ms.configManager.GuildConfig(guildID) == nil {
			if err := ms.configManager.EnsureMinimalGuildConfig(guildID); err != nil {
				log.ErrorLoggerRaw().Error("Error adding minimal dormant guild entry", "guildID", guildID, "err", err)
			} else {
				log.ApplicationLogger().Info("📘 Guild listed in config with disabled defaults", "guildID", guildID)
			}
		}
	}
}

// HandleGuildCreate processes a new guild becoming available.
func (ms *MonitoringService) HandleGuildCreate(guildID string) {
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
			log.ErrorLoggerRaw().Error("Error adding dormant guild entry for new guild", "guildID", guildID, "err", err)
			return
		}
		log.ApplicationLogger().Info("🆕 New guild listed in config with disabled defaults", "guildID", guildID)
		ms.initializeGuildCache(guildID)
	}
}

// HandleGuildUpdate updates the OwnerID cache when the server ownership changes.
func (ms *MonitoringService) HandleGuildUpdate(guildID string, ownerID string) {
	if guildID == "" {
		return
	}
	if !ms.handlesGuild(guildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_update",
		slog.String("guildID", guildID),
	)
	defer done()

	if ms.store != nil {
		prev, ok, err := ms.store.GetGuildOwnerID(guildID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to read guild owner cache during guild update",
				"operation", "monitoring.handle_guild_update.get_owner",
				"guildID", guildID,
				"err", err,
			)
		} else if ok && prev != ownerID {
			log.ApplicationLogger().Info("Guild owner changed", "guildID", guildID, "from", prev, "to", ownerID)
		}
		if err := ms.store.SetGuildOwnerID(guildID, ownerID); err != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to persist guild owner cache during guild update",
				"operation", "monitoring.handle_guild_update.set_owner",
				"guildID", guildID,
				"ownerID", ownerID,
				"err", err,
			)
		}
	}
}

// PresenceUpdateSnapshot contains the pure data needed for a presence update.
type PresenceUpdateSnapshot struct {
	GuildID      string
	UserID       string
	Username     string
	Avatar       string
	Status       string
	ClientStatus ClientStatusSnapshot
}

type ClientStatusSnapshot struct {
	Desktop string
	Mobile  string
	Web     string
}

// HandlePresenceUpdate processes presence updates (includes avatar).
func (ms *MonitoringService) HandlePresenceUpdate(p PresenceUpdateSnapshot) {
	if !ms.handlesGuild(p.GuildID) {
		return
	}
	if !ms.isFeatureBot(p.GuildID, "moderation") {
		return
	}
	if p.Username == "" {
		log.ApplicationLogger().Debug("PresenceUpdate ignored (empty username)", "userID", p.UserID, "guildID", p.GuildID)
		ms.handlePresenceWatch(p)
		return
	}

	done := perf.StartGatewayEvent(
		"presence_update",
		slog.String("guildID", p.GuildID),
		slog.String("userID", p.UserID),
	)
	defer done()

	if runCtx := ms.currentRunCtx(); runCtx != nil {
		ms.markEvent(runCtx)
	}
	ms.checkAvatarChange(p.GuildID, p.UserID, p.Avatar, p.Username)
	ms.handlePresenceWatch(p)
}

func (ms *MonitoringService) handlePresenceWatch(p PresenceUpdateSnapshot) {
	if ms.configManager == nil {
		return
	}
	cfg := ms.scopedConfig()
	if cfg == nil {
		return
	}
	rc := cfg.ResolveRuntimeConfig(p.GuildID)
	features := cfg.ResolveFeatures(p.GuildID)
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

	userID := strings.TrimSpace(p.UserID)
	if userID == "" {
		return
	}

	// For bot checking, the adapter handles verifying if the userID is the bot's ID.
	// Since we decouple from session, the domain shouldn't need to fetch the BotID from the session.
	// We can add BotID to MonitoringService.
	botID := ms.botInstanceID // Approximate, or we can fetch it via another means.
	isBotTarget := watchBot && botID != "" && userID == botID
	isUserTarget := watchUserID != "" && userID == watchUserID
	if !isBotTarget && !isUserTarget {
		return
	}

	snap := presenceSnapshot{
		Status:       normalizeStatus(p.Status),
		ClientStatus: normalizeClientStatus(p.ClientStatus),
	}

	prev, hasPrev, changed := ms.presence.observe(userID, snap)
	if !changed {
		return
	}

	statusChange := ""
	if hasPrev {
		if prev.Status != snap.Status {
			statusChange = fmt.Sprintf("%s -> %s", statusDisplay(prev.Status), statusDisplay(snap.Status))
		}
	} else {
		statusChange = statusDisplay(snap.Status)
	}

	deviceChanges := deviceStatusChanges(prev.ClientStatus, snap.ClientStatus)

	username := strings.TrimSpace(p.Username)
	if username == "" {
		username = userID
	}

	target := "user"
	if isBotTarget {
		target = "bot"
	}

	fields := []any{
		"target", target,
		"userID", userID,
		"username", username,
		"status", presenceStatusLabel(snap.Status, snap.ClientStatus),
		"devices", clientStatusSummary(snap.ClientStatus),
	}
	if p.GuildID != "" {
		fields = append(fields, "guildID", p.GuildID)
	}
	if statusChange != "" {
		fields = append(fields, "status_change", statusChange)
	}
	if len(deviceChanges) > 0 {
		fields = append(fields, "device_changes", strings.Join(deviceChanges, "; "))
	}

	log.ApplicationLogger().Info("Presence watch update", fields...)
}

func presenceSnapshotEqual(a, b presenceSnapshot) bool {
	if a.Status != b.Status {
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

func normalizeClientStatus(cs ClientStatusSnapshot) ClientStatusSnapshot {
	cs.Desktop = normalizeStatus(cs.Desktop)
	cs.Mobile = normalizeStatus(cs.Mobile)
	cs.Web = normalizeStatus(cs.Web)
	return cs
}

func clientStatusEqual(a, b ClientStatusSnapshot) bool {
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

func presenceStatusLabel(status string, client ClientStatusSnapshot) string {
	label := statusDisplay(status)
	if isActiveStatus(client.Mobile) {
		label += " (mobile)"
	}
	return label
}

func clientStatusSummary(cs ClientStatusSnapshot) string {
	cs = normalizeClientStatus(cs)
	return fmt.Sprintf("desktop=%s mobile=%s web=%s", statusDisplay(cs.Desktop), statusDisplay(cs.Mobile), statusDisplay(cs.Web))
}

func deviceStatusChanges(prev, cur ClientStatusSnapshot) []string {
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
