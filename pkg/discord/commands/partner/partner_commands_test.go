package partner

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestPartnerCommandRegistration(t *testing.T) {
	session, _ := newPartnerCommandTestSession(t)
	cm := files.NewMemoryConfigManager()
	router := core.NewCommandRouter(session, cm)

	NewPartnerCommands(cm).RegisterCommands(router)

	cmd, ok := router.GetRegistry().GetCommand("partner")
	if !ok {
		t.Fatal("expected /partner command to be registered")
	}

	options := cmd.Options()
	got := map[string]struct{}{}
	for _, opt := range options {
		if opt != nil {
			got[opt.Name] = struct{}{}
		}
	}

	for _, name := range []string{"add", "read", "update", "delete", "list", "sync"} {
		if _, exists := got[name]; !exists {
			t.Fatalf("expected /partner to contain subcommand %q, got %#v", name, got)
		}
	}

	// Discord API requires all required options to come before optional ones.
	for _, sub := range options {
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
