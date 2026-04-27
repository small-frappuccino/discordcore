package config

import (
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

var dormantGuildBootstrapRoutes = map[string]struct{}{
	"config get":              {},
	"config list":             {},
	"config smoke_test":       {},
	"config commands_enabled": {},
	"config command_channel":  {},
	"config allowed_role_add": {},
	"config allowed_role_remove": {},
	"config allowed_role_list": {},
	"config qotd_channel":    {},
	"config qotd_enabled":    {},
	"config qotd_schedule":   {},
}

// AllowsDormantGuildBootstrapRoute reports whether a route should stay usable
// while a guild is still dormant and the commands service is disabled.
func AllowsDormantGuildBootstrapRoute(routeKey core.InteractionRouteKey) bool {
	if routeKey.Kind != core.InteractionKindSlash {
		return false
	}
	_, ok := dormantGuildBootstrapRoutes[strings.TrimSpace(routeKey.Path)]
	return ok
}