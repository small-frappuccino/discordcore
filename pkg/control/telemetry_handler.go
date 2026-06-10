package control

import (
	"encoding/json"
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

type UITelemetryPayload struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
	Path   string  `json:"path"`
}

func (s *Server) handleUITelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload UITelemetryPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	log.ApplicationLogger().Info("UI Performance Metric",
		"source", "frontend",
		"metric", payload.Metric,
		"value", payload.Value,
		"path", payload.Path,
	)
	w.WriteHeader(http.StatusNoContent)
}

type UILogPayload struct {
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Path    string                 `json:"path"`
	Context map[string]interface{} `json:"context,omitempty"`
}

func (s *Server) handleUILogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload UILogPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	logger := log.ApplicationLogger()
	args := []interface{}{
		"source", "frontend",
		"path", payload.Path,
	}
	for k, v := range payload.Context {
		args = append(args, k, v)
	}

	switch payload.Level {
	case "error":
		logger.Error(payload.Message, args...)
	case "warn":
		logger.Warn(payload.Message, args...)
	default:
		logger.Info(payload.Message, args...)
	}

	w.WriteHeader(http.StatusNoContent)
}
