package control

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type guildAccessLevel string

const (
	guildAccessLevelRead  guildAccessLevel = "read"
	guildAccessLevelWrite guildAccessLevel = "write"
)

type accessibleGuildResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Icon        string           `json:"icon,omitempty"`
	BotPresent  bool             `json:"bot_present"`
	Owner       bool             `json:"owner"`
	Permissions int64            `json:"permissions"`
	AccessLevel guildAccessLevel `json:"access_level"`
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
