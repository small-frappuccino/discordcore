package core

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func resolveInteractionRouteKey(i *discordgo.InteractionCreate) InteractionRouteKey {
	if i == nil {
		return InteractionRouteKey{}
	}

	switch {
	case IsAutocompleteInteraction(i):
		return InteractionRouteKey{
			Kind:          InteractionKindAutocomplete,
			Path:          commandRoutePath(i),
			FocusedOption: focusedOptionName(i),
		}
	case IsSlashCommandInteraction(i):
		return InteractionRouteKey{
			Kind: InteractionKindSlash,
			Path: commandRoutePath(i),
		}
	case i.Type == discordgo.InteractionMessageComponent:
		customID := interactionCustomID(i)
		return InteractionRouteKey{
			Kind:     InteractionKindComponent,
			Path:     interactionCustomRouteID(customID),
			CustomID: customID,
		}
	case i.Type == discordgo.InteractionModalSubmit:
		customID := interactionCustomID(i)
		return InteractionRouteKey{
			Kind:     InteractionKindModal,
			Path:     interactionCustomRouteID(customID),
			CustomID: customID,
		}
	default:
		return InteractionRouteKey{}
	}
}

func commandRoutePath(i *discordgo.InteractionCreate) string {
	if i == nil {
		return ""
	}
	return GetCommandPath(i)
}

func focusedOptionName(i *discordgo.InteractionCreate) string {
	if i == nil {
		return ""
	}

	focusedOpt, hasFocus := HasFocusedOption(i.ApplicationCommandData().Options)
	if !hasFocus || focusedOpt == nil {
		return ""
	}

	return focusedOpt.Name
}

func interactionCustomID(i *discordgo.InteractionCreate) string {
	if i == nil {
		return ""
	}

	switch i.Type {
	case discordgo.InteractionMessageComponent:
		return i.MessageComponentData().CustomID
	case discordgo.InteractionModalSubmit:
		return i.ModalSubmitData().CustomID
	default:
		return ""
	}
}

func interactionCustomRouteID(customID string) string {
	if customID == "" {
		return ""
	}
	routeID, _, found := strings.Cut(customID, "|")
	if !found {
		return customID
	}
	return routeID
}