package config

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type serviceConfigVisibilityClass string

const (
	serviceConfigVisibilitySetupState       serviceConfigVisibilityClass = "setup_state"
	serviceConfigVisibilityDetailedError   serviceConfigVisibilityClass = "detailed_error"
	serviceConfigVisibilityShortConfirmation serviceConfigVisibilityClass = "short_confirmation"
)

func serviceConfigVisibilityIsEphemeral(class serviceConfigVisibilityClass) bool {
	switch class {
	case serviceConfigVisibilityShortConfirmation:
		return false
	case serviceConfigVisibilitySetupState,
		serviceConfigVisibilityDetailedError:
		return true
	default:
		return true
	}
}

func serviceConfigResponseBuilder(session *discordgo.Session, class serviceConfigVisibilityClass) *core.ResponseBuilder {
	builder := core.NewResponseBuilder(session)
	if serviceConfigVisibilityIsEphemeral(class) {
		builder = builder.Ephemeral()
	}
	return builder
}

func serviceConfigCommandError(class serviceConfigVisibilityClass, message string) error {
	return core.NewCommandError(message, serviceConfigVisibilityIsEphemeral(class))
}

func serviceConfigDetailedCommandError(message string) error {
	return serviceConfigCommandError(serviceConfigVisibilityDetailedError, message)
}

func serviceConfigSetupStateResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return serviceConfigResponseBuilder(session, serviceConfigVisibilitySetupState)
}

func serviceConfigShortConfirmationResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return serviceConfigResponseBuilder(session, serviceConfigVisibilityShortConfirmation)
}