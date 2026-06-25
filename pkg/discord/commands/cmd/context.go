package cmd

import (
	"context"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"

	"github.com/small-frappuccino/discordcore/pkg/config"
)

// DIContainer provides an abstraction for accessing required services.
type DIContainer interface {
	ConfigProvider() config.Provider
}

// Tx defines an atomic database transaction boundary.
type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Context carries request-scoped state for command handlers.
type Context struct {
	context.Context
	Event   *discord.InteractionEvent
	Client  *api.Client
	Options []discord.CommandInteractionOption
	Logger  *slog.Logger
	DI      DIContainer
	Tx      Tx
	GuildID discord.GuildID
	UserID  discord.UserID
}

// CommandHandler defines the canonical function signature for executing a slash command.
type CommandHandler func(ctx *Context) error

// NewContext creates a new Context.
func NewContext(ctx context.Context, client *api.Client, event *discord.InteractionEvent, logger *slog.Logger, di DIContainer, tx Tx) *Context {
	cmdCtx := &Context{
		Context: ctx,
		Event:   event,
		Client:  client,
		Logger:  logger,
		DI:      di,
		Tx:      tx,
	}

	if event != nil {
		cmdCtx.GuildID = event.GuildID
		if event.Member != nil {
			cmdCtx.UserID = event.Member.User.ID
		} else if event.User != nil {
			cmdCtx.UserID = event.User.ID
		}

		if data, ok := event.Data.(*discord.CommandInteraction); ok && data != nil {
			cmdCtx.Options = data.Options
		}
	}

	return cmdCtx
}

// StringOption retrieves the string value of a command option by its name.
func (ctx *Context) StringOption(name string) (string, bool) {
	for _, opt := range ctx.Options {
		if opt.Name == name {
			return opt.String(), true
		}
	}
	return "", false
}

// RespondMessage transmits a synchronous text response to the interaction.
func (ctx *Context) RespondMessage(content string) error {
	data := api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString(content),
		},
	}
	return ctx.Client.RespondInteraction(ctx.Event.ID, ctx.Event.Token, data)
}
