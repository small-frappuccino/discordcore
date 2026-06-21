package commands

import (
	"fmt"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

// NewArikawaMissingConfigErrorData returns a generic error payload for missing config.
func NewArikawaMissingConfigErrorData(guildID, feature, commandPath string) api.InteractionResponseData {
	return api.InteractionResponseData{
		Content: option.NewNullableString(fmt.Sprintf("❌ Configuration missing for %s. Please ensure it is configured in the dashboard.", feature)),
		Flags:   discord.EphemeralMessage,
	}
}
