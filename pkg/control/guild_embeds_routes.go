package control

import (
	"encoding/json"
	"errors"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"log/slog"
	"net/http"
)

func (s *Server) handleGuildEmbedsRoutes(w http.ResponseWriter, r *http.Request, guildID string, tail []string, auth requestAuthorization) {
	if len(tail) == 1 && tail[0] == "embeds" {
		switch r.Method {
		case http.MethodGet:
			if !s.authorizeGuildControlAccess(w, r, auth, guildID, guildAccessLevelRead) {
				return
			}
			s.handleGuildEmbedsList(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if len(tail) == 2 && tail[0] == "embeds" {
		key := tail[1]
		switch r.Method {
		case http.MethodGet:
			if !s.authorizeGuildControlAccess(w, r, auth, guildID, guildAccessLevelRead) {
				return
			}
			s.handleGuildEmbedGet(w, r, guildID, key)
		case http.MethodPut:
			if !s.authorizeGuildControlAccess(w, r, auth, guildID, guildAccessLevelWrite) {
				return
			}
			s.handleGuildEmbedPut(w, r, guildID, key)
		case http.MethodDelete:
			if !s.authorizeGuildControlAccess(w, r, auth, guildID, guildAccessLevelWrite) {
				return
			}
			s.handleGuildEmbedDelete(w, r, guildID, key)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleGuildEmbedsList(w http.ResponseWriter, r *http.Request, guildID string) {
	embeds, err := s.configManager.CustomEmbeds(guildID)
	if err != nil {
		if errors.Is(err, files.ErrInvalidCustomEmbedInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.log().LogAttrs(r.Context(), slog.LevelError, "failed to get custom embeds", slog.String("guild_id", guildID), slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if embeds == nil {
		embeds = []files.CustomEmbedConfig{}
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log(), http.StatusOK, embeds)
}

func (s *Server) handleGuildEmbedGet(w http.ResponseWriter, r *http.Request, guildID, key string) {
	embed, err := s.configManager.CustomEmbed(guildID, key)
	if err != nil {
		if errors.Is(err, files.ErrCustomEmbedNotFound) {
			http.Error(w, "embed not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, files.ErrInvalidCustomEmbedInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.log().LogAttrs(r.Context(), slog.LevelError, "failed to get custom embed", slog.String("guild_id", guildID), slog.String("key", key), slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, embed)
}

func (s *Server) handleGuildEmbedPut(w http.ResponseWriter, r *http.Request, guildID, key string) {
	r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
	var input files.CustomEmbedConfig
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if input.Key != "" && input.Key != key {
		http.Error(w, "key in body does not match url", http.StatusBadRequest)
		return
	}
	input.Key = key

	if err := s.configManager.SetCustomEmbedProperties(guildID, key, input); err != nil {
		if errors.Is(err, files.ErrInvalidCustomEmbedInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.log().LogAttrs(r.Context(), slog.LevelError, "failed to set custom embed properties", slog.String("guild_id", guildID), slog.String("key", key), slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Always sync the fields.
	if err := s.configManager.SetCustomEmbedFields(guildID, key, input.Fields); err != nil {
		if errors.Is(err, files.ErrInvalidCustomEmbedInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.log().LogAttrs(r.Context(), slog.LevelError, "failed to set custom embed fields", slog.String("guild_id", guildID), slog.String("key", key), slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	embed, err := s.configManager.CustomEmbed(guildID, key)
	if err != nil {
		s.log().LogAttrs(r.Context(), slog.LevelError, "failed to retrieve custom embed after update", slog.String("guild_id", guildID), slog.String("key", key), slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, embed)
}

func (s *Server) handleGuildEmbedDelete(w http.ResponseWriter, r *http.Request, guildID, key string) {
	deleted, err := s.configManager.DeleteCustomEmbed(guildID, key)
	if err != nil {
		if errors.Is(err, files.ErrCustomEmbedNotFound) {
			http.Error(w, "embed not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, files.ErrInvalidCustomEmbedInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.log().LogAttrs(r.Context(), slog.LevelError, "failed to delete custom embed", slog.String("guild_id", guildID), slog.String("key", key), slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, deleted)
}
