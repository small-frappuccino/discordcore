package embeds

import (
	"errors"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// ErrPostingMissing is returned when a posting's channel or message can no longer be found.
var ErrPostingMissing = errors.New("embed posting is missing or inaccessible")

// Publisher abstracts the external system (e.g., Discord) for custom embed synchronization.
type Publisher interface {
	// UpdatePosting edits an existing message with the provided custom embed layout.
	UpdatePosting(channelID, messageID string, embed files.CustomEmbedConfig) error
}
