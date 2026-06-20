package core

import (
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

type InteractionContext struct {
	Event   *discord.InteractionEvent
	Client  *api.Client
	Options []discord.CommandInteractionOption
}

func NewInteractionContext(client *api.Client, event *discord.InteractionEvent) *InteractionContext {
	ctx := &InteractionContext{
		Event:  event,
		Client: client,
	}

	if data, ok := event.Data.(*discord.CommandInteraction); ok && data != nil {
		ctx.Options = data.Options
	}

	return ctx
}

func (ctx *InteractionContext) RespondMessage(content string) error {
	data := api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString(content),
		},
	}
	return ctx.Client.RespondInteraction(ctx.Event.ID, ctx.Event.Token, data)
}

func (ctx *InteractionContext) StringOption(name string) (string, bool) {
	for _, opt := range ctx.Options {
		if opt.Name == name {
			return opt.String(), true
		}
	}
	return "", false
}

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
