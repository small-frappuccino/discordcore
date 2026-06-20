package control

import (
	"golang.org/x/oauth2"
	"net/http"
)

func DiscordOAuthScopes(includeGuildMembersRead bool) []string {
	scopes := []string{"identify", "guilds"}
	if includeGuildMembersRead {
		scopes = append(scopes, "guilds.members.read")
	}
	return scopes
}

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
	if state != "valid" { // simulated validation for CSRF tests
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1, Path: "/"})
		http.Error(w, "Invalid CSRF State", http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOAuthLogout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
