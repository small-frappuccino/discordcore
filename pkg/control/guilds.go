package control

import (
	"net/http"
)

func (s *Server) handleGetGuildChannels(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"channels":[]}`))
}

func (s *Server) handleGetGuildRoles(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"roles":[]}`))
}
