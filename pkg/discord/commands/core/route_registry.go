package core

import "strings"

type interactionRouteRegistry struct {
	routes map[string]*interactionRouteEntry
}

type interactionRouteEntry struct {
	slash        slashRouteEntry
	autocomplete autocompleteRouteEntry
	component    componentRouteEntry
	modal        modalRouteEntry
}

type slashRouteEntry struct {
	handler  SlashHandler
	ackPolicy InteractionAckPolicy
	explicit bool
}

type autocompleteRouteEntry struct {
	handler  AutocompleteHandler
	ackPolicy InteractionAckPolicy
	explicit bool
}

type componentRouteEntry struct {
	handler  ComponentHandler
	ackPolicy InteractionAckPolicy
	explicit bool
}

type modalRouteEntry struct {
	handler  ModalHandler
	ackPolicy InteractionAckPolicy
	explicit bool
}

func newInteractionRouteRegistry() *interactionRouteRegistry {
	return &interactionRouteRegistry{
		routes: make(map[string]*interactionRouteEntry),
	}
}

// RegisterInteractionRoute registers any supported interaction handlers under
// the same normalized route path or stable route ID.
func (cr *CommandRouter) RegisterInteractionRoute(binding InteractionRouteBinding) {
	cr.RegisterInteractionRoutes(binding)
}

// RegisterInteractionRoutes registers one or more interaction route bindings.
func (cr *CommandRouter) RegisterInteractionRoutes(bindings ...InteractionRouteBinding) {
	cr.registerInteractionRoutes(true, bindings...)
}

// RegisterSlashRoute registers a slash route by canonical route path.
func (cr *CommandRouter) RegisterSlashRoute(routePath string, handler SlashHandler) {
	cr.RegisterInteractionRoute(InteractionRouteBinding{Path: routePath, Slash: handler})
}

// RegisterAutocompleteRoute registers an autocomplete route by canonical route path.
func (cr *CommandRouter) RegisterAutocompleteRoute(routePath string, handler AutocompleteHandler) {
	cr.RegisterInteractionRoute(InteractionRouteBinding{Path: routePath, Autocomplete: handler})
}

// RegisterComponentRoute registers a component route by stable route ID.
func (cr *CommandRouter) RegisterComponentRoute(routeID string, handler ComponentHandler) {
	cr.RegisterInteractionRoute(InteractionRouteBinding{Path: routeID, Component: handler})
}

// RegisterModalRoute registers a modal route by stable route ID.
func (cr *CommandRouter) RegisterModalRoute(routeID string, handler ModalHandler) {
	cr.RegisterInteractionRoute(InteractionRouteBinding{Path: routeID, Modal: handler})
}

func (cr *CommandRouter) lookupSlashHandler(routeKey InteractionRouteKey) (SlashHandler, bool) {
	entry, exists := cr.lookupInteractionRouteEntry(routeKey.Path)
	if !exists || entry.slash.handler == nil {
		return nil, false
	}
	return entry.slash.handler, true
}

func (cr *CommandRouter) lookupAutocompleteHandler(routeKey InteractionRouteKey) (AutocompleteHandler, bool) {
	entry, exists := cr.lookupInteractionRouteEntry(routeKey.Path)
	if !exists || entry.autocomplete.handler == nil {
		return nil, false
	}
	return entry.autocomplete.handler, true
}

func (cr *CommandRouter) lookupComponentHandler(routeKey InteractionRouteKey) (ComponentHandler, bool) {
	entry, exists := cr.lookupInteractionRouteEntry(routeKey.Path)
	if !exists || entry.component.handler == nil {
		return nil, false
	}
	return entry.component.handler, true
}

func (cr *CommandRouter) lookupModalHandler(routeKey InteractionRouteKey) (ModalHandler, bool) {
	entry, exists := cr.lookupInteractionRouteEntry(routeKey.Path)
	if !exists || entry.modal.handler == nil {
		return nil, false
	}
	return entry.modal.handler, true
}

func (cr *CommandRouter) lookupInteractionRouteEntry(path string) (*interactionRouteEntry, bool) {
	if cr == nil || cr.routeRegistry == nil {
		return nil, false
	}
	path = JoinRoutePath(path)
	if path == "" {
		return nil, false
	}
	entry, exists := cr.routeRegistry.routes[path]
	return entry, exists
}

