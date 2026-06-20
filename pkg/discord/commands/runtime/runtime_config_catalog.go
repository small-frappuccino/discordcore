package runtime

import (
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// runtimeInteractionCatalog keeps the runtime-config interaction wiring local
// to the feature so the registrar only applies the declared command tree and
// interaction bindings.
type runtimeInteractionCatalog struct {
	configManager *files.ConfigManager
}

func newRuntimeInteractionCatalog(configManager *files.ConfigManager) runtimeInteractionCatalog {
	return runtimeInteractionCatalog{configManager: configManager}
}

func (catalog runtimeInteractionCatalog) register(router *legacycore.CommandRouter) {
	if router == nil {
		return
	}

	catalog.registerSlashTree(router)
	router.RegisterInteractionRoutes(catalog.bindings()...)
}

func (catalog runtimeInteractionCatalog) registerSlashTree(router *legacycore.CommandRouter) {
	runtimeCommand := newRuntimeSubCommand(catalog.configManager)

	if existing, ok := router.GetRegistry().GetCommand(groupName); ok {
		if group, ok := existing.(*legacycore.GroupCommand); ok {
			group.AddSubCommand(runtimeCommand)
			router.RegisterSlashCommand(group)
			return
		}
	}

	checker := legacycore.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	group := legacycore.NewGroupCommand(groupName, "Manage server configuration", checker)
	group.AddSubCommand(runtimeCommand)
	router.RegisterSlashCommand(group)
}

func (catalog runtimeInteractionCatalog) bindings() []legacycore.InteractionRouteBinding {
	componentHandler := catalog.componentHandler()
	bindings := make([]legacycore.InteractionRouteBinding, 0, len(runtimeComponentRouteIDs())+1)
	for _, routeID := range runtimeComponentRouteIDs() {
		bindings = append(bindings, legacycore.InteractionRouteBinding{
			Path:      routeID,
			Component: componentHandler,
			AckPolicy: runtimeComponentAckPolicy(routeID),
		})
	}

	bindings = append(bindings, legacycore.InteractionRouteBinding{
		Path:      modalEditValueID,
		Modal:     catalog.modalHandler(),
		AckPolicy: legacycore.InteractionAckPolicy{Mode: legacycore.InteractionAckModeDefer},
	})

	return bindings
}

func (catalog runtimeInteractionCatalog) componentHandler() legacycore.ComponentHandler {
	return legacycore.ComponentHandlerFunc(func(ctx *legacycore.Context) error {
		if ctx == nil || ctx.Session == nil || ctx.Interaction == nil {
			return nil
		}

		done := startRuntimeConfigInteractionTrace(ctx.Interaction)
		defer done()

		ackPolicy := runtimeComponentAckPolicy(ctx.RouteKey.Path)
		handled, err := authorizeRuntimeComponentInteraction(ctx, ackPolicy)
		if err != nil {
			return fmt.Errorf("runtimeInteractionCatalog.componentHandler: %w", err)
		}
		if handled {
			return nil
		}

		handleComponent(ctx.Session, ctx.Interaction, catalog.configManager, runtimeInteractionApplier(ctx))
		return nil
	})
}

func (catalog runtimeInteractionCatalog) modalHandler() legacycore.ModalHandler {
	return legacycore.ModalHandlerFunc(func(ctx *legacycore.Context) error {
		if ctx == nil || ctx.Session == nil || ctx.Interaction == nil {
			return nil
		}

		done := startRuntimeConfigInteractionTrace(ctx.Interaction)
		defer done()

		ackPolicy := legacycore.InteractionAckPolicy{Mode: legacycore.InteractionAckModeDefer}
		handled, err := authorizeRuntimeModalInteraction(ctx, ackPolicy)
		if err != nil {
			return fmt.Errorf("runtimeInteractionCatalog.modalHandler: %w", err)
		}
		if handled {
			return nil
		}

		handleModalSubmit(ctx.Session, ctx.Interaction, catalog.configManager, runtimeInteractionApplier(ctx))
		return nil
	})
}

func runtimeComponentRouteIDs() []string {
	return []string{
		cidSelectKey,
		cidSelectGroup,
		cidButtonMain,
		cidButtonHelp,
		cidButtonBack,
		cidButtonDetail,
		cidButtonToggle,
		cidButtonEdit,
		cidButtonReset,
		cidButtonReload,
	}
}

func runtimeComponentAckPolicy(routeID string) legacycore.InteractionAckPolicy {
	if routeID == cidButtonEdit {
		return legacycore.InteractionAckPolicy{}
	}

	return legacycore.InteractionAckPolicy{Mode: legacycore.InteractionAckModeDefer}
}
