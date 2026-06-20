package legacycore

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestBaseFunctions(t *testing.T) {
	cb := NewContextBuilder(nil, nil, nil)

	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "test",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name: "sub",
						Type: discordgo.ApplicationCommandOptionSubCommand,
					},
				},
			},
			User: &discordgo.User{ID: "user1"},
		},
	}

	ctx := cb.BuildContext(i)
	if ctx.UserID != "user1" {
		t.Fatal("user not extracted")
	}
	if GetSubCommandName(i) != "sub" {
		t.Fatal("subcommand name failed")
	}

	if len(GetSubCommandOptions(i)) != 0 {
		t.Fatal("subcommand options failed")
	}
	CommandLogEntry(i, "cmd", "u")
	if err := ValidateGuildContext(ctx); err == nil {
		t.Fatal("expected guild error")
	}
	ctx.GuildID = "g"
	if err := ValidateGuildContext(ctx); err == nil {
		t.Fatal("expected config error")
	}
	ctx.GuildConfig = &files.GuildConfig{}
	if err := ValidateGuildContext(ctx); err != nil {
		t.Fatal("expected no error")
	}
	ctxNoUser := &Context{}
	if err := ValidateUserContext(ctxNoUser); err == nil {
		t.Fatal("expected user error")
	}

	_, found := HasFocusedOption([]*discordgo.ApplicationCommandInteractionDataOption{
		{Focused: true},
	})
	if !found {
		t.Fatal("expected focused")
	}

	if path := GetCommandPath(i); path != "test sub" {
		t.Fatalf("path failed: %s", path)
	}

	if IsAutocompleteInteraction(i) {
		t.Fatal("should not be auto")
	}
	if !IsSlashCommandInteraction(i) {
		t.Fatal("should be slash")
	}

	fields := CreateLogFields(ctx, map[string]any{"extra": "val"})
	if fields["extra"] != "val" {
		t.Fatal("fields failed")
	}

	if err := RequiresGuildConfig(ctxNoUser); err == nil {
		t.Fatal("requires guild config error expected")
	}

	if err := SafeGuildAccess(ctx, func(gc *files.GuildConfig) error { return nil }); err != nil {
		t.Fatal("safe guild access failed")
	}
}
