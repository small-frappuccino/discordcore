package config

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestDormantGuildBootstrapRouteDomainSeparatesBaseAndQOTD(t *testing.T) {
	t.Parallel()

	baseRoute := core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "config commands_enabled"}
	qotdRoute := core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "config qotd_schedule"}
	blockedRoute := core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "partner list"}

	if domain, ok := DormantGuildBootstrapRouteDomain(baseRoute); !ok || domain != "" {
		t.Fatalf("expected base bootstrap route in default domain, got domain=%q ok=%v", domain, ok)
	}
	if domain, ok := DormantGuildBootstrapRouteDomain(qotdRoute); !ok || domain != files.BotDomainQOTD {
		t.Fatalf("expected qotd bootstrap route in qotd domain, got domain=%q ok=%v", domain, ok)
	}
	if domain, ok := DormantGuildBootstrapRouteDomain(blockedRoute); ok || domain != "" {
		t.Fatalf("expected non-bootstrap route to be rejected, got domain=%q ok=%v", domain, ok)
	}

	if !AllowsDormantGuildBootstrapRouteForDomain("", baseRoute) {
		t.Fatal("expected base bootstrap route to be allowed in default domain")
	}
	if AllowsDormantGuildBootstrapRouteForDomain(files.BotDomainQOTD, baseRoute) {
		t.Fatal("expected base bootstrap route to be blocked in qotd domain")
	}
	if !AllowsDormantGuildBootstrapRouteForDomain(files.BotDomainQOTD, qotdRoute) {
		t.Fatal("expected qotd bootstrap route to be allowed in qotd domain")
	}
	if AllowsDormantGuildBootstrapRouteForDomain("", qotdRoute) {
		t.Fatal("expected qotd bootstrap route to be blocked in default domain")
	}
	if AllowsDormantGuildBootstrapRouteForDomain(files.BotDomainQOTD, blockedRoute) {
		t.Fatal("expected non-bootstrap route to remain blocked")
	}
}