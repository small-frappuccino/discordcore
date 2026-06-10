package core

import "testing"

func TestRouteRegistryCoverage(t *testing.T) {
	router := &CommandRouter{
		routeRegistry: newInteractionRouteRegistry(),
	}

	// register a slash route
	router.RegisterSlashRoute("path", nil)
	router.RegisterAutocompleteRoute("path", nil)
	router.RegisterComponentRoute("path", nil)
	router.RegisterModalRoute("path", nil)

	// Test JoinRoutePath
	if p := JoinRoutePath("a", "b"); p != "a b" {
		t.Fatal("join")
	}
	if p := JoinRoutePath("a", ""); p != "a" {
		t.Fatal("join empty")
	}

	// Test domains
	router.RegisterInteractionRouteForDomain("domain2", InteractionRouteBinding{Path: "path2"})

	// Test register slash commands
	cmd := NewSimpleCommand("cmd", "desc", nil, func(ctx *Context) error { return nil }, false, false)
	router.registerSlashCommandRoutes(cmd)
	router.registerSlashSubCommandRoutes("parent", cmd)

	// test domains with commands
	router.registerSlashCommandRoutesForDomain("domain3", cmd)
	router.registerSlashSubCommandRoutesForDomain("domain3", "parent", cmd)

	// InteractionRouteDomain
	d := router.InteractionRouteDomain(InteractionRouteKey{Kind: InteractionKindSlash, Path: "path"})
	if d != "" {
		t.Fatal("domain lookup")
	}

	// Test error cases for router handlers
	router.HandleInteraction(nil, nil)
}
