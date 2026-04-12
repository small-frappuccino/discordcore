package control

import "net/http"

func (s *Server) handleDiscordOAuthLogin(w http.ResponseWriter, r *http.Request) {
	s.oauthControl().handleLogin(w, r)
}

func (s *Server) handleDiscordOAuthCallback(w http.ResponseWriter, r *http.Request) {
	s.oauthControl().handleCallback(w, r)
}

func (s *Server) handleDiscordOAuthMe(w http.ResponseWriter, r *http.Request) {
	s.oauthControl().handleMe(w, r)
}

func (s *Server) handleDiscordOAuthStatus(w http.ResponseWriter, r *http.Request) {
	s.oauthControl().handleStatus(w, r)
}

func (s *Server) handleDiscordOAuthLogout(w http.ResponseWriter, r *http.Request) {
	s.oauthControl().handleLogout(w, r)
}
