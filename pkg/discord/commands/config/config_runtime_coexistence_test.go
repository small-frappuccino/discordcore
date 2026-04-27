package config

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	runtimecfg "github.com/small-frappuccino/discordcore/pkg/discord/commands/runtime"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestConfigRuntimeCoexistenceRegisterOrderConfigThenRuntime(t *testing.T) {
	session, _ := newConfigCommandTestSession(t)
	cm := files.NewMemoryConfigManager()
	router := core.NewCommandRouter(session, cm)

	NewConfigCommands(cm).RegisterCommands(router)
	runtimecfg.NewRuntimeConfigCommands(cm).RegisterCommands(router)

	assertConfigGroupContainsSubcommands(t, router, []string{
		"set",
		"get",
		"list",
		"qotd_enabled",
		"qotd_channel",
		"runtime",
		"webhook_embed_create",
		"webhook_embed_read",
		"webhook_embed_update",
		"webhook_embed_delete",
		"webhook_embed_list",
	})
}

func TestConfigRuntimeCoexistenceRegisterOrderRuntimeThenConfig(t *testing.T) {
	session, _ := newConfigCommandTestSession(t)
	cm := files.NewMemoryConfigManager()
	router := core.NewCommandRouter(session, cm)

	runtimecfg.NewRuntimeConfigCommands(cm).RegisterCommands(router)
	NewConfigCommands(cm).RegisterCommands(router)

	assertConfigGroupContainsSubcommands(t, router, []string{
		"set",
		"get",
		"list",
		"qotd_enabled",
		"qotd_channel",
		"runtime",
		"webhook_embed_create",
		"webhook_embed_read",
		"webhook_embed_update",
		"webhook_embed_delete",
		"webhook_embed_list",
	})
}

func assertConfigGroupContainsSubcommands(t *testing.T, router *core.CommandRouter, expected []string) {
	t.Helper()

	cmd, ok := router.GetRegistry().GetCommand("config")
	if !ok {
		t.Fatal("expected /config command to be registered")
	}

	options := cmd.Options()
	got := make(map[string]struct{}, len(options))
	for _, opt := range options {
		if opt != nil {
			got[opt.Name] = struct{}{}
		}
	}

	for _, name := range expected {
		if _, ok := got[name]; !ok {
			t.Fatalf("expected /config to contain subcommand %q, got %#v", name, got)
		}
	}

	all := router.GetRegistry().GetAllCommands()
	if _, ok := all["ping"]; !ok {
		t.Fatalf("expected /ping command to be registered")
	}
	if _, ok := all["echo"]; !ok {
		t.Fatalf("expected /echo command to be registered")
	}
}

func TestConfigRuntimeCoexistenceCommandOptionsAreSubcommands(t *testing.T) {
	session, _ := newConfigCommandTestSession(t)
	cm := files.NewMemoryConfigManager()
	router := core.NewCommandRouter(session, cm)
	NewConfigCommands(cm).RegisterCommands(router)
	runtimecfg.NewRuntimeConfigCommands(cm).RegisterCommands(router)

	cmd, ok := router.GetRegistry().GetCommand("config")
	if !ok {
		t.Fatal("expected /config command")
	}
	for _, opt := range cmd.Options() {
		if opt == nil {
			continue
		}
		if opt.Type != discordgo.ApplicationCommandOptionSubCommand {
			t.Fatalf("expected option %q to be subcommand, got type=%v", opt.Name, opt.Type)
		}
	}
}

func TestConfigSubcommandsRequiredOptionsBeforeOptional(t *testing.T) {
	session, _ := newConfigCommandTestSession(t)
	cm := files.NewMemoryConfigManager()
	router := core.NewCommandRouter(session, cm)
	NewConfigCommands(cm).RegisterCommands(router)
	runtimecfg.NewRuntimeConfigCommands(cm).RegisterCommands(router)

	cmd, ok := router.GetRegistry().GetCommand("config")
	if !ok {
		t.Fatal("expected /config command")
	}

	for _, sub := range cmd.Options() {
		if sub == nil || sub.Type != discordgo.ApplicationCommandOptionSubCommand || len(sub.Options) == 0 {
			continue
		}
		seenOptional := false
		for _, opt := range sub.Options {
			if opt == nil {
				continue
			}
			if !opt.Required {
				seenOptional = true
				continue
			}
			if seenOptional {
				t.Fatalf("subcommand %q has required option %q after optional options", sub.Name, opt.Name)
			}
		}
	}
}
