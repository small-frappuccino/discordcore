package control

import (
	"encoding/json"
	"net/http"
)

// serveHealthRoute constructs an HTTP handler that evaluates a health resolver and securely serializes the operational state into JSON.
func serveHealthRoute[T any](resolver func() T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if resolver == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"offline"}`))
			return
		}

		data := resolver()

		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(data); err != nil {
			http.Error(w, `{"error":"internal marshal failure"}`, http.StatusInternalServerError)
		}
	}
}

func (s *Server) qotdHealthResolver() interface{} {
	if s.qotdService == nil {
		return map[string]string{"status": "offline"}
	}
	return map[string]string{"status": "ok"} // Simplified for now
}

func (s *Server) moderationHealthResolver() interface{} {
	if s.moderationMetrics == nil {
		return map[string]string{"status": "offline"}
	}
	return s.moderationMetrics
}

func (s *Server) cacheHealthResolver() interface{} {
	if s.cacheObservability == nil {
		return map[string]string{"status": "offline"}
	}
	return s.cacheObservability()
}
