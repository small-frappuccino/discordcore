package app

import (
	"fmt"
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/cmd"
)

// Middleware defines a chainable interceptor for CommandHandlers.
type Middleware func(next cmd.CommandHandler) cmd.CommandHandler

// Chain builds a middleware chain into a single CommandHandler.
func Chain(handler cmd.CommandHandler, middlewares ...Middleware) cmd.CommandHandler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// RateLimitMiddleware enforces basic rate limiting (stub for now).
func RateLimitMiddleware() Middleware {
	return func(next cmd.CommandHandler) cmd.CommandHandler {
		return func(ctx *cmd.Context) error {
			// In O(1) routing we do not block unless rate limited.
			slog.Debug("RateLimitMiddleware evaluating request", slog.String("user", ctx.UserID.String()))
			return next(ctx)
		}
	}
}

// PermissionsMiddleware enforces that the feature is enabled in the config.
func PermissionsMiddleware(feature string) Middleware {
	return func(next cmd.CommandHandler) cmd.CommandHandler {
		return func(ctx *cmd.Context) error {
			cfgProv := ctx.DI.ConfigProvider()
			if cfgProv != nil && ctx.GuildID.IsValid() {
				enabled, _ := cfgProv.Config().ResolveFeatures(ctx.GuildID.String()).Lookup(feature)
				if !enabled {
					msg := fmt.Sprintf("Feature %s is disabled.", feature)
					return ctx.RespondMessage(msg)
				}
			}
			return next(ctx)
		}
	}
}
