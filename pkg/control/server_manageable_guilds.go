package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

var errBotGuildIDsProviderUnavailable = errors.New("bot guild ids provider unavailable")

type guildAccessLevel string

const (
	guildAccessLevelRead  guildAccessLevel = "read"
	guildAccessLevelWrite guildAccessLevel = "write"
)

type accessibleGuildCacheEntry struct {
	guilds    []accessibleGuildResponse
	expiresAt time.Time
}

type accessibleGuildResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Icon        string           `json:"icon,omitempty"`
	Owner       bool             `json:"owner"`
	Permissions int64            `json:"permissions"`
	AccessLevel guildAccessLevel `json:"access_level"`
}

func (s *Server) handleDiscordOAuthAccessibleGuilds(w http.ResponseWriter, r *http.Request) {
	s.handleDiscordOAuthGuildAccessList(w, r, false)
}

func (s *Server) handleDiscordOAuthManageableGuilds(w http.ResponseWriter, r *http.Request) {
	s.handleDiscordOAuthGuildAccessList(w, r, true)
}

func (s *Server) handleDiscordOAuthGuildAccessList(
	w http.ResponseWriter,
	r *http.Request,
	writeOnly bool,
) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	oauth, ok := s.requireDiscordOAuth(w)
	if !ok {
		return
	}

	session, err := oauth.sessionFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), defaultAccessibleGuildsQuery)
	defer cancel()

	accessible, err := s.resolveAccessibleGuilds(ctx, oauth, session)
	if err != nil {
		status := statusForAccessibleGuildsError(err)
		message := "failed to resolve accessible guilds"
		if status == http.StatusUnauthorized {
			message = "oauth session requires re-authentication"
		}
		log.ApplicationLogger().Error(
			"Failed to resolve accessible guilds",
			"operation", "control.oauth.guild_access.resolve",
			"userID", session.User.ID,
			"writeOnly", writeOnly,
			"err", err,
		)
		http.Error(w, message, status)
		return
	}

	if writeOnly {
		accessible = filterAccessibleGuildsByLevel(accessible, guildAccessLevelWrite)
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"guilds": accessible,
		"count":  len(accessible),
	})
}

func (s *Server) authorizeGuildAccess(
	w http.ResponseWriter,
	r *http.Request,
	auth requestAuthorization,
	guildID string,
) bool {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		http.Error(w, "guild_id is required", http.StatusBadRequest)
		return false
	}

	switch auth.mode {
	case requestAuthModeBearer:
		// Fixed bearer token remains available for trusted internal automation.
		return true
	case requestAuthModeDiscordOAuthSession:
		requiredAccess := requiredGuildAccessLevel(r.Method)
		ctx, cancel := context.WithTimeout(r.Context(), defaultAccessibleGuildsQuery)
		defer cancel()

		accessible, err := s.resolveAccessibleGuilds(ctx, s.discordOAuth, auth.oauthSession)
		if err != nil {
			status := statusForAccessibleGuildsError(err)
			message := "failed to authorize guild access"
			if status == http.StatusUnauthorized {
				message = "oauth session requires re-authentication"
			}
			log.ApplicationLogger().Error(
				"Failed to authorize guild route access",
				"operation", "control.guild_routes.authorize_guild",
				"userID", auth.oauthSession.User.ID,
				"guildID", guildID,
				"requiredAccess", requiredAccess,
				"err", err,
			)
			http.Error(w, message, status)
			return false
		}

		for _, guild := range accessible {
			if strings.TrimSpace(guild.ID) != guildID {
				continue
			}
			if guildAccessIncludes(guild.AccessLevel, requiredAccess) {
				return true
			}

			log.ApplicationLogger().Warn(
				"Guild route access denied",
				"operation", "control.guild_routes.authorize_guild",
				"userID", auth.oauthSession.User.ID,
				"guildID", guildID,
				"reason", "insufficient dashboard access level",
				"requiredAccess", requiredAccess,
				"actualAccess", guild.AccessLevel,
			)
			http.Error(w, "forbidden", http.StatusForbidden)
			return false
		}

		log.ApplicationLogger().Warn(
			"Guild route access denied",
			"operation", "control.guild_routes.authorize_guild",
			"userID", auth.oauthSession.User.ID,
			"guildID", guildID,
			"reason", "guild not accessible by authenticated user",
			"requiredAccess", requiredAccess,
		)
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	default:
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return false
	}
}

