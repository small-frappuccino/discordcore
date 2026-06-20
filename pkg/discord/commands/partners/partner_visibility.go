package partners

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordgo"
)

type partnerVisibilityClass string

const (
	partnerVisibilityEntryMutation        partnerVisibilityClass = "entry_mutation"
	partnerVisibilityEntryRead            partnerVisibilityClass = "entry_read"
	partnerVisibilityBoardState           partnerVisibilityClass = "board_state"
	partnerVisibilityAdministrativeAction partnerVisibilityClass = "administrative_action"
	partnerVisibilityDetailedError        partnerVisibilityClass = "detailed_error"
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

func partnerResponseBuilder(session *discordgo.Session, class partnerVisibilityClass) *legacycore.ResponseBuilder {
	builder := legacycore.NewResponseBuilder(session)
	if partnerVisibilityIsEphemeral(class) {
		builder = builder.Ephemeral()
	}
	return builder
}

func partnerCommandError(class partnerVisibilityClass, message string) error {
	return &legacycore.CommandError{Message: message, Ephemeral: partnerVisibilityIsEphemeral(class)}
}

func partnerDetailedCommandError(message string) error {
	return partnerCommandError(partnerVisibilityDetailedError, message)
}

func partnerEntryMutationResponseBuilder(session *discordgo.Session) *legacycore.ResponseBuilder {
	return partnerResponseBuilder(session, partnerVisibilityEntryMutation)
}

func partnerEntryReadResponseBuilder(session *discordgo.Session) *legacycore.ResponseBuilder {
	return partnerResponseBuilder(session, partnerVisibilityEntryRead)
}

func partnerBoardStateResponseBuilder(session *discordgo.Session) *legacycore.ResponseBuilder {
	return partnerResponseBuilder(session, partnerVisibilityBoardState)
}

func partnerAdministrativeActionResponseBuilder(session *discordgo.Session) *legacycore.ResponseBuilder {
	return partnerResponseBuilder(session, partnerVisibilityAdministrativeAction)
}
