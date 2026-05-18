package control

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/util"
)

// LiveHealthSnapshot is the JSON payload /v1/health/live returns. It is
// intentionally minimal: an external poller (UptimeRobot, cron, another
// VM running curl, etc.) only needs to confirm that the process is up,
// reachable, and serving HTTP. Anything richer belongs on the subsystem
// health endpoints (/v1/health/qotd, /v1/health/moderation), which expose
// internal counters.
//
// The fields are stable strings/integers so external poller scripts can
// `jq` them without breaking when new subsystem metrics are added.
type LiveHealthSnapshot struct {
	Status        string `json:"status"`
	App           string `json:"app"`
	AppVersion    string `json:"app_version,omitempty"`
	CoreVersion   string `json:"core_version"`
	BotUser       string `json:"bot_user,omitempty"`
	StartedAt     string `json:"started_at"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// handleLiveHealthRoute serves the GET /v1/health/live liveness probe.
//
// This endpoint exists to be polled by an external supervisor (a cron on
// another machine, UptimeRobot, etc.) so silent crashes — including
// runtime.throw paths that bypass deferred logging — are detected and
// surfaced even when nothing inside the process can report them.
//
// Auth: same gate as the rest of /v1/*. The operator configures the
// poller with the bearer token; rejecting unauthenticated requests
// prevents drive-by fingerprinting of bot identity and uptime.
//
// 200 OK with the snapshot whenever the HTTP server is reachable. There
// is no failure-mode branch — the only way this endpoint does not return
// 200 is if the process or socket is unreachable, which is precisely the
// signal the external poller needs.
func (s *Server) handleLiveHealthRoute(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
		return
	}

	startedAt := s.startedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	uptime := time.Since(startedAt)
	if uptime < 0 {
		uptime = 0
	}

	snapshot := LiveHealthSnapshot{
		Status:        "ok",
		App:           strings.TrimSpace(util.ConfiguredAppName),
		AppVersion:    strings.TrimSpace(util.AppVersion),
		CoreVersion:   util.DiscordCoreVersion,
		BotUser:       strings.TrimSpace(util.DiscordBotName),
		StartedAt:     startedAt.UTC().Format(time.RFC3339),
		UptimeSeconds: int64(uptime.Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	// Response status header is already in flight; nothing recoverable.
	_ = json.NewEncoder(w).Encode(snapshot)
}
