package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

var errBotGuildIDsProviderUnavailable = errors.New("bot guild ids provider unavailable")

type manageableGuildResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Owner       bool   `json:"owner"`
	Permissions int64  `json:"permissions"`
}

func (s *Server) handleDiscordOAuthManageableGuilds(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(r.Context(), defaultManageableGuildsQuery)
	defer cancel()

	manageable, err := s.resolveManageableGuilds(ctx, oauth, session)
	if err != nil {
		status := statusForManageableGuildsError(err)
		message := "failed to resolve manageable guilds"
		if status == http.StatusUnauthorized {
			message = "oauth session requires re-authentication"
		}
		log.ApplicationLogger().Error(
			"Failed to resolve manageable guilds",
			"operation", "control.oauth.manageable_guilds.resolve",
			"userID", session.User.ID,
			"err", err,
		)
		http.Error(w, message, status)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"guilds": manageable,
		"count":  len(manageable),
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
		ctx, cancel := context.WithTimeout(r.Context(), defaultManageableGuildsQuery)
		defer cancel()

		manageable, err := s.resolveManageableGuilds(ctx, s.discordOAuth, auth.oauthSession)
		if err != nil {
			status := statusForManageableGuildsError(err)
			message := "failed to authorize guild access"
			if status == http.StatusUnauthorized {
				message = "oauth session requires re-authentication"
			}
			log.ApplicationLogger().Error(
				"Failed to authorize guild route access",
				"operation", "control.guild_routes.authorize_guild",
				"userID", auth.oauthSession.User.ID,
				"guildID", guildID,
				"err", err,
			)
			http.Error(w, message, status)
			return false
		}

		for _, guild := range manageable {
			if strings.TrimSpace(guild.ID) == guildID {
				return true
			}
		}

		log.ApplicationLogger().Warn(
			"Guild route access denied",
			"operation", "control.guild_routes.authorize_guild",
			"userID", auth.oauthSession.User.ID,
			"guildID", guildID,
			"reason", "guild not manageable by authenticated user",
		)
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	default:
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return false
	}
}

func (s *Server) resolveManageableGuilds(
	ctx context.Context,
	oauth *discordOAuthProvider,
	session discordOAuthSession,
) ([]manageableGuildResponse, error) {
	if oauth == nil {
		return nil, fmt.Errorf("resolve manageable guilds: oauth provider is nil")
	}

	botGuildSet, err := s.resolveBotGuildIDSet(ctx)
	if err != nil {
		return nil, err
	}

	freshSession, err := oauth.ensureFreshSessionAccessToken(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("resolve manageable guilds: refresh oauth access token: %w", err)
	}

	userGuilds, err := oauth.fetchUserGuilds(ctx, freshSession.AccessToken, freshSession.TokenType)
	if err != nil {
		return nil, fmt.Errorf("resolve manageable guilds: fetch user guilds: %w", err)
	}

	out := make([]manageableGuildResponse, 0, len(userGuilds))
	for _, guild := range userGuilds {
		guildID := strings.TrimSpace(guild.ID)
		if guildID == "" {
			continue
		}
		if _, ok := botGuildSet[guildID]; !ok {
			continue
		}
		if !isGuildManageableByUser(guild) {
			continue
		}

		out = append(out, manageableGuildResponse{
			ID:          guildID,
			Name:        strings.TrimSpace(guild.Name),
			Icon:        strings.TrimSpace(guild.Icon),
			Owner:       guild.Owner,
			Permissions: guild.Permissions,
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

	return out, nil
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

func statusForManageableGuildsError(err error) int {
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
