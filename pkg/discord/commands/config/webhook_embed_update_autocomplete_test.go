package config

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestWebhookEmbedAutocompleteRoutesThroughConfigCommandTree(t *testing.T) {
	const (
		guildID = "guild-webhook-autocomplete"
		ownerID = "owner-webhook-autocomplete"
	)

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	if err := cm.CreateWebhookEmbedUpdate(guildID, files.WebhookEmbedUpdateConfig{
		MessageID:  "m-alpha",
		WebhookURL: "https://discord.com/api/webhooks/123/token-alpha",
		Embed:      []byte(`{"title":"alpha"}`),
	}); err != nil {
		t.Fatalf("create guild webhook embed update alpha: %v", err)
	}
	if err := cm.CreateWebhookEmbedUpdate(guildID, files.WebhookEmbedUpdateConfig{
		MessageID:  "m-beta",
		WebhookURL: "https://discord.com/api/webhooks/123/token-beta",
		Embed:      []byte(`{"title":"beta"}`),
	}); err != nil {
		t.Fatalf("create guild webhook embed update beta: %v", err)
	}
	if err := cm.CreateWebhookEmbedUpdate("", files.WebhookEmbedUpdateConfig{
		MessageID:  "m-global",
		WebhookURL: "https://discord.com/api/webhooks/123/token-global",
		Embed:      []byte(`{"title":"global"}`),
	}); err != nil {
		t.Fatalf("create global webhook embed update: %v", err)
	}

	router.HandleInteraction(session, newConfigAutocompleteInteraction(guildID, ownerID, "webhook_embed_read", []*discordgo.ApplicationCommandInteractionDataOption{
		autocompleteStringOpt(optionMessageID, "m-", true),
	}))

	resp := rec.lastResponse(t)
	if resp.Type != discordgo.InteractionApplicationCommandAutocompleteResult {
		t.Fatalf("expected autocomplete response type, got %v", resp.Type)
	}
	if len(resp.Data.Choices) != 2 {
		t.Fatalf("expected 2 guild autocomplete choices, got %#v", resp.Data.Choices)
	}
	if resp.Data.Choices[0].Value != "m-alpha" || resp.Data.Choices[1].Value != "m-beta" {
		t.Fatalf("unexpected guild autocomplete values: %#v", resp.Data.Choices)
	}

	router.HandleInteraction(session, newConfigAutocompleteInteraction(guildID, ownerID, "webhook_embed_delete", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionScope, scopeGlobal),
		autocompleteStringOpt(optionMessageID, "m-", true),
	}))

	resp = rec.lastResponse(t)
	if len(resp.Data.Choices) != 1 {
		t.Fatalf("expected 1 global autocomplete choice, got %#v", resp.Data.Choices)
	}
	if resp.Data.Choices[0].Value != "m-global" {
		t.Fatalf("unexpected global autocomplete value: %#v", resp.Data.Choices)
	}
}

func TestWebhookEmbedAutocompleteSchemaEnabledOnMessageIDOptions(t *testing.T) {
	session, _ := newConfigCommandTestSession(t)
	cm := files.NewMemoryConfigManager()
	router := coreRouterForSchemaTest(t, session, cm)

	cmd, ok := router.GetRegistry().GetCommand("config")
	if !ok {
		t.Fatal("expected /config command to be registered")
	}

	expected := map[string]struct{}{
		"webhook_embed_read":   {},
		"webhook_embed_update": {},
		"webhook_embed_delete": {},
	}
	for _, option := range cmd.Options() {
		if option == nil {
			continue
		}
		if _, ok := expected[option.Name]; !ok {
			continue
		}
		if len(option.Options) == 0 || option.Options[0] == nil {
			t.Fatalf("expected %s to expose a message_id option", option.Name)
		}
		if option.Options[0].Name != optionMessageID {
			t.Fatalf("expected first option of %s to be message_id, got %s", option.Name, option.Options[0].Name)
		}
		if !option.Options[0].Autocomplete {
			t.Fatalf("expected message_id option of %s to use autocomplete", option.Name)
		}
		delete(expected, option.Name)
	}

	if len(expected) != 0 {
		t.Fatalf("missing expected webhook autocomplete subcommands in schema: %#v", expected)
	}
}

func coreRouterForSchemaTest(t *testing.T, session *discordgo.Session, cm *files.ConfigManager) *core.CommandRouter {
	t.Helper()
	router := core.NewCommandRouter(session, cm)
	NewConfigCommands(cm).RegisterCommands(router)
	return router
}

func newConfigAutocompleteInteraction(guildID, userID, subCommand string, options []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-autocomplete-" + subCommand,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommandAutocomplete,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "config",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:    subCommand,
						Type:    discordgo.ApplicationCommandOptionSubCommand,
						Options: options,
					},
				},
			},
		},
	}
}

func autocompleteStringOpt(name, value string, focused bool) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:    name,
		Type:    discordgo.ApplicationCommandOptionString,
		Value:   value,
		Focused: focused,
	}
}