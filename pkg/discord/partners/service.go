package partners

import (
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// PartnerService manages the rendering and synchronization of partner boards.
type PartnerService struct {
	configManager *files.ConfigManager
	syncer        *partnerPostingSyncer
	renderer      *BoardRenderer
}

// NewPartnerService creates a new partner domain service.
func NewPartnerService(configManager *files.ConfigManager) *PartnerService {
	return &PartnerService{
		configManager: configManager,
		syncer:        newPartnerPostingSyncer(configManager),
		renderer:      NewBoardRenderer(),
	}
}

// Sync updates all active postings of a partner board to match the provided layout.
func (s *PartnerService) Sync(
	client *api.Client,
	guildID string,
	postings []files.CustomEmbedPostingConfig,
	embeds []discord.Embed,
) partnerSyncResult {
	return s.syncer.Sync(
		client,
		guildID,
		postings,
		embeds,
	)
}

// Render returns the Discord embed payloads for a partner board.
func (s *PartnerService) Render(template PartnerBoardTemplate, partners []PartnerRecord) ([]discord.Embed, error) {
	return s.renderer.Render(template, partners)
}

// FormatSyncSummary returns a human-readable summary of the sync operation.
func (s *PartnerService) FormatSyncSummary(result partnerSyncResult, action string) string {
	return formatPartnerSyncSummary(result, action)
}

// SyncConfig performs a full render and sync for the guild's current config.
func (s *PartnerService) SyncConfig(guildID string, client *api.Client) error {
	return s.syncer.SyncConfig(guildID, client)
}
