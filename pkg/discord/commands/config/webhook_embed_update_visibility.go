package config

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type webhookEmbedVisibilityClass string

const (
	webhookEmbedVisibilityRead              webhookEmbedVisibilityClass = "read"
	webhookEmbedVisibilityList              webhookEmbedVisibilityClass = "list"
	webhookEmbedVisibilityPreview           webhookEmbedVisibilityClass = "preview"
	webhookEmbedVisibilityRenderedPayload   webhookEmbedVisibilityClass = "rendered_payload"
	webhookEmbedVisibilityDetailedError     webhookEmbedVisibilityClass = "detailed_error"
	webhookEmbedVisibilityShortConfirmation webhookEmbedVisibilityClass = "short_confirmation"
)

func webhookEmbedVisibilityIsEphemeral(class webhookEmbedVisibilityClass) bool {
	switch class {
	case webhookEmbedVisibilityShortConfirmation:
		return false
	case webhookEmbedVisibilityRead,
		webhookEmbedVisibilityList,
		webhookEmbedVisibilityPreview,
		webhookEmbedVisibilityRenderedPayload,
		webhookEmbedVisibilityDetailedError:
		return true
	default:
		return true
	}
}

func webhookEmbedResponseBuilder(session *discordgo.Session, class webhookEmbedVisibilityClass) *core.ResponseBuilder {
	builder := core.NewResponseBuilder(session)
	if webhookEmbedVisibilityIsEphemeral(class) {
		builder = builder.Ephemeral()
	}
	return builder
}

func webhookEmbedCommandError(class webhookEmbedVisibilityClass, message string) error {
	return core.NewCommandError(message, webhookEmbedVisibilityIsEphemeral(class))
}