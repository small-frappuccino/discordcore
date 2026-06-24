package commands

import (
	"context"
	"errors"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// ErrInvalidEventData indicates that the interaction event payload is malformed or nil.
var ErrInvalidEventData = errors.New("interaction event payload is structurally invalid or nil")

// ArikawaContext provides a safe execution boundary and dependency injection container
// for domain commands, encapsulating the raw Arikawa primitives.
type ArikawaContext struct {
	Client      *api.Client
	Interaction *discord.InteractionEvent
	Config      config.Provider
	Logger      *slog.Logger
	GuildID     discord.GuildID
	UserID      discord.UserID
	GuildConfig *files.GuildConfig
	ctx         context.Context
}

// NewArikawaContext constructs an operational context securely. It validates the
// payload defensively to avoid runtime panics when faced with malformed inputs.
func NewArikawaContext(event discord.InteractionEvent, configManager config.Provider) (*ArikawaContext, error) {
	// Defensive Validation against bizzare payloads.
	if event.SenderID() == 0 {
		return nil, ErrInvalidEventData
	}

	logger := log.DiscordLogger()
	if logger == nil {
		logger = slog.Default() // Fallback to avoid nil pointer dereference
	}

	ctx := &ArikawaContext{
		Interaction: &event,
		Config:      configManager,
		Logger:      logger,
		GuildID:     event.GuildID,
		UserID:      event.SenderID(),
		ctx:         context.Background(),
	}

	if configManager != nil && event.GuildID.IsValid() {
		ctx.GuildConfig = configManager.GuildConfig(event.GuildID.String())
	}

	return ctx, nil
}

// Context returns the standard library context.
func (c *ArikawaContext) Context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// WithContext updates the underlying execution context.
func (c *ArikawaContext) WithContext(ctx context.Context) {
	c.ctx = ctx
}

// Respond responds to the interaction with the given message data.
func (c *ArikawaContext) Respond(data api.InteractionResponseData) error {
	if c.Client == nil || c.Interaction == nil {
		return errors.New("cannot respond: nil client or interaction")
	}
	return c.Client.RespondInteraction(c.Interaction.ID, c.Interaction.Token, api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &data,
	})
}

// Defer defers the interaction with optional message flags.
func (c *ArikawaContext) Defer(flags discord.MessageFlags) error {
	if c.Client == nil || c.Interaction == nil {
		return errors.New("cannot defer: nil client or interaction")
	}
	var data *api.InteractionResponseData
	if flags != 0 {
		data = &api.InteractionResponseData{Flags: flags}
	}
	return c.Client.RespondInteraction(c.Interaction.ID, c.Interaction.Token, api.InteractionResponse{
		Type: api.DeferredMessageInteractionWithSource,
		Data: data,
	})
}

// SetClient explicitly sets the API client for this request boundary.
func (c *ArikawaContext) SetClient(client *api.Client) {
	c.Client = client
}
