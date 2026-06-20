package core

import (
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

// InteractionContext encapsulates the contextual state of a Discord interaction.
// It provides a unified interface for accessing event data, client bindings,
// and parsed command options during the execution of a command handler.
type InteractionContext struct {
	Event   *discord.InteractionEvent
	Client  *api.Client
	Options []discord.CommandInteractionOption
}

// NewInteractionContext initializes a new InteractionContext from a raw interaction event.
// It extracts and flattens command options if the underlying event represents a slash command.
func NewInteractionContext(client *api.Client, event *discord.InteractionEvent) *InteractionContext {
	ctx := &InteractionContext{
		Event:  event,
		Client: client,
	}

	// Type assert the interaction data to extract specific command options.
	// We safely ignore non-command interactions (e.g. autocomplete) as their options
	// are handled differently or irrelevant in this specific context layer.
	if data, ok := event.Data.(*discord.CommandInteraction); ok && data != nil {
		ctx.Options = data.Options
	}

	return ctx
}

// RespondMessage transmits a synchronous text response to the interaction.
// It constructs a MessageInteractionWithSource payload, acknowledging the event
// and displaying the provided content directly to the user.
func (ctx *InteractionContext) RespondMessage(content string) error {
	data := api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString(content),
		},
	}
	return ctx.Client.RespondInteraction(ctx.Event.ID, ctx.Event.Token, data)
}

// StringOption retrieves the string value of a command option by its name.
// It returns true if the option is found and successfully cast to a string,
// or false if the option is missing or possesses a different fundamental type.
func (ctx *InteractionContext) StringOption(name string) (string, bool) {
	for _, opt := range ctx.Options {
		if opt.Name == name {
			return opt.String(), true
		}
	}
	return "", false
}

// HasRole verifies if the executing member possesses the specified Discord role.
// It evaluates the cached role slice provided within the interaction payload,
// preventing external API queries. Returns false if the interaction occurred
// outside of a guild context (e.g. direct messages).
func (ctx *InteractionContext) HasRole(roleID discord.RoleID) bool {
	if ctx.Event.Member == nil {
		return false
	}
	for _, r := range ctx.Event.Member.RoleIDs {
		if r == roleID {
			return true
		}
	}
	return false
}
