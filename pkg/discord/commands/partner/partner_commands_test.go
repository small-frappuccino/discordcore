package partner

import (
	"path/filepath"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestPartnerCommandRegistration(t *testing.T) {
	session, _ := newPartnerCommandTestSession(t)
	cm := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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
}
