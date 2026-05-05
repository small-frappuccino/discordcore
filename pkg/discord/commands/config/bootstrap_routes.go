package config

import (
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

var dormantGuildBootstrapRoutesByDomain = map[string]map[string]struct{}{
	"": {
		"config get":                 {},
		"config list":                {},
		"config smoke_test":          {},
		"config commands_enabled":    {},
		"config command_channel":     {},
		"config allowed_role_add":    {},
		"config allowed_role_remove": {},
		"config allowed_role_list":   {},
	},
	files.BotDomainQOTD: {
		"config qotd_get":      {},
		"config qotd_channel":  {},
		"config qotd_enabled":  {},
		"config qotd_schedule": {},
	},
}

// DormantGuildBootstrapRouteDomain reports the bootstrap domain classification
// for a route that stays enabled while the full command surface is disabled.
func DormantGuildBootstrapRouteDomain(routeKey core.InteractionRouteKey) (string, bool) {
	if routeKey.Kind != core.InteractionKindSlash {
		return "", false
	}
	path := strings.TrimSpace(routeKey.Path)
	for domain, routes := range dormantGuildBootstrapRoutesByDomain {
		if _, ok := routes[path]; ok {
			return domain, true
		}
	}
	return "", false
}

// AllowsDormantGuildBootstrapRouteForDomain reports whether a route should stay
// usable for the requested domain while a guild is still dormant and the
// commands service is disabled.
func AllowsDormantGuildBootstrapRouteForDomain(domain string, routeKey core.InteractionRouteKey) bool {
	if routeKey.Kind != core.InteractionKindSlash {
		return false
	}
	routes := dormantGuildBootstrapRoutesByDomain[files.NormalizeBotDomain(domain)]
	if len(routes) == 0 {
		return false
	}
	_, ok := routes[strings.TrimSpace(routeKey.Path)]
	return ok
}

// AllowsDormantGuildBootstrapRoute reports whether a route should stay usable
// while a guild is still dormant and the commands service is disabled.
func AllowsDormantGuildBootstrapRoute(routeKey core.InteractionRouteKey) bool {
	_, ok := DormantGuildBootstrapRouteDomain(routeKey)
	return ok
}