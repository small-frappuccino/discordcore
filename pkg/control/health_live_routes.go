package control

import (
	"net/http"
	"strings"
	"sync"
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
var liveHealthSnapshotPool = sync.Pool{
	New: func() any {
		return new(LiveHealthSnapshot)
	},
}

type liveHealthResolver struct {
	s    *Server
	snap *LiveHealthSnapshot
}

func (res liveHealthResolver) resolve() (any, string) {
	startedAt := res.s.startedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	uptime := time.Since(startedAt)
	if uptime < 0 {
		uptime = 0
	}
	botUser := strings.TrimSpace(files.DiscordBotName)
	botAvatarURL := ""
	if res.s.discordSession != nil {
		session, err := res.s.discordSession("")
		if err == nil && session != nil && session.State != nil && session.State.User != nil {
			if session.State.User.Username != "" {
				botUser = session.State.User.Username
			}
			botAvatarURL = session.State.User.AvatarURL("")
		}
	}

	res.snap.Status = "ok"
	res.snap.App = strings.TrimSpace(files.ConfiguredAppName)
	res.snap.AppVersion = strings.TrimSpace(files.AppVersion)
	res.snap.CoreVersion = files.DiscordCoreVersion
	res.snap.BotUser = botUser
	res.snap.BotAvatarURL = botAvatarURL
	res.snap.StartedAt = startedAt.UTC().Format(time.RFC3339)
	res.snap.UptimeSeconds = int64(uptime.Seconds())

	return res.snap, ""
}

func (s *Server) handleLiveHealthRoute(w http.ResponseWriter, r *http.Request) {
	snap := liveHealthSnapshotPool.Get().(*LiveHealthSnapshot)
	defer liveHealthSnapshotPool.Put(snap)

	serveHealthRoute(s, w, r, liveHealthResolver{s: s, snap: snap})
}