func requiredGuildAccessLevel(method string) guildAccessLevel {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return guildAccessLevelRead
	default:
		return guildAccessLevelWrite
	}
}

func guildAccessIncludes(actual, required guildAccessLevel) bool {
	if required == guildAccessLevelRead {
		return actual == guildAccessLevelRead || actual == guildAccessLevelWrite
	}
	return actual == guildAccessLevelWrite
}

func (s *Server) resolveAccessibleGuilds(
	ctx context.Context,
	oauth *discordOAuthProvider,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	if oauth == nil {
		return nil, fmt.Errorf("resolve accessible guilds: oauth provider is nil")
	}
	if cached, ok := s.cachedAccessibleGuilds(session.ID, time.Now().UTC()); ok {
		return cached, nil
	}

	botGuildSet, err := s.resolveBotGuildIDSet(ctx)
	if err != nil {
		return nil, err
	}

	freshSession, err := oauth.ensureFreshSessionAccessToken(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("resolve accessible guilds: refresh oauth access token: %w", err)
	}

	userGuilds, err := oauth.fetchUserGuilds(ctx, freshSession.AccessToken, freshSession.TokenType)
	if err != nil {
		return nil, fmt.Errorf("resolve accessible guilds: fetch user guilds: %w", err)
	}

	out := make([]accessibleGuildResponse, 0, len(userGuilds))
	for _, guild := range userGuilds {
		guildID := strings.TrimSpace(guild.ID)
		if guildID == "" {
			continue
		}
		if _, ok := botGuildSet[guildID]; !ok {
			continue
		}

		accessLevel, ok := s.resolveGuildAccessLevel(guild, session.User.ID)
		if !ok {
			continue
		}

		out = append(out, accessibleGuildResponse{
			ID:          guildID,
			Name:        strings.TrimSpace(guild.Name),
			Icon:        strings.TrimSpace(guild.Icon),
			Owner:       guild.Owner,
			Permissions: guild.Permissions,
			AccessLevel: accessLevel,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(out[i].Name))
		lj := strings.ToLower(strings.TrimSpace(out[j].Name))
		if li == lj {
			return out[i].ID < out[j].ID
		}
		return li < lj
	})

	s.storeAccessibleGuilds(session, out, time.Now().UTC())
	return out, nil
}

func (s *Server) resolveManageableGuilds(
	ctx context.Context,
	oauth *discordOAuthProvider,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	accessible, err := s.resolveAccessibleGuilds(ctx, oauth, session)
	if err != nil {
		return nil, err
	}
	return filterAccessibleGuildsByLevel(accessible, guildAccessLevelWrite), nil
}

func (s *Server) resolveGuildAccessLevel(
	guild discordOAuthGuild,
	userID string,
) (guildAccessLevel, bool) {
	if isGuildManageableByUser(guild) {
		return guildAccessLevelWrite, true
	}

	if s == nil || s.configManager == nil {
		return "", false
	}

	guildID := strings.TrimSpace(guild.ID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return "", false
	}

	guildConfig := s.configManager.GuildConfig(guildID)
	if guildConfig == nil {
		return "", false
	}
	if len(guildConfig.Roles.DashboardRead) == 0 && len(guildConfig.Roles.DashboardWrite) == 0 {
		return "", false
	}

	session, err := s.discordSessionForGuild(guildID)
	if err != nil || session == nil {
		return "", false
	}

	member, err := resolveGuildMemberFromDiscordSession(session, guildID, userID)
	if err != nil || member == nil {
		return "", false
	}

	memberRoleSet := make(map[string]struct{}, len(member.Roles))
	for _, roleID := range member.Roles {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		memberRoleSet[roleID] = struct{}{}
	}

	if matchesAnyRole(memberRoleSet, guildConfig.Roles.DashboardWrite) {
		return guildAccessLevelWrite, true
	}
	if matchesAnyRole(memberRoleSet, guildConfig.Roles.DashboardRead) {
		return guildAccessLevelRead, true
	}

	return "", false
}

func matchesAnyRole(memberRoleSet map[string]struct{}, roleIDs []string) bool {
	if len(memberRoleSet) == 0 || len(roleIDs) == 0 {
		return false
	}
	for _, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		if _, ok := memberRoleSet[roleID]; ok {
			return true
		}
	}
	return false
}

