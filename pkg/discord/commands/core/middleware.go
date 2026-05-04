package core

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
)

// InteractionHandlerFunc is the normalized execution function used by the
// unified interaction router after a route key has been resolved.
type InteractionHandlerFunc func(ctx *Context) error

// InteractionMiddleware wraps a normalized interaction execution using the
// resolved route key as common input.
type InteractionMiddleware func(routeKey InteractionRouteKey, next InteractionHandlerFunc) InteractionHandlerFunc

// UseMiddleware appends router middlewares in registration order.
func (cr *CommandRouter) UseMiddleware(middlewares ...InteractionMiddleware) {
	if cr == nil {
		return
	}
	for _, middleware := range middlewares {
		if middleware == nil {
			continue
		}
		cr.middlewares = append(cr.middlewares, middleware)
	}
}

func defaultInteractionMiddlewares(cr *CommandRouter) []InteractionMiddleware {
	if cr == nil {
		return nil
	}
	return []InteractionMiddleware{
		cr.telemetryMiddleware(),
		cr.errorMappingMiddleware(),
		cr.permissionGateMiddleware(),
		cr.ackPolicyMiddleware(),
	}
}

func (cr *CommandRouter) telemetryMiddleware() InteractionMiddleware {
	return func(routeKey InteractionRouteKey, next InteractionHandlerFunc) InteractionHandlerFunc {
		return func(ctx *Context) error {
			done := perf.StartGatewayEvent(
				"interaction_route",
				slog.String("kind", routeKey.Kind.String()),
				slog.String("path", routeKey.Path),
				slog.String("guildID", ctx.GuildID),
				slog.String("userID", ctx.UserID),
			)
			defer done()
			return next(ctx)
		}
	}
}

func (cr *CommandRouter) permissionGateMiddleware() InteractionMiddleware {
	return func(routeKey InteractionRouteKey, next InteractionHandlerFunc) InteractionHandlerFunc {
		if routeKey.Kind != InteractionKindSlash {
			return next
		}

		return func(ctx *Context) error {
			handler, exists := cr.lookupSlashHandler(ctx.RouteKey)
			if !exists {
				return next(ctx)
			}

			if handler.RequiresGuild() && ctx.GuildID == "" {
				slog.Warn("Command used outside of guild", "commandPath", ctx.RouteKey.Path)
				return NewCommandError("This command only works inside a server, so I'm keeping this failure private.", true)
			}

			if ctx.GuildConfig != nil && len(ctx.GuildConfig.Roles.Allowed) > 0 && !cr.permChecker.HasPermission(ctx.GuildID, ctx.UserID) {
				slog.Warn("User without allowed role tried to use command", "commandPath", ctx.RouteKey.Path)
				return NewCommandError("You don't have access to this command, so I'm keeping this reply private.", true)
			}

			if handler.RequiresPermissions() && !cr.permChecker.HasPermission(ctx.GuildID, ctx.UserID) {
				slog.Warn("User without permission tried to use command", "commandPath", ctx.RouteKey.Path)
				return NewCommandError("You don't have access to this command, so I'm keeping this reply private.", true)
			}

			return next(ctx)
		}
	}
}

func (cr *CommandRouter) errorMappingMiddleware() InteractionMiddleware {
	return func(routeKey InteractionRouteKey, next InteractionHandlerFunc) InteractionHandlerFunc {
		if routeKey.Kind != InteractionKindSlash {
			return next
		}

		return func(ctx *Context) error {
			err := next(ctx)
			if err == nil {
				return nil
			}

			slog.Error("Slash route failed", "commandPath", ctx.RouteKey.Path, "err", err)
			respondToSlashError(ctx, err)
			return nil
		}
	}
}

func (cr *CommandRouter) ackPolicyMiddleware() InteractionMiddleware {
	return func(routeKey InteractionRouteKey, next InteractionHandlerFunc) InteractionHandlerFunc {
		return func(ctx *Context) error {
			policy, exists := cr.lookupInteractionAckPolicy(routeKey)
			if !exists || !policy.requiresAck() {
				return next(ctx)
			}

			if err := applyInteractionAckPolicy(ctx, routeKey, policy); err != nil {
				return fmt.Errorf("ack interaction: %w", err)
			}

			return next(ctx)
		}
	}
}

func applyInteractionAckPolicy(ctx *Context, routeKey InteractionRouteKey, policy InteractionAckPolicy) error {
	if ctx == nil || ctx.Session == nil || ctx.Interaction == nil || !policy.requiresAck() {
		return nil
	}

	switch routeKey.Kind {
	case InteractionKindSlash:
		return NewResponseManager(ctx.Session).DeferResponse(ctx.Interaction, policy.Ephemeral)
	case InteractionKindComponent, InteractionKindModal:
		return ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
	default:
		return nil
	}
}

func respondToSlashError(ctx *Context, err error) {
	if ctx == nil || ctx.Session == nil || ctx.Interaction == nil || err == nil {
		return
	}

	if cmdErr, ok := err.(*CommandError); ok {
		builder := NewResponseBuilder(ctx.Session)
		if cmdErr.Ephemeral {
			builder = builder.Ephemeral()
		}
		builder.Error(ctx.Interaction, cmdErr.Message)
		return
	}

	NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, "An error occurred while executing the command. I'm keeping this reply private because this result is mainly for the person who ran the command.")
}

func chainInteractionMiddleware(routeKey InteractionRouteKey, final InteractionHandlerFunc, middlewares []InteractionMiddleware) InteractionHandlerFunc {
	if final == nil {
		return nil
	}

	wrapped := final
	for index := len(middlewares) - 1; index >= 0; index-- {
		middleware := middlewares[index]
		if middleware == nil {
			continue
		}
		wrapped = middleware(routeKey, wrapped)
	}

	return wrapped
}

func (cr *CommandRouter) executeRoute(ctx *Context, routeKey InteractionRouteKey, final InteractionHandlerFunc) error {
	if cr == nil || ctx == nil || final == nil {
		return nil
	}

	ctx.SetRouter(cr)
	ctx.RouteKey = routeKey

	handler := chainInteractionMiddleware(routeKey, final, cr.middlewares)
	if handler == nil {
		return nil
	}

	return handler(ctx)
}