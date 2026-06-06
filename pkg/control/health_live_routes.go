package control

import (
	"net/http"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
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
	BotAvatarURL  string `json:"bot_avatar_url,omitempty"`
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
// Auth and method gating are delegated to serveHealthRoute. There is no
// failure-mode branch — the only way this endpoint does not return 200
// is if the process or socket is unreachable, which is precisely the
// signal the external poller needs.
func (s *Server) handleLiveHealthRoute(w http.ResponseWriter, r *http.Request) {
	s.serveHealthRoute(w, r, func() (any, string) {
		startedAt := s.startedAt
		if startedAt.IsZero() {
			startedAt = time.Now().UTC()
		}
		uptime := time.Since(startedAt)
		if uptime < 0 {
			uptime = 0
		}
		botUser := strings.TrimSpace(files.DiscordBotName)
		botAvatarURL := ""
		if s.discordSession != nil {
			session, err := s.discordSession("")
			if err == nil && session != nil && session.State != nil && session.State.User != nil {
				if session.State.User.Username != "" {
					botUser = session.State.User.Username
				}
				botAvatarURL = session.State.User.AvatarURL("")
			}
		}

		return LiveHealthSnapshot{
			Status:        "ok",
			App:           strings.TrimSpace(files.ConfiguredAppName),
			AppVersion:    strings.TrimSpace(files.AppVersion),
			CoreVersion:   files.DiscordCoreVersion,
			BotUser:       botUser,
			BotAvatarURL:  botAvatarURL,
			StartedAt:     startedAt.UTC().Format(time.RFC3339),
			UptimeSeconds: int64(uptime.Seconds()),
		}, ""
	})
}
