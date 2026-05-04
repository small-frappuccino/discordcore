package partner

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type partnerVisibilityClass string

const (
	partnerVisibilityEntryMutation      partnerVisibilityClass = "entry_mutation"
	partnerVisibilityEntryRead          partnerVisibilityClass = "entry_read"
	partnerVisibilityBoardState         partnerVisibilityClass = "board_state"
	partnerVisibilityAdministrativeAction partnerVisibilityClass = "administrative_action"
	partnerVisibilityDetailedError      partnerVisibilityClass = "detailed_error"
)

func partnerVisibilityIsEphemeral(class partnerVisibilityClass) bool {
	switch class {
	case partnerVisibilityEntryMutation,
		partnerVisibilityEntryRead,
		partnerVisibilityBoardState,
		partnerVisibilityAdministrativeAction,
		partnerVisibilityDetailedError:
		return true
	default:
		return true
	}
}

func partnerResponseBuilder(session *discordgo.Session, class partnerVisibilityClass) *core.ResponseBuilder {
	builder := core.NewResponseBuilder(session)
	if partnerVisibilityIsEphemeral(class) {
		builder = builder.Ephemeral()
	}
	return builder
}

func partnerCommandError(class partnerVisibilityClass, message string) error {
	return core.NewCommandError(message, partnerVisibilityIsEphemeral(class))
}

func partnerDetailedCommandError(message string) error {
	return partnerCommandError(partnerVisibilityDetailedError, message)
}

func partnerEntryMutationResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return partnerResponseBuilder(session, partnerVisibilityEntryMutation)
}

func partnerEntryReadResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return partnerResponseBuilder(session, partnerVisibilityEntryRead)
}

func partnerBoardStateResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return partnerResponseBuilder(session, partnerVisibilityBoardState)
}

func partnerAdministrativeActionResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return partnerResponseBuilder(session, partnerVisibilityAdministrativeAction)
}