package control

import (
	"encoding/json"
	"log/slog"
	"net/http"
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

	s.log().LogAttrs(r.Context(), slog.LevelInfo, "UI Performance Metric",
		slog.String("source", "frontend"),
		slog.String("metric", payload.Metric),
		slog.Float64("value", payload.Value),
		slog.String("path", payload.Path),
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

	logger := s.log()
	attrs := make([]slog.Attr, 0, 2+len(payload.Context))
	attrs = append(attrs, slog.String("source", "frontend"), slog.String("path", payload.Path))
	for k, v := range payload.Context {
		attrs = append(attrs, slog.Any(k, v))
	}

	var lvl slog.Level
	switch payload.Level {
	case "error":
		lvl = slog.LevelError
	case "warn":
		lvl = slog.LevelWarn
	default:
		lvl = slog.LevelInfo
	}

	logger.LogAttrs(r.Context(), lvl, payload.Message, attrs...)

	w.WriteHeader(http.StatusNoContent)
}
