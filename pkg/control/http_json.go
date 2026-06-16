package control

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"log/slog"
)

// ErrDecodeJSONBody represents a JSON decoding failure in control endpoints.
var ErrDecodeJSONBody = errors.New("decode JSON body failed")

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return ErrDecodeJSONBody
	}
	return nil
}

func writeJSON[T any](w http.ResponseWriter, logger *slog.Logger, status int, payload T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		if logger != nil {
			logger.LogAttrs(context.Background(), slog.LevelError, "Failed to encode control response", slog.Int("status", status), slog.Any("err", err))
		} else {
			log.ApplicationLogger().LogAttrs(context.Background(), slog.LevelError, "Failed to encode control response", slog.Int("status", status), slog.Any("err", err))
		}
	}
}
