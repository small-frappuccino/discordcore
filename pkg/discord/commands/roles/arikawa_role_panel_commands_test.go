package roles

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	rolesvc "github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestRolePanelCommands_Registration(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	svc := rolesvc.NewRolePanelService(cm)
	rc := NewRolePanelCommands(cm, svc)

	router := legacycore.NewArikawaCommandRouter("fake-token", cm)
	rc.RegisterCommands(router)

	cmds := router.GetAllCommands()
	if len(cmds) == 0 {
		t.Errorf("expected commands to be registered, got none")
	}

	if _, ok := cmds[rolePanelCommandName]; !ok {
		t.Errorf("expected command %s to be registered", rolePanelCommandName)
	}
}
