package runtime

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
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

func (catalog runtimeInteractionCatalog) register(router *core.CommandRouter) {
	if router == nil {
		return
	}

	catalog.registerSlashTree(router)
	router.RegisterInteractionRoutes(catalog.bindings()...)
}

func (catalog runtimeInteractionCatalog) registerSlashTree(router *core.CommandRouter) {
	runtimeCommand := newRuntimeSubCommand(catalog.configManager)

	if existing, ok := router.GetRegistry().GetCommand(groupName); ok {
		if group, ok := existing.(*core.GroupCommand); ok {
			group.AddSubCommand(runtimeCommand)
			router.RegisterSlashCommand(group)
			return
		}
	}

	checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	group := core.NewGroupCommand(groupName, "Manage server configuration", checker)
	group.AddSubCommand(runtimeCommand)
	router.RegisterSlashCommand(group)
}

func (catalog runtimeInteractionCatalog) bindings() []core.InteractionRouteBinding {
	componentHandler := catalog.componentHandler()
	bindings := make([]core.InteractionRouteBinding, 0, len(runtimeComponentRouteIDs())+1)
	for _, routeID := range runtimeComponentRouteIDs() {
		bindings = append(bindings, core.InteractionRouteBinding{
			Path:      routeID,
			Component: componentHandler,
			AckPolicy: runtimeComponentAckPolicy(routeID),
		})
	}

	bindings = append(bindings, core.InteractionRouteBinding{
		Path:      modalEditValueID,
		Modal:     catalog.modalHandler(),
		AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeDefer},
	})

	return bindings
}

func (catalog runtimeInteractionCatalog) componentHandler() core.ComponentHandler {
	return core.ComponentHandlerFunc(func(ctx *core.Context) error {
		if ctx == nil || ctx.Session == nil || ctx.Interaction == nil {
			return nil
		}

		done := startRuntimeConfigInteractionTrace(ctx.Interaction)
		defer done()

		ackPolicy := runtimeComponentAckPolicy(ctx.RouteKey.Path)
		handled, err := authorizeRuntimeComponentInteraction(ctx, ackPolicy)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}

		handleComponent(ctx.Session, ctx.Interaction, catalog.configManager, runtimeInteractionApplier(ctx))
		return nil
	})
}

func (catalog runtimeInteractionCatalog) modalHandler() core.ModalHandler {
	return core.ModalHandlerFunc(func(ctx *core.Context) error {
		if ctx == nil || ctx.Session == nil || ctx.Interaction == nil {
			return nil
		}

		done := startRuntimeConfigInteractionTrace(ctx.Interaction)
		defer done()

		ackPolicy := core.InteractionAckPolicy{Mode: core.InteractionAckModeDefer}
		handled, err := authorizeRuntimeModalInteraction(ctx, ackPolicy)
		if err != nil {
			return err
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

func runtimeComponentAckPolicy(routeID string) core.InteractionAckPolicy {
	if routeID == cidButtonEdit {
		return core.InteractionAckPolicy{}
	}

	return core.InteractionAckPolicy{Mode: core.InteractionAckModeDefer}
}