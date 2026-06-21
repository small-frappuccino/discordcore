package commands

import (
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
)

// ArikawaCommand defines the strict contract for an Arikawa-native slash command.
// This interface abstracts vertical domains from the raw router execution loop.
type ArikawaCommand interface {
	Name() string
	Description() string
	Options() []discord.CommandOption
	Handle(ctx *ArikawaContext) error
	RequiresGuild() bool
	RequiresPermissions() bool
}

// DefaultMemberPermissionsProvider specifies optional member permission floors.
type DefaultMemberPermissionsProvider interface {
	DefaultMemberPermissions() discord.Permissions
}

// ComponentHandler interface for components.
type ComponentHandler interface {
	HandleComponent(ctx *ArikawaContext) error
}

// ModalHandler interface for modals.
type ModalHandler interface {
	HandleModal(ctx *ArikawaContext) error
}

// AutocompleteHandler interface for autocompletes.
type AutocompleteHandler interface {
	HandleAutocomplete(ctx *ArikawaContext, focusedOption string) (api.AutocompleteChoices, error)
}

// InteractionRouteKey represents a unique routing path for a given command.
type InteractionRouteKey struct {
	Path string
}

// ArikawaRegisterer is the interface that allows domain commands to register themselves.
type ArikawaRegisterer interface {
	Register(cmd ArikawaCommand)
	RegisterComponent(customIDPrefix string, handler ComponentHandler)
}
