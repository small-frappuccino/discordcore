package control

import (
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/messages"
)

type messagesHealthResolver struct {
	s *Server
}

func (res messagesHealthResolver) resolve() (any, string) {
	if res.s.health.messagesMetricsResolve == nil {
		return nil, "messages metrics not wired"
	}
	metrics := res.s.health.messagesMetricsResolve()
	if metrics == nil {
		return nil, "messages metrics not available"
	}
	provider, ok := metrics.(messages.SnapshotProvider)
	if !ok {
		return nil, "messages metrics not enabled (no SnapshotProvider attached)"
	}
	return provider.Snapshot(), ""
}

func (s *Server) handleMessagesHealthRoute(w http.ResponseWriter, r *http.Request) {
	serveHealthRoute(s, w, r, messagesHealthResolver{s: s})
}
