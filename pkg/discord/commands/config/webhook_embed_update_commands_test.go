package config

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

func TestParseScopeDefaultsToGuildInGuildContext(t *testing.T) {
	t.Parallel()

	ctx := &core.Context{GuildID: "guild-1"}
	extractor := core.NewOptionExtractor(nil)

	scope, err := parseScope(ctx, extractor)
	if err != nil {
		t.Fatalf("parseScope returned error: %v", err)
	}
	if scope != "guild-1" {
		t.Fatalf("expected scope guild-1, got %q", scope)
	}
}

func TestParseScopeRequiresExplicitScopeOutsideGuild(t *testing.T) {
	t.Parallel()

	ctx := &core.Context{GuildID: ""}
	extractor := core.NewOptionExtractor(nil)

	if _, err := parseScope(ctx, extractor); err == nil {
		t.Fatal("expected error when scope is omitted outside guild context")
	}
}

func TestParseScopeExplicitGlobal(t *testing.T) {
	t.Parallel()

	ctx := &core.Context{GuildID: "guild-1"}
	extractor := core.NewOptionExtractor([]*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name:  optionScope,
			Type:  discordgo.ApplicationCommandOptionString,
			Value: scopeGlobal,
		},
	})

	scope, err := parseScope(ctx, extractor)
	if err != nil {
		t.Fatalf("parseScope returned error: %v", err)
	}
	if scope != "" {
		t.Fatalf("expected global scope as empty string, got %q", scope)
	}
}

func TestParseScopeExplicitGuild(t *testing.T) {
	t.Parallel()

	ctx := &core.Context{GuildID: "guild-2"}
	extractor := core.NewOptionExtractor([]*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name:  optionScope,
			Type:  discordgo.ApplicationCommandOptionString,
			Value: scopeGuild,
		},
	})

	scope, err := parseScope(ctx, extractor)
	if err != nil {
		t.Fatalf("parseScope returned error: %v", err)
	}
	if scope != "guild-2" {
		t.Fatalf("expected scope guild-2, got %q", scope)
	}
}

func TestParseScopeRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	ctx := &core.Context{GuildID: "guild-3"}
	extractor := core.NewOptionExtractor([]*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name:  optionScope,
			Type:  discordgo.ApplicationCommandOptionString,
			Value: "invalid",
		},
	})

	if _, err := parseScope(ctx, extractor); err == nil {
		t.Fatal("expected error for invalid scope value")
	}
}
