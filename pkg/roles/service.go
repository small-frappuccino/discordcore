package roles

import (
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

// RolePanelService manages the rendering and synchronization of role panels.
type RolePanelService struct {
	configManager *files.ConfigManager
	syncer        *rolePanelPostingSyncer
}

// NewRolePanelService creates a new role panel domain service.
func NewRolePanelService(configManager *files.ConfigManager) *RolePanelService {
	return &RolePanelService{
		configManager: configManager,
		syncer:        newRolePanelPostingSyncer(configManager),
	}
}

// Sync updates all active postings of a role panel to match the provided layout.
func (s *RolePanelService) Sync(
	session *discordgo.Session,
	guildID string,
	key string,
	postings []files.RolePanelPostingConfig,
	panel *files.RolePanelConfig,
) rolePanelSyncResult {
	return s.syncer.Sync(rolePanelSyncRequest{
		Session:    session,
		GuildID:    guildID,
		Key:        key,
		Postings:   postings,
		Embed:      renderRolePanelEmbed(*panel),
		Components: renderRolePanelComponents(*panel),
	})
}

// RenderEmbed returns the Discord embed payload for a role panel.
func (s *RolePanelService) RenderEmbed(panel *files.RolePanelConfig) *discordgo.MessageEmbed {
	return renderRolePanelEmbed(*panel)
}

// RenderComponents returns the Discord message components for a role panel.
func (s *RolePanelService) RenderComponents(panel *files.RolePanelConfig) []discordgo.MessageComponent {
	return renderRolePanelComponents(*panel)
}

// FormatSyncSummary returns a human-readable summary of the sync operation.
func (s *RolePanelService) FormatSyncSummary(result rolePanelSyncResult, action string) string {
	return formatRolePanelSyncSummary(result, action)
}
