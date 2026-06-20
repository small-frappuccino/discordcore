package control

import (
	"log/slog"
	"net/http"
)

func (s *Server) handleGetGuildChannels(w http.ResponseWriter, r *http.Request) {
	slog.Debug("Granular inspection: Routing request for guild channels", slog.String("path", r.URL.Path))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"channels":[]}`))
}

func (s *Server) handleGetGuildRoles(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"roles":[]}`))
}
