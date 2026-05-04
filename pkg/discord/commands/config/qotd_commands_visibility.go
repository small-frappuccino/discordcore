package config

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type qotdConfigVisibilityClass string

const (
	qotdConfigVisibilityDetailedError     qotdConfigVisibilityClass = "detailed_error"
	qotdConfigVisibilityShortConfirmation qotdConfigVisibilityClass = "short_confirmation"
)

func qotdConfigVisibilityIsEphemeral(class qotdConfigVisibilityClass) bool {
	switch class {
	case qotdConfigVisibilityShortConfirmation:
		return false
	case qotdConfigVisibilityDetailedError:
		return true
	default:
		return true
	}
}

func qotdConfigResponseBuilder(session *discordgo.Session, class qotdConfigVisibilityClass) *core.ResponseBuilder {
	builder := core.NewResponseBuilder(session)
	if qotdConfigVisibilityIsEphemeral(class) {
		builder = builder.Ephemeral()
	}
	return builder
}

func qotdConfigCommandError(class qotdConfigVisibilityClass, message string) error {
	return core.NewCommandError(message, qotdConfigVisibilityIsEphemeral(class))
}

func qotdConfigDetailedCommandError(message string) error {
	return qotdConfigCommandError(qotdConfigVisibilityDetailedError, message)
}

func qotdConfigShortConfirmationResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return qotdConfigResponseBuilder(session, qotdConfigVisibilityShortConfirmation)
}