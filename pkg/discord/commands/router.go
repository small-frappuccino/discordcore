package commands

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// ErrCommandNotFound is returned when a slash command interaction does not map to a registered handler.
var ErrCommandNotFound = errors.New("command not found in registry")

// ErrAlreadyAcknowledged allows handlers to gracefully exit without logging an error
// if they have already sent a response to Discord.
var ErrAlreadyAcknowledged = errors.New("interaction has already been acknowledged")

// CommandRouter natively routes incoming Arikawa interactions to their respective handlers.
// It bypasses the DiscordGo compatibility layer completely.
type CommandRouter struct {
	registry   *CommandRegistry
	components map[string]ComponentHandler
	client     *api.Client
	config     *files.ConfigManager
}

// NewCommandRouter instantiates a pure Arikawa command router.
func NewCommandRouter(client *api.Client, config *files.ConfigManager) *CommandRouter {
	return &CommandRouter{
		registry:   NewCommandRegistry(),
		components: make(map[string]ComponentHandler),
		client:     client,
		config:     config,
	}
}

// Register delegates the slash command registration to the thread-safe registry.
func (r *CommandRouter) Register(cmd ArikawaCommand) {
	r.registry.Register(cmd)
}

// RegisterComponent associates a stable custom ID prefix with a component handler.
func (r *CommandRouter) RegisterComponent(customIDPrefix string, handler ComponentHandler) {
	if r.components == nil {
		r.components = make(map[string]ComponentHandler)
	}
	r.components[customIDPrefix] = handler
}

// HandleEvent intercepts an Arikawa interaction and dispatches it.
func (r *CommandRouter) HandleEvent(event *discord.InteractionEvent) error {
	if event == nil {
		return nil
	}

	switch data := event.Data.(type) {
	case *discord.CommandInteraction:
		cmd, exists := r.registry.GetCommand(data.Name)
		if !exists {
			slog.Warn("Intercepted service degradation: Unregistered command executed",
				slog.String("command", data.Name),
				slog.String("interaction_id", event.ID.String()),
			)
			return ErrCommandNotFound
		}

		ctx, err := NewArikawaContext(*event, r.config)
		if err != nil {
			slog.Warn("Intercepted service degradation: Invalid interaction context",
				slog.String("interaction_id", event.ID.String()),
				slog.Any("error", err),
			)
			return err
		}
		ctx.SetClient(r.client)

		if err := cmd.Handle(ctx); err != nil && !errors.Is(err, ErrAlreadyAcknowledged) {
			r.logHandlerError("command", data.Name, event, err)
			return err
		}
		return nil

	default:
		// Attempt to extract CustomID if it implements discord.ComponentID
		if cmp, ok := data.(interface{ ID() discord.ComponentID }); ok {
			rawID := string(cmp.ID())

			var handler ComponentHandler
			var matchedID string

			// Operational Annotation: We iterate prefixes to support dynamically
			// generated suffixes (e.g. `role|12345`). Since map iteration is random,
			// overlapping prefixes may yield non-deterministic routing. Use distinct namespaces.
			for prefix, h := range r.components {
				if strings.HasPrefix(rawID, prefix) {
					handler = h
					matchedID = prefix
					break
				}
			}

			if handler != nil {
				ctx, err := NewArikawaContext(*event, r.config)
				if err != nil {
					slog.Warn("Intercepted service degradation: Invalid interaction context",
						slog.String("interaction_id", event.ID.String()),
						slog.Any("error", err),
					)
					return err
				}
				ctx.SetClient(r.client)

				if err := handler.HandleComponent(ctx); err != nil && !errors.Is(err, ErrAlreadyAcknowledged) {
					r.logHandlerError("component", matchedID, event, err)
					return err
				}
			} else {
				slog.Warn("Intercepted service degradation: Unregistered component executed",
					slog.String("custom_id", rawID),
					slog.String("interaction_id", event.ID.String()),
				)
			}
		}
		return nil
	}
}

func (r *CommandRouter) logHandlerError(kind, name string, event *discord.InteractionEvent, err error) {
	logger := log.ErrorLoggerRaw()
	if logger == nil {
		logger = slog.Default()
	}

	logger.Error("Arikawa handler execution failed",
		slog.String("kind", kind),
		slog.String("name", name),
		slog.String("request_id", event.ID.String()),
		slog.Any("error", err),
		slog.Any("stack_trace", log.LazyStackTrace{}),
	)
}

// Registry grants read-only access to the underlying registry.
func (r *CommandRouter) Registry() *CommandRegistry {
	return r.registry
}
