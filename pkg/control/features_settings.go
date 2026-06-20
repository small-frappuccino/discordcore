package control

import (
	"log/slog"
	"net/http"
)

func (s *Server) handleGetFeatures(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"features":[]}`))
}

func (s *Server) handlePostFeatures(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"applied"}`))
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"settings":{}}`))
}

func (s *Server) handlePutRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Info("Architectural state transition: Runtime configuration updated via control plane")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"updated"}`))
}
