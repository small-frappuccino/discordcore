package core

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// HandleInteraction routes interactions to the appropriate handlers.
//
// PR 1 keeps legacy slash and autocomplete behavior intact while moving the
// dispatch entrypoint out of registry.go so later work can add component and
// modal routing without regrowing the command sync hotspot.
func (cr *CommandRouter) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil {
		return
	}
	if !cr.shouldHandleGuild(i.GuildID) {
		return
	}

	routeKey := resolveInteractionRouteKey(i)
	switch routeKey.Kind {
	case InteractionKindAutocomplete:
		cr.handleAutocompleteRoute(i, routeKey)
	case InteractionKindSlash:
		cr.handleSlashCommandRoute(i, routeKey)
	case InteractionKindComponent:
		cr.handleComponentRoute(i, routeKey)
	case InteractionKindModal:
		cr.handleModalRoute(i, routeKey)
	default:
		return
	}
}

// handleSlashCommand processes slash commands.
// Kept as a compatibility wrapper for existing tests and call sites.
func (cr *CommandRouter) handleSlashCommand(i *discordgo.InteractionCreate) {
	cr.handleSlashCommandRoute(i, InteractionRouteKey{
		Kind: InteractionKindSlash,
		Path: commandRoutePath(i),
	})
}

func (cr *CommandRouter) handleSlashCommandRoute(i *discordgo.InteractionCreate, routeKey InteractionRouteKey) {
	if routeKey.Path == "" {
		routeKey.Path = commandRoutePath(i)
	}

	ctx := cr.contextBuilder.BuildContext(i)
	err := cr.executeRoute(ctx, routeKey, func(ctx *Context) error {
		handler, exists := cr.lookupSlashHandler(ctx.RouteKey)
		if !exists {
			return NewCommandError("Command not found", true)
		}
		return handler.Handle(ctx)
	})
	if err != nil {
		slog.Error("Slash route returned unmapped error", "commandPath", routeKey.Path, "err", err)
		respondToSlashError(ctx, err)
	}
}

// handleAutocomplete processes autocomplete interactions.
// Kept as a compatibility wrapper for existing tests and call sites.
func (cr *CommandRouter) handleAutocomplete(i *discordgo.InteractionCreate) {
	cr.handleAutocompleteRoute(i, InteractionRouteKey{
		Kind:          InteractionKindAutocomplete,
		Path:          commandRoutePath(i),
		FocusedOption: focusedOptionName(i),
	})
}

func (cr *CommandRouter) handleAutocompleteRoute(i *discordgo.InteractionCreate, routeKey InteractionRouteKey) {
	if routeKey.Path == "" {
		routeKey.Path = commandRoutePath(i)
	}
	if routeKey.FocusedOption == "" {
		routeKey.FocusedOption = focusedOptionName(i)
	}

	ctx := cr.contextBuilder.BuildContext(i)
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	err := cr.executeRoute(ctx, routeKey, func(ctx *Context) error {
		handler, exists := cr.lookupAutocompleteHandler(ctx.RouteKey)
		if !exists || ctx.RouteKey.FocusedOption == "" {
			return nil
		}

		var err error
		choices, err = handler.HandleAutocomplete(ctx, ctx.RouteKey.FocusedOption)
		return err
	})
	if err != nil {
		slog.Error("Autocomplete handler failed", "err", err)
		choices = []*discordgo.ApplicationCommandOptionChoice{}
	}

	NewResponseBuilder(ctx.Session).Build().Autocomplete(i, choices)
}


func (cr *CommandRouter) handleComponentRoute(i *discordgo.InteractionCreate, routeKey InteractionRouteKey) {
	if routeKey.Path == "" {
		routeKey.Path = interactionCustomRouteID(interactionCustomID(i))
	}
	if routeKey.CustomID == "" {
		routeKey.CustomID = interactionCustomID(i)
	}

	ctx := cr.contextBuilder.BuildContext(i)
	err := cr.executeRoute(ctx, routeKey, func(ctx *Context) error {
		handler, exists := cr.lookupComponentHandler(ctx.RouteKey)
		if !exists {
			return nil
		}
		return handler.HandleComponent(ctx)
	})
	if err != nil {
		slog.Error("Component handler failed", "routeID", routeKey.Path, "customID", routeKey.CustomID, "err", err)
	}
}

func (cr *CommandRouter) handleModalRoute(i *discordgo.InteractionCreate, routeKey InteractionRouteKey) {
	if routeKey.Path == "" {
		routeKey.Path = interactionCustomRouteID(interactionCustomID(i))
	}
	if routeKey.CustomID == "" {
		routeKey.CustomID = interactionCustomID(i)
	}

	ctx := cr.contextBuilder.BuildContext(i)
	err := cr.executeRoute(ctx, routeKey, func(ctx *Context) error {
		handler, exists := cr.lookupModalHandler(ctx.RouteKey)
		if !exists {
			return nil
		}
		return handler.HandleModal(ctx)
	})
	if err != nil {
		slog.Error("Modal handler failed", "routeID", routeKey.Path, "customID", routeKey.CustomID, "err", err)
	}
}