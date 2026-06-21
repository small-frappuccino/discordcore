package roles

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/api"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	rolesvc "github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestRolePanelCommands_Registration(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	svc := rolesvc.NewRolePanelService(cm)
	rc := NewRolePanelCommands(cm, svc)

	router := commands.NewCommandRouter(api.NewClient("dummy"), cm)
	rc.RegisterCommands(router)

	cmds := router.Registry().GetAllCommands()
	if len(cmds) == 0 {
		t.Errorf("expected commands to be registered, got none")
	}

	if _, ok := cmds[rolePanelCommandName]; !ok {
		t.Errorf("expected command %s to be registered", rolePanelCommandName)
	}
}