func filterAccessibleGuildsByLevel(
	guilds []accessibleGuildResponse,
	level guildAccessLevel,
) []accessibleGuildResponse {
	if len(guilds) == 0 {
		return nil
	}

	filtered := make([]accessibleGuildResponse, 0, len(guilds))
	for _, guild := range guilds {
		if guild.AccessLevel != level {
			continue
		}
		filtered = append(filtered, guild)
	}
	return filtered
}

func (s *Server) cachedAccessibleGuilds(
	sessionID string,
	now time.Time,
) ([]accessibleGuildResponse, bool) {
	if s == nil || s.accessibleGuildsTTL <= 0 {
		return nil, false
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, false
	}

	s.accessibleGuildsMu.RLock()
	entry, ok := s.accessibleGuilds[sessionID]
	s.accessibleGuildsMu.RUnlock()
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.IsZero() && !now.Before(entry.expiresAt) {
		s.accessibleGuildsMu.Lock()
		delete(s.accessibleGuilds, sessionID)
		s.accessibleGuildsMu.Unlock()
		return nil, false
	}
	return cloneAccessibleGuildResponses(entry.guilds), true
}

func (s *Server) storeAccessibleGuilds(
	session discordOAuthSession,
	guilds []accessibleGuildResponse,
	now time.Time,
) {
	if s == nil || s.accessibleGuildsTTL <= 0 {
		return
	}

	sessionID := strings.TrimSpace(session.ID)
	if sessionID == "" {
		return
	}

	expiresAt := now.Add(s.accessibleGuildsTTL)
	if !session.ExpiresAt.IsZero() && session.ExpiresAt.Before(expiresAt) {
		expiresAt = session.ExpiresAt
	}

	s.accessibleGuildsMu.Lock()
	s.accessibleGuilds[sessionID] = accessibleGuildCacheEntry{
		guilds:    cloneAccessibleGuildResponses(guilds),
		expiresAt: expiresAt,
	}
	s.accessibleGuildsMu.Unlock()
}

func cloneAccessibleGuildResponses(
	guilds []accessibleGuildResponse,
) []accessibleGuildResponse {
	if len(guilds) == 0 {
		return nil
	}

	out := make([]accessibleGuildResponse, len(guilds))
	copy(out, guilds)
	return out
}

func (s *Server) resolveBotGuildIDSet(ctx context.Context) (map[string]struct{}, error) {
	bindings, err := s.resolveBotGuildBindings(ctx)
	if err != nil {
		return nil, err
	}

	set := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		trimmed := strings.TrimSpace(binding.GuildID)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set, nil
}

func (s *Server) resolveBotGuildBindings(ctx context.Context) ([]BotGuildBinding, error) {
	if s == nil || s.botGuildBindings == nil {
		if s != nil && s.botGuildIDsProvider != nil {
			ids, err := s.botGuildIDsProvider(ctx)
			if err != nil {
				return nil, fmt.Errorf("resolve bot guild bindings: %w", err)
			}
			out := make([]BotGuildBinding, 0, len(ids))
			for _, guildID := range ids {
				out = append(out, BotGuildBinding{GuildID: guildID})
			}
			return out, nil
		}
		return nil, errBotGuildIDsProviderUnavailable
	}

	bindings, err := s.botGuildBindings(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve bot guild bindings: %w", err)
	}
	return bindings, nil
}

func isGuildManageableByUser(guild discordOAuthGuild) bool {
	if guild.Owner {
		return true
	}
	if guild.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		return true
	}
	if guild.Permissions&discordgo.PermissionManageGuild == discordgo.PermissionManageGuild {
		return true
	}
	return false
}

func statusForAccessibleGuildsError(err error) int {
	switch {
	case errors.Is(err, errBotGuildIDsProviderUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, errGuildDiscoveryRequired):
		return http.StatusNotFound
	case errors.Is(err, errDiscordOAuthSessionReauthenticationRequired):
		return http.StatusUnauthorized
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return http.StatusGatewayTimeout
	default:
		return http.StatusBadGateway
	}
}

func statusForManageableGuildsError(err error) int {
	return statusForAccessibleGuildsError(err)
}
