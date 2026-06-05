package roles

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

// rolePanelVisibilityClass classifies the outward visibility of a role panel
// command reply. Configuration-style replies stay ephemeral so the channel
// stays focused on the published panel itself.
type rolePanelVisibilityClass string

const (
	rolePanelVisibilityConfiguration rolePanelVisibilityClass = "configuration"
	rolePanelVisibilityPreview       rolePanelVisibilityClass = "preview"
	rolePanelVisibilityToggle        rolePanelVisibilityClass = "toggle"
	rolePanelVisibilityDetailedError rolePanelVisibilityClass = "detailed_error"
)

func rolePanelVisibilityIsEphemeral(class rolePanelVisibilityClass) bool {
	switch class {
	case rolePanelVisibilityConfiguration,
		rolePanelVisibilityPreview,
		rolePanelVisibilityToggle,
		rolePanelVisibilityDetailedError:
		return true
	default:
		return true
	}
}

func rolePanelResponseBuilder(session *discordgo.Session, class rolePanelVisibilityClass) *core.ResponseBuilder {
	builder := core.NewResponseBuilder(session)
	if rolePanelVisibilityIsEphemeral(class) {
		builder = builder.Ephemeral()
	}
	return builder
}

func rolePanelDetailedCommandError(message string) error {
	return &core.CommandError{Message: message, Ephemeral: rolePanelVisibilityIsEphemeral(rolePanelVisibilityDetailedError)}
}

func rolePanelConfigurationResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return rolePanelResponseBuilder(session, rolePanelVisibilityConfiguration)
}

func rolePanelPreviewResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return rolePanelResponseBuilder(session, rolePanelVisibilityPreview)
}

func rolePanelToggleResponseBuilder(ctx *core.Context) *core.ResponseBuilder {
	disableEphemeral := false
	if ctx != nil {
		if ctx.GuildConfig != nil {
			disableEphemeral = ctx.GuildConfig.RuntimeConfig.DisableInteractiveEphemeral
		} else if ctx.Config != nil && ctx.GuildID != "" {
			if gc := ctx.Config.GuildConfig(ctx.GuildID); gc != nil {
				disableEphemeral = gc.RuntimeConfig.DisableInteractiveEphemeral
			}
		}
	}

	builder := core.NewResponseBuilder(ctx.Session)
	if !disableEphemeral && rolePanelVisibilityIsEphemeral(rolePanelVisibilityToggle) {
		builder = builder.Ephemeral()
	}
	return builder
}
