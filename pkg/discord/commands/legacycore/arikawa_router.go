package legacycore

import (
	"encoding/json"
	"errors"
	"log/slog"
	"runtime"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordgo"
)

// ErrAlreadyAcknowledged is returned by handlers that have already sent a response to Discord
// and do not want the router to log a failure.
var ErrAlreadyAcknowledged = errors.New("interaction has already been acknowledged")

// ArikawaCommandRouter manages routing and execution of Arikawa commands.
type ArikawaCommandRouter struct {
	commands   map[string]ArikawaCommand
	components map[string]ArikawaComponentHandler
	client     *api.Client
	config     *files.ConfigManager
}

// ArikawaComponentHandler defines the interface for native component handlers.
type ArikawaComponentHandler interface {
	HandleComponent(ctx *ArikawaContext) error
}

// NewArikawaCommandRouter creates a new Arikawa router.
func NewArikawaCommandRouter(token string, config *files.ConfigManager) *ArikawaCommandRouter {
	router := &ArikawaCommandRouter{
		commands:   make(map[string]ArikawaCommand),
		components: make(map[string]ArikawaComponentHandler),
		client:     api.NewClient("Bot " + token),
		config:     config,
	}
	log.ApplicationLogger().Info("Initialized Arikawa command router")
	return router
}

// Register registers an Arikawa command.
func (r *ArikawaCommandRouter) Register(cmd ArikawaCommand) {
	r.commands[cmd.Name()] = cmd
}

// RegisterComponent registers a component interaction handler.
func (r *ArikawaCommandRouter) RegisterComponent(customID string, handler ArikawaComponentHandler) {
	r.components[customID] = handler
}

// HandleRawEvent intercepts raw Discord events and routes INTERACTION_CREATE natively to Arikawa.
func (r *ArikawaCommandRouter) HandleRawEvent(s *discordgo.Session, e *discordgo.Event) {
	if e.Type != "INTERACTION_CREATE" {
		return
	}

	var interactionEvent discord.InteractionEvent
	if err := json.Unmarshal(e.RawData, &interactionEvent); err != nil {
		return
	}

	r.HandleInteractionEvent(&interactionEvent)
}

// HandleInteractionEvent natively processes an Arikawa InteractionEvent (useful for testing or direct routing).
func (r *ArikawaCommandRouter) HandleInteractionEvent(interactionEvent *discord.InteractionEvent) {
	switch data := interactionEvent.Data.(type) {
	case *discord.CommandInteraction:
		cmd, exists := r.commands[data.Name]
		if !exists {
			return
		}

		ctx := r.buildContext(interactionEvent)
		if err := cmd.Handle(ctx); err != nil && !errors.Is(err, ErrAlreadyAcknowledged) {
			r.logHandlerError("command", data.Name, interactionEvent, err)
		}
	default:
		// Attempt to extract CustomID if it's a component interaction
		if cmp, ok := data.(interface{ ID() discord.ComponentID }); ok {
			var handler ArikawaComponentHandler
			var matchedID string

			for id, h := range r.components {
				if strings.HasPrefix(string(cmp.ID()), id) {
					handler = h
					matchedID = id
					break
				}
			}

			if handler != nil {
				ctx := r.buildContext(interactionEvent)
				if err := handler.HandleComponent(ctx); err != nil && !errors.Is(err, ErrAlreadyAcknowledged) {
					r.logHandlerError("component", matchedID, interactionEvent, err)
				}
			}
		}
	}
}

func (r *ArikawaCommandRouter) logHandlerError(kind, name string, interactionEvent *discord.InteractionEvent, err error) {
	stackBuf := make([]byte, 4096)
	n := runtime.Stack(stackBuf, false)

	log.ErrorLoggerRaw().Error("Arikawa handler failed",
		slog.String("kind", kind),
		slog.String("name", name),
		slog.String("request_id", interactionEvent.ID.String()),
		slog.Int("http_status", 500),
		slog.Any("error", err),
		slog.String("stack_trace", string(stackBuf[:n])),
	)
}

func (r *ArikawaCommandRouter) buildContext(interactionEvent *discord.InteractionEvent) *ArikawaContext {
	ctx := &ArikawaContext{
		Client:      r.client,
		Interaction: interactionEvent,
		Config:      r.config,
		Logger:      log.DiscordLogger(),
		GuildID:     interactionEvent.GuildID,
	}

	if interactionEvent.Member != nil {
		ctx.UserID = interactionEvent.Member.User.ID
	} else if interactionEvent.User != nil {
		ctx.UserID = interactionEvent.User.ID
	}

	if r.config != nil {
		ctx.GuildConfig = r.config.GuildConfig(interactionEvent.GuildID.String())
	}
	return ctx
}

// GetAllCommands returns all registered Arikawa commands.
func (r *ArikawaCommandRouter) GetAllCommands() map[string]ArikawaCommand {
	return r.commands
}

// ConvertArikawaOptions converts Arikawa command options to Discordgo options via JSON bridge.
func ConvertArikawaOptions(opts []discord.CommandOption) []*discordgo.ApplicationCommandOption {
	if len(opts) == 0 {
		return nil
	}
	b, err := json.Marshal(opts)
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to marshal arikawa options", slog.Any("error", err))
		return nil
	}

	var dgoOpts []*discordgo.ApplicationCommandOption
	if err := json.Unmarshal(b, &dgoOpts); err != nil {
		log.ErrorLoggerRaw().Error("Failed to unmarshal to discordgo options", slog.Any("error", err))
		return nil
	}
	return dgoOpts
}

// NewArikawaMissingConfigErrorData constructs an actionable error response data for Arikawa
func NewArikawaMissingConfigErrorData(guildID, featureName, dashboardPath string) api.InteractionResponseData {
	url := "https://discordcore.app/manage/" + guildID + dashboardPath
	return api.InteractionResponseData{
		Content: option.NewNullableString("The **" + featureName + "** feature has not been fully configured on the dashboard. Please configure it to use this command."),
		Flags:   discord.EphemeralMessage,
		Components: discord.ComponentsPtr(
			&discord.ButtonComponent{
				Label: "Configure Feature",
				Style: discord.LinkButtonStyle(discord.URL(url)),
			},
		),
	}
}
