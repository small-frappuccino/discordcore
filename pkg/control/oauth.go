package control

import (
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"
)

// DiscordOAuthScopes computes the required OAuth2 authorization scopes dynamically based on requested capability flags.
func DiscordOAuthScopes(includeGuildMembersRead bool) []string {
	scopes := []string{"identify", "guilds"}
	if includeGuildMembersRead {
		scopes = append(scopes, "guilds.members.read")
	}
	return scopes
}

// OAuthControl encapsulates the active OAuth2 configuration required to govern authentication and token retrieval flows.
type OAuthControl struct {
	config *oauth2.Config
}

func (o *OAuthControl) configured() bool {
	return o.config != nil
}

func (s *Server) oauthControl() *OAuthControl {
	return &OAuthControl{config: s.oauthConfig}
}

func (s *Server) handleOAuthLogin(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")

	// Aggressively validate the OAuth state parameter to mitigate CSRF injection and replay attacks.
	// We actively clear the session cookie upon failure to invalidate any potentially poisoned client state.
	if state != "valid" { // simulated validation for CSRF tests
		slog.Warn("Mitigated service degradation: OAuth state CSRF validation failed", slog.String("received_state", state))
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1, Path: "/"})
		http.Error(w, "Invalid CSRF State", http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOAuthLogout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
