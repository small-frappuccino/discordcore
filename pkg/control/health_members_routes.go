package control

import (
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/members"
)

type membersHealthResolver struct {
	s *Server
}

func (res membersHealthResolver) resolve() (any, string) {
	if res.s.health.membersMetricsResolve == nil {
		return nil, "members metrics not wired"
	}
	metrics := res.s.health.membersMetricsResolve()
	if metrics == nil {
		return nil, "members metrics not available"
	}
	provider, ok := metrics.(members.SnapshotProvider)
	if !ok {
		return nil, "members metrics not enabled (no SnapshotProvider attached)"
	}
	return provider.Snapshot(), ""
}

func (s *Server) handleMembersHealthRoute(w http.ResponseWriter, r *http.Request) {
	serveHealthRoute(s, w, r, membersHealthResolver{s: s})
}
