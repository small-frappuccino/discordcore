package control

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.ApplicationLogger().Error("Failed to encode control response", "status", status, "err", err)
	}
}
