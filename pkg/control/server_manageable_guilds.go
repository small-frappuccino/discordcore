package control

import (
	"context"
	"net/http"
)

func (s *Server) handleDiscordOAuthAccessibleGuilds(w http.ResponseWriter, r *http.Request) {
	s.oauthControl().handleGuildAccessList(w, r, false)
}

func (s *Server) handleDiscordOAuthManageableGuilds(w http.ResponseWriter, r *http.Request) {
	s.oauthControl().handleGuildAccessList(w, r, true)
}

func (s *Server) resolveAccessibleGuilds(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return s.oauthControl().resolveAccessibleGuilds(ctx, session)
}

func (s *Server) resolveManageableGuilds(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return s.oauthControl().resolveManageableGuilds(ctx, session)
}

func (s *Server) invalidateAccessibleGuildsCache() {
	s.oauthControl().invalidateAccessibleGuildsCache()
}

func (s *Server) resolveBotGuildBindings(ctx context.Context) ([]BotGuildBinding, error) {
	if s == nil || s.botGuildSource == nil {
		return nil, errBotGuildIDsProviderUnavailable
	}
	return s.botGuildSource.Bindings(ctx)
}

func statusForManageableGuildsError(err error) int {
	return statusForAccessibleGuildsError(err)
}