func (cr *CommandRouter) lookupInteractionAckPolicy(routeKey InteractionRouteKey) (InteractionAckPolicy, bool) {
	entry, exists := cr.lookupInteractionRouteEntry(routeKey.Path)
	if !exists || entry == nil {
		return InteractionAckPolicy{}, false
	}

	switch routeKey.Kind {
	case InteractionKindSlash:
		if entry.slash.handler == nil {
			return InteractionAckPolicy{}, false
		}
		return entry.slash.ackPolicy, true
	case InteractionKindAutocomplete:
		if entry.autocomplete.handler == nil {
			return InteractionAckPolicy{}, false
		}
		return entry.autocomplete.ackPolicy, true
	case InteractionKindComponent:
		if entry.component.handler == nil {
			return InteractionAckPolicy{}, false
		}
		return entry.component.ackPolicy, true
	case InteractionKindModal:
		if entry.modal.handler == nil {
			return InteractionAckPolicy{}, false
		}
		return entry.modal.ackPolicy, true
	default:
		return InteractionAckPolicy{}, false
	}
}

func (cr *CommandRouter) registerSlashCommandRoutes(cmd Command) {
	if cr == nil || cr.routeRegistry == nil || cmd == nil {
		return
	}
	cr.registerDerivedInteractionRouteTree(strings.TrimSpace(cmd.Name()), cmd)
}

func (cr *CommandRouter) registerSlashSubCommandRoutes(parentName string, subcmd SubCommand) {
	if cr == nil || cr.routeRegistry == nil || subcmd == nil {
		return
	}
	cr.registerDerivedInteractionRouteTree(JoinRoutePath(parentName, subcmd.Name()), subcmd)
}

func (cr *CommandRouter) registerDerivedInteractionRouteTree(path string, handler SlashHandler) {
	cr.registerInteractionRoutes(false, collectInteractionRouteBindings(path, handler)...)
}

func (cr *CommandRouter) registerInteractionRoutes(explicit bool, bindings ...InteractionRouteBinding) {
	if cr == nil || cr.routeRegistry == nil {
		return
	}
	for _, binding := range bindings {
		cr.storeInteractionRoute(binding, explicit)
	}
}

func (cr *CommandRouter) storeInteractionRoute(binding InteractionRouteBinding, explicit bool) {
	if cr == nil || cr.routeRegistry == nil || !binding.hasHandlers() {
		return
	}
	path := JoinRoutePath(binding.Path)
	if path == "" {
		return
	}

	entry := cr.routeRegistry.routes[path]
	if entry == nil {
		entry = &interactionRouteEntry{}
		cr.routeRegistry.routes[path] = entry
	}

	if binding.Slash != nil && !(entry.slash.explicit && !explicit) {
		entry.slash = slashRouteEntry{handler: binding.Slash, ackPolicy: binding.AckPolicy, explicit: explicit}
	}
	if binding.Autocomplete != nil && !(entry.autocomplete.explicit && !explicit) {
		entry.autocomplete = autocompleteRouteEntry{handler: binding.Autocomplete, ackPolicy: binding.AckPolicy, explicit: explicit}
	}
	if binding.Component != nil && !(entry.component.explicit && !explicit) {
		entry.component = componentRouteEntry{handler: binding.Component, ackPolicy: binding.AckPolicy, explicit: explicit}
	}
	if binding.Modal != nil && !(entry.modal.explicit && !explicit) {
		entry.modal = modalRouteEntry{handler: binding.Modal, ackPolicy: binding.AckPolicy, explicit: explicit}
	}
}

func collectInteractionRouteBindings(path string, handler SlashHandler) []InteractionRouteBinding {
	path = strings.TrimSpace(path)
	if path == "" || handler == nil {
		return nil
	}

	binding := InteractionRouteBinding{Path: path, Slash: handler}
	if provider, ok := handler.(AutocompleteRouteProvider); ok {
		binding.Autocomplete = provider.AutocompleteRouteHandler()
	}
	if provider, ok := handler.(InteractionAckPolicyProvider); ok {
		binding.AckPolicy = provider.InteractionAckPolicy()
	}
	bindings := []InteractionRouteBinding{binding}

	group, ok := handler.(*GroupCommand)
	if !ok {
		return bindings
	}

	for _, subcmd := range group.subcommands {
		childPath := JoinRoutePath(path, subcmd.Name())
		bindings = append(bindings, collectInteractionRouteBindings(childPath, subcmd)...)
	}

	return bindings
}

// JoinRoutePath normalizes and joins route path segments with spaces.
func JoinRoutePath(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		for _, field := range strings.Fields(part) {
			if field == "" {
				continue
			}
			filtered = append(filtered, field)
		}
	}
	return strings.Join(filtered, " ")
}