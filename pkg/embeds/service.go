package embeds

import (
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// EmbedService manages the rendering and synchronization of custom embeds.
type EmbedService struct {
	configManager *files.ConfigManager
	syncer        *customEmbedPostingSyncer
}

// NewEmbedService creates a new embed domain service.
func NewEmbedService(configManager *files.ConfigManager, publisher Publisher) *EmbedService {
	return &EmbedService{
		configManager: configManager,
		syncer:        newCustomEmbedPostingSyncer(configManager, publisher),
	}
}

// Sync updates all active postings of a custom embed to match the provided layout.
func (s *EmbedService) Sync(
	guildID string,
	key string,
	postings []files.CustomEmbedPostingConfig,
	embed files.CustomEmbedConfig,
) customEmbedSyncResult {
	return s.syncer.Sync(guildID, key, postings, embed)
}

// FormatSyncSummary returns a human-readable summary of the sync operation.
func (s *EmbedService) FormatSyncSummary(result customEmbedSyncResult, action string) string {
	return formatCustomEmbedSyncSummary(result, action)
}
