package config

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type configCommandVisibilityClass string

const (
	configCommandVisibilityRead              configCommandVisibilityClass = "read"
	configCommandVisibilityList              configCommandVisibilityClass = "list"
	configCommandVisibilityDetailedError     configCommandVisibilityClass = "detailed_error"
	configCommandVisibilityShortConfirmation configCommandVisibilityClass = "short_confirmation"
)

func configCommandVisibilityIsEphemeral(class configCommandVisibilityClass) bool {
	switch class {
	case configCommandVisibilityShortConfirmation:
		return false
	case configCommandVisibilityRead,
		configCommandVisibilityList,
		configCommandVisibilityDetailedError:
		return true
	default:
		return true
	}
}

func configCommandResponseBuilder(session *discordgo.Session, class configCommandVisibilityClass) *core.ResponseBuilder {
	builder := core.NewResponseBuilder(session)
	if configCommandVisibilityIsEphemeral(class) {
		builder = builder.Ephemeral()
	}
	return builder
}

func configCommandError(class configCommandVisibilityClass, message string) error {
	return core.NewCommandError(message, configCommandVisibilityIsEphemeral(class))
}

func configCommandDetailedCommandError(message string) error {
	return configCommandError(configCommandVisibilityDetailedError, message)
}

func configCommandCurrentStateResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return configCommandResponseBuilder(session, configCommandVisibilityRead)
}

func configCommandAvailableOptionsResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return configCommandResponseBuilder(session, configCommandVisibilityList)
}

func configCommandShortConfirmationResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return configCommandResponseBuilder(session, configCommandVisibilityShortConfirmation)
}