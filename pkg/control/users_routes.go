package control

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type userPreferencesResponse struct {
	Theme    string `json:"theme"`
	Timezone string `json:"timezone"`
}

type userPreferencesPutRequest struct {
	Theme    string `json:"theme"`
	Timezone string `json:"timezone"`
}

func (s *Server) handleUserPreferencesGet(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}

	if auth.mode != requestAuthModeDiscordOAuthSession {
		http.Error(w, "user preferences require oauth session", http.StatusForbidden)
		return
	}

	userID := auth.oauthSession.User.ID
	prefs, err := s.storage.GetUserPreferences(r.Context(), userID)
	if err != nil {
		s.log().LogAttrs(r.Context(), slog.LevelError, "failed to get user preferences", slog.Any("err", err))
		http.Error(w, "failed to get user preferences", http.StatusInternalServerError)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, userPreferencesResponse{
		Theme:    prefs.Theme,
		Timezone: prefs.Timezone,
	})
}

func (s *Server) handleUserPreferencesPut(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}

	if auth.mode != requestAuthModeDiscordOAuthSession {
		http.Error(w, "user preferences require oauth session", http.StatusForbidden)
		return
	}

	var req userPreferencesPutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if req.Theme == "" {
		req.Theme = "system"
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}

	userID := auth.oauthSession.User.ID
	prefs := &storage.UserPreferences{
		UserID:   userID,
		Theme:    req.Theme,
		Timezone: req.Timezone,
	}

	if err := s.storage.UpdateUserPreferences(r.Context(), prefs); err != nil {
		s.log().LogAttrs(r.Context(), slog.LevelError, "failed to update user preferences", slog.Any("err", err))
		http.Error(w, "failed to update user preferences", http.StatusInternalServerError)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, userPreferencesResponse{
		Theme:    prefs.Theme,
		Timezone: prefs.Timezone,
	})
}
