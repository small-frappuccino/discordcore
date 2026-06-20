package core

import (
	"encoding/json"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
)

func TestContext_StringOption(t *testing.T) {
	rawOption := `{"name":"test_opt","type":3,"value":"test_value"}`
	var opt discord.CommandInteractionOption
	err := json.Unmarshal([]byte(rawOption), &opt)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	ctx := &InteractionContext{
		Options: []discord.CommandInteractionOption{opt},
	}

	val, ok := ctx.StringOption("test_opt")
	if !ok || val != "test_value" {
		t.Fatalf("expected test_value, got %v ok=%v", val, ok)
	}
}

func TestContext_HasRole(t *testing.T) {
	ctx := &InteractionContext{
		Event: &discord.InteractionEvent{
			Member: &discord.Member{
				RoleIDs: []discord.RoleID{discord.RoleID(123)},
			},
		},
	}

	if !ctx.HasRole(discord.RoleID(123)) {
		t.Fatal("expected role 123 to be found")
	}
	if ctx.HasRole(discord.RoleID(456)) {
		t.Fatal("expected role 456 to not be found")
	}
}
