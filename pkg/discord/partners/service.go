package partners

import (
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// PartnerService orchestrates the rendering and synchronization of cross-server partner boards.
// It translates foreign configurations into paginated Discord embeds and synchronizes
// their state across registered channels and webhooks.
type PartnerService struct {
	configManager *files.ConfigManager
	syncer        *partnerPostingSyncer
	renderer      *BoardRenderer
}

// NewPartnerService instantiates the primary domain service for partner boards.
// It mandates the injection of the configuration manager to ensure state coherence.
func NewPartnerService(configManager *files.ConfigManager) *PartnerService {
	return &PartnerService{
		configManager: configManager,
		syncer:        newPartnerPostingSyncer(configManager),
		renderer:      NewBoardRenderer(),
	}
}

// Sync dispatches a structural update to all active partner board postings.
// It encapsulates the underlying batch reconciliation mechanism to protect
// callers from transient Discord API failures.
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

// Render compiles the partner list and its associated template into a paginated array of Discord embeds.
// It guarantees that the resulting embeds strictly adhere to Discord's character and capacity limitations.
func (s *PartnerService) Render(template PartnerBoardTemplate, partners []PartnerRecord) ([]discord.Embed, error) {
	return s.renderer.Render(template, partners)
}

// FormatSyncSummary maps the aggregated sync result structure into a human-readable diagnostic format.
func (s *PartnerService) FormatSyncSummary(result partnerSyncResult, action string) string {
	return formatPartnerSyncSummary(result, action)
}

// SyncConfig performs a full configuration read, render, and state sync loop for the specified guild.
func (s *PartnerService) SyncConfig(guildID string, client *api.Client) error {
	return s.syncer.SyncConfig(guildID, client)
}
