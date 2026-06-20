package tickets

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	discordtickets "github.com/small-frappuccino/discordcore/pkg/discord/tickets"
	"github.com/small-frappuccino/discordcore/pkg/files"
	pkgtickets "github.com/small-frappuccino/discordcore/pkg/tickets"
)

// TicketRouter intercepts gateway events to process ticket components.
type TicketRouter struct {
	state  *state.State
	svc    *discordtickets.Service
	mgr    *pkgtickets.Manager
	config *files.ConfigManager
	logger *slog.Logger
}

// NewTicketRouter instantiates the Arikawa native router.
func NewTicketRouter(st *state.State, svc *discordtickets.Service, mgr *pkgtickets.Manager, cm *files.ConfigManager, logger *slog.Logger) *TicketRouter {
	r := &TicketRouter{
		state:  st,
		svc:    svc,
		mgr:    mgr,
		config: cm,
		logger: logger,
	}
	st.AddHandler(r.HandleInteraction)
	return r
}

// HandleInteraction routes component interactions and enforces deferral before synchronous I/O.
func (r *TicketRouter) HandleInteraction(e *gateway.InteractionCreateEvent) {
	var customID string
	var values []string

	switch data := e.Data.(type) {
	case *discord.ButtonInteraction:
		customID = string(data.CustomID)
	case *discord.StringSelectInteraction:
		customID = string(data.CustomID)
		values = data.Values
	default:
		return
	}

	switch customID {
	case "ticket_category_select", "ticket_close", "ticket_transcript", "ticket_reopen", "ticket_delete":
		// Enforce timeout invariant: Deferred interaction response immediately.
		err := r.state.RespondInteraction(e.ID, e.Token, api.InteractionResponse{
			Type: api.DeferredMessageInteractionWithSource,
			Data: &api.InteractionResponseData{
				Flags: discord.EphemeralMessage,
			},
		})
		if err != nil {
			r.logger.Error("failed to defer interaction", "error", err)
			return
		}

		// Transition to sync I/O.
		r.dispatch(e, customID, values)
	}
}

func (r *TicketRouter) dispatch(e *gateway.InteractionCreateEvent, customID string, values []string) {
	ctx := context.Background()
	var err error

	switch customID {
	case "ticket_category_select":
		err = r.handleCategorySelect(ctx, e, values)
	case "ticket_close":
		err = r.handleClose(ctx, e)
	case "ticket_transcript":
		err = r.handleTranscript(ctx, e)
	case "ticket_reopen":
		err = r.handleReopen(ctx, e)
	case "ticket_delete":
		err = r.handleDelete(ctx, e)
	}

	if err != nil {
		r.logger.Error("ticket interaction failed", "error", err)
		r.state.EditInteractionResponse(e.AppID, e.Token, api.EditInteractionResponseData{
			Content: option.NewNullableString(fmt.Sprintf("Error: %v", err)),
		})
	}
}

func (r *TicketRouter) handleCategorySelect(ctx context.Context, e *gateway.InteractionCreateEvent, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("no category selected")
	}
	categoryName := values[0]

	cfg := r.config.GuildConfig(e.GuildID.String())
	if cfg == nil || !cfg.Tickets.Enabled {
		return fmt.Errorf("tickets are not enabled on this server")
	}

	var roleID string
	for _, cat := range cfg.Tickets.Categories {
		if strings.EqualFold(cat.Name, categoryName) {
			roleID = cat.RoleID
			break
		}
	}
	if roleID == "" {
		return fmt.Errorf("invalid category selected")
	}

	nextID, err := r.mgr.NextID(ctx, e.GuildID.String())
	if err != nil {
		return fmt.Errorf("next id: %w", err)
	}

	channelName := pkgtickets.GenerateTicketName(nextID)

	var parentID discord.ChannelID
	if ch, err := r.state.Channel(e.ChannelID); err == nil && ch.ParentID.IsValid() {
		parentID = ch.ParentID
	}

	roleIDParsed, _ := discord.ParseSnowflake(roleID)
	ch, err := r.svc.CreateTicketChannel(ctx, e.GuildID, e.SenderID(), discord.RoleID(roleIDParsed), channelName, parentID)
	if err != nil {
		return fmt.Errorf("create channel: %w", err)
	}

	_, err = r.state.EditInteractionResponse(e.AppID, e.Token, api.EditInteractionResponseData{
		Content: option.NewNullableString(fmt.Sprintf("Ticket created: <#%s>", ch.ID)),
	})
	return err
}

func (r *TicketRouter) handleClose(ctx context.Context, e *gateway.InteractionCreateEvent) error {
	ch, err := r.state.Channel(e.ChannelID)
	if err != nil {
		return fmt.Errorf("fetch channel: %w", err)
	}
	if !pkgtickets.IsOpenTicket(ch.Name) {
		return fmt.Errorf("not an open ticket")
	}

	if err := r.svc.CloseTicket(ctx, ch); err != nil {
		return err
	}
	_, err = r.state.EditInteractionResponse(e.AppID, e.Token, api.EditInteractionResponseData{
		Content: option.NewNullableString("Ticket has been closed."),
	})
	return err
}

func (r *TicketRouter) handleTranscript(ctx context.Context, e *gateway.InteractionCreateEvent) error {
	cfg := r.config.GuildConfig(e.GuildID.String())
	var auditChannelID string
	if cfg != nil {
		auditChannelID = cfg.Tickets.TranscriptChannelID
	}
	if auditChannelID == "" {
		return fmt.Errorf("audit channel is not configured")
	}

	auditIDParsed, _ := discord.ParseSnowflake(auditChannelID)
	err := r.svc.GenerateAndUploadTranscript(ctx, e.ChannelID, discord.ChannelID(auditIDParsed))
	if err != nil {
		return err
	}

	_, err = r.state.EditInteractionResponse(e.AppID, e.Token, api.EditInteractionResponseData{
		Content: option.NewNullableString("Transcript generated."),
	})
	return err
}

func (r *TicketRouter) handleReopen(ctx context.Context, e *gateway.InteractionCreateEvent) error {
	ch, err := r.state.Channel(e.ChannelID)
	if err != nil {
		return fmt.Errorf("fetch channel: %w", err)
	}
	if !pkgtickets.IsClosedTicket(ch.Name) {
		return fmt.Errorf("not a closed ticket")
	}

	if err := r.svc.ReopenTicket(ctx, ch); err != nil {
		return err
	}
	_, err = r.state.EditInteractionResponse(e.AppID, e.Token, api.EditInteractionResponseData{
		Content: option.NewNullableString("Ticket reopened."),
	})
	return err
}

func (r *TicketRouter) handleDelete(ctx context.Context, e *gateway.InteractionCreateEvent) error {
	if err := r.svc.DeleteTicket(ctx, e.ChannelID); err != nil {
		return err
	}
	return nil
}
