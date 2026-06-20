package legacycore

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestSimpleCommandAndManager(t *testing.T) {
	cmd := NewSimpleCommand("cmd", "desc", nil, func(ctx *Context) error { return nil }, false, false)
	cmd.WithAutocomplete(AutocompleteHandlerFunc(func(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
		return nil, nil
	}))

	if cmd.Name() != "cmd" {
		t.Fatal("name")
	}
	if cmd.Description() != "desc" {
		t.Fatal("desc")
	}
	if cmd.Options() != nil {
		t.Fatal("options")
	}
	if cmd.RequiresGuild() != false {
		t.Fatal("req guild")
	}
	if cmd.RequiresPermissions() != false {
		t.Fatal("req perm")
	}
	if cmd.Handle(nil) != nil {
		t.Fatal("handle")
	}
	if cmd.AutocompleteRouteHandler() == nil {
		t.Fatal("autocomplete")
	}

	// Test CommandRouter accessors
	session, _ := discordgo.New("Bot test")
	cm := NewCommandManager(session, &files.ConfigManager{})
	cr := cm.GetRouter()

	if cr.GetSession() == nil {
		t.Fatal("session")
	}
	if cr.GetRegistry() == nil {
		t.Fatal("registry")
	}
	if cr.GetPermissionChecker() == nil {
		t.Fatal("checker")
	}

	cr.SetStore(nil)
	if cr.GetStore() != nil {
		t.Fatal("store")
	}
	cr.SetCache(nil)
	cr.SetRuntimeApplier(nil)
	if cr.GetRuntimeApplier() != nil {
		t.Fatal("applier")
	}
	cr.SetTaskRouter(nil)
	if cr.GetTaskRouter() != nil {
		t.Fatal("task router")
	}
}
func TestCommandManager_BuildGuildSubCommandOption(t *testing.T) {
	session, _ := discordgo.New("Bot test")
	cm := NewCommandManager(session, &files.ConfigManager{})

	// test nil cases
	opt, ok := cm.buildGuildSubCommandOption("g", "p", nil)
	if ok || opt != nil {
		t.Fatal("nil")
	}

	// test GroupCommand empty
	group := NewGroupCommand("g1", "desc", nil)
	opt, ok = cm.buildGuildSubCommandOption("g", "p", group)
	if ok || opt != nil {
		t.Fatal("empty group")
	}

	// test nested group
	group2 := NewGroupCommand("g2", "desc", nil)
	group2.AddSubCommand(NewSimpleCommand("sub2", "desc", nil, nil, false, false))
	group.AddSubCommand(group2)

	opt, ok = cm.buildGuildSubCommandOption("g", "p", group)
	if !ok || opt.Type != discordgo.ApplicationCommandOptionSubCommandGroup {
		t.Fatal("nested group")
	}

	// test regular command that SHOULD sync
	cmd2 := NewSimpleCommand("cmd2", "desc", []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionSubCommand},
	}, nil, false, false)
	opt, ok = cm.buildGuildSubCommandOption("", "p", cmd2)
	if !ok || opt.Type != discordgo.ApplicationCommandOptionSubCommandGroup {
		t.Fatal("subcmd with options")
	}
}
