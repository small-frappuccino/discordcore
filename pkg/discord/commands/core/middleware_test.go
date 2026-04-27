package core

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestCommandRouterMiddlewareUsesRouteKeyPipeline(t *testing.T) {
	session, _ := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)

	var order []string
	router.UseMiddleware(
		func(routeKey InteractionRouteKey, next InteractionHandlerFunc) InteractionHandlerFunc {
			return func(ctx *Context) error {
				order = append(order, "mw1-before:"+routeKey.Path)
				err := next(ctx)
				order = append(order, "mw1-after:"+ctx.RouteKey.Path)
				return err
			}
		},
		func(routeKey InteractionRouteKey, next InteractionHandlerFunc) InteractionHandlerFunc {
			return func(ctx *Context) error {
				order = append(order, "mw2-before:"+routeKey.Path)
				err := next(ctx)
				order = append(order, "mw2-after:"+ctx.RouteKey.Path)
				return err
			}
		},
	)

	router.RegisterComponentRoute("runtimecfg:action:edit", ComponentHandlerFunc(func(ctx *Context) error {
		order = append(order, "handler:"+ctx.RouteKey.Path)
		if ctx.RouteKey.Kind != InteractionKindComponent {
			t.Fatalf("unexpected route kind: %v", ctx.RouteKey.Kind)
		}
		return nil
	}))

	router.HandleInteraction(session, buildComponentInteraction("runtimecfg:action:edit|main|ALL|bot_theme|global", "guild", "user"))

	want := []string{
		"mw1-before:runtimecfg:action:edit",
		"mw2-before:runtimecfg:action:edit",
		"handler:runtimecfg:action:edit",
		"mw2-after:runtimecfg:action:edit",
		"mw1-after:runtimecfg:action:edit",
	}
	if len(order) != len(want) {
		t.Fatalf("middleware call count mismatch: got %d want %d order=%v", len(order), len(want), order)
	}
	for index := range want {
		if order[index] != want[index] {
			t.Fatalf("middleware order mismatch at %d: got %q want %q full=%v", index, order[index], want[index], order)
		}
	}
}
