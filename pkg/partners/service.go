package partners

import (
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
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
	session *discordgo.Session,
	guildID string,
	postings []files.CustomEmbedPostingConfig,
	embeds []*discordgo.MessageEmbed,
) partnerSyncResult {
	return s.syncer.Sync(
		session,
		guildID,
		postings,
		embeds,
	)
}

// Render returns the Discord embed payloads for a partner board.
func (s *PartnerService) Render(template PartnerBoardTemplate, partners []PartnerRecord) ([]*discordgo.MessageEmbed, error) {
	return s.renderer.Render(template, partners)
}

// FormatSyncSummary returns a human-readable summary of the sync operation.
func (s *PartnerService) FormatSyncSummary(result partnerSyncResult, action string) string {
	return formatPartnerSyncSummary(result, action)
}

// SyncConfig performs a full render and sync for the guild's current config.
func (s *PartnerService) SyncConfig(guildID string, session *discordgo.Session) error {
	return s.syncer.SyncConfig(guildID, session)
}
