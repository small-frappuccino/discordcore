package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var errBotGuildIDsProviderUnavailable = errors.New("bot guild ids provider unavailable")

type guildAccessLevel string

const (
	guildAccessLevelRead  guildAccessLevel = "read"
	guildAccessLevelWrite guildAccessLevel = "write"
)

type accessibleGuildCacheEntry struct {
	guilds    []cachedAccessibleGuild
	expiresAt time.Time
}

type cachedAccessibleGuild struct {
	guild      discordOAuthGuild
	botPresent bool
}

type accessibleGuildResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Icon        string           `json:"icon,omitempty"`
	BotPresent  bool             `json:"bot_present"`
	Owner       bool             `json:"owner"`
	Permissions int64            `json:"permissions"`
	AccessLevel guildAccessLevel `json:"access_level"`
}

func (s *Server) handleDiscordOAuthAccessibleGuilds(w http.ResponseWriter, r *http.Request) {
	s.oauthControl().handleGuildAccessList(w, r, false)
}

func (s *Server) handleDiscordOAuthManageableGuilds(w http.ResponseWriter, r *http.Request) {
	s.oauthControl().handleGuildAccessList(w, r, true)
}

func (s *Server) resolveAccessibleGuilds(
	ctx context.Context,
	oauth *discordOAuthProvider,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return s.oauthControlWithProvider(oauth).resolveAccessibleGuilds(ctx, session)
}

func (s *Server) resolveAccessibleGuildsFresh(
	ctx context.Context,
	oauth *discordOAuthProvider,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return s.oauthControlWithProvider(oauth).resolveAccessibleGuildsFresh(ctx, session)
}

func (s *Server) resolveAccessibleGuildsRefreshed(
	ctx context.Context,
	oauth *discordOAuthProvider,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return s.oauthControlWithProvider(oauth).resolveAccessibleGuildsRefreshed(ctx, session)
}

func (s *Server) resolveManageableGuilds(
	ctx context.Context,
	oauth *discordOAuthProvider,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return s.oauthControlWithProvider(oauth).resolveManageableGuilds(ctx, session)
}

func (s *Server) invalidateAccessibleGuildsCache() {
	s.oauthControl().invalidateAccessibleGuildsCache()
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

func requestWantsFreshGuildAccess(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("fresh"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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

func cloneCachedAccessibleGuilds(guilds []cachedAccessibleGuild) []cachedAccessibleGuild {
	if len(guilds) == 0 {
		return nil
	}

	out := make([]cachedAccessibleGuild, len(guilds))
	copy(out, guilds)
	return out
}

func cloneAccessibleGuildResponses(guilds []accessibleGuildResponse) []accessibleGuildResponse {
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

func shouldSuppressAccessibleGuildsRequestError(parent context.Context, err error) bool {
	if err == nil || parent == nil {
		return false
	}

	parentErr := parent.Err()
	if parentErr == nil {
		return false
	}

	return errors.Is(err, parentErr)
}

func statusForManageableGuildsError(err error) int {
	return statusForAccessibleGuildsError(err)
}
