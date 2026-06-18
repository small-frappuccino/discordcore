package embeds

import (
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

// EmbedService manages the rendering and synchronization of custom embeds.
type EmbedService struct {
	configManager *files.ConfigManager
	syncer        *customEmbedPostingSyncer
}

// NewEmbedService creates a new embed domain service.
func NewEmbedService(configManager *files.ConfigManager) *EmbedService {
	return &EmbedService{
		configManager: configManager,
		syncer:        newCustomEmbedPostingSyncer(configManager),
	}
}

// Sync updates all active postings of a custom embed to match the provided layout.
func (s *EmbedService) Sync(
	session *discordgo.Session,
	guildID string,
	key string,
	postings []files.CustomEmbedPostingConfig,
	embed *discordgo.MessageEmbed,
) customEmbedSyncResult {
	return s.syncer.Sync(session, guildID, key, postings, embed)
}

// Render returns the Discord embed payload for a given custom embed configuration.
func (s *EmbedService) Render(ce files.CustomEmbedConfig) *discordgo.MessageEmbed {
	return renderCustomEmbed(ce)
}

// RenderArikawa returns the Discord embed payload for a given custom embed configuration natively for Arikawa.
func (s *EmbedService) RenderArikawa(ce files.CustomEmbedConfig) discord.Embed {
	return RenderArikawa(ce)
}

// FormatSyncSummary returns a human-readable summary of the sync operation.
func (s *EmbedService) FormatSyncSummary(result customEmbedSyncResult, action string) string {
	return formatCustomEmbedSyncSummary(result, action)
}
