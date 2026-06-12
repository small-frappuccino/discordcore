package core

import (
	"testing"

	"github.com/small-frappuccino/discordgo"
)

func TestTypes(t *testing.T) {
	cmdErr := &CommandError{
		Message: "msg",
		Code:    "CODE_1",
	}
	if cmdErr.Error() != "msg" {
		t.Fatal("CommandError string failed")
	}
	if cmdErr.CommandErrorCode() != "CODE_1" {
		t.Fatal("CommandErrorCode failed")
	}
	valErr := &ValidationError{
		Field:   "field1",
		Message: "val_msg",
	}
	if valErr.Error() != "val_msg" {
		t.Fatal("ValidationError string failed")
	}
	if valErr.ValidationField() != "field1" {
		t.Fatal("ValidationField failed")
	}
	reg := &CommandRegistry{
		commands:    make(map[string]Command),
		subcommands: make(map[string]map[string]Command),
	}
	reg.RegisterSubCommand("parent", &mockCmd{name: "sub"})
	if len(reg.GetAllSubCommands("parent")) != 1 {
		t.Fatal("GetAllSubCommands failed")
	}
}

type mockCmd struct{ name string }

func (m *mockCmd) Name() string                                   { return m.name }
func (m *mockCmd) Description() string                            { return "" }
func (m *mockCmd) Options() []*discordgo.ApplicationCommandOption { return nil }
func (m *mockCmd) Handle(ctx *Context) error                      { return nil }
func (m *mockCmd) RequiresGuild() bool                            { return false }
func (m *mockCmd) RequiresPermissions() bool                      { return false }
