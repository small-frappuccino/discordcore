package partners

import (
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// BoardService defines partner board reads and mutations.
type BoardService interface {
	GetPartnerBoard(guildID string) (files.PartnerBoardConfig, error)
	GetPartnerBoardTarget(guildID string) (files.EmbedUpdateTargetConfig, error)
	SetPartnerBoardTarget(guildID string, target files.EmbedUpdateTargetConfig) error
	GetPartnerBoardTemplate(guildID string) (files.PartnerBoardTemplateConfig, error)
	SetPartnerBoardTemplate(guildID string, template files.PartnerBoardTemplateConfig) error

	ListPartners(guildID string) ([]files.PartnerEntryConfig, error)
	GetPartner(guildID, name string) (files.PartnerEntryConfig, error)
	CreatePartner(guildID string, partner files.PartnerEntryConfig) error
	UpdatePartner(guildID, currentName string, partner files.PartnerEntryConfig) error
	DeletePartner(guildID, name string) error
}

// BoardMutationNotifier receives mutation events for async board sync.
type BoardMutationNotifier interface {
	Notify(guildID string) error
}

// BoardApplicationService coordinates partner board persistence and auto-sync notifications.
type BoardApplicationService struct {
	configManager *files.ConfigManager
	notifier      BoardMutationNotifier
}

// NewBoardApplicationService creates a board service using ConfigManager persistence.
func NewBoardApplicationService(
	configManager *files.ConfigManager,
	notifier BoardMutationNotifier,
) *BoardApplicationService {
	return &BoardApplicationService{
		configManager: configManager,
		notifier:      notifier,
	}
}

// GetPartnerBoard reads the full board configuration for a guild.
func (s *BoardApplicationService) GetPartnerBoard(guildID string) (files.PartnerBoardConfig, error) {
	if err := s.validate(); err != nil {
		return files.PartnerBoardConfig{}, err
	}
	return s.configManager.GetPartnerBoard(guildID)
}

// GetPartnerBoardTarget reads the board update target for a guild.
func (s *BoardApplicationService) GetPartnerBoardTarget(guildID string) (files.EmbedUpdateTargetConfig, error) {
	if err := s.validate(); err != nil {
		return files.EmbedUpdateTargetConfig{}, err
	}
	return s.configManager.GetPartnerBoardTarget(guildID)
}

// SetPartnerBoardTarget persists target changes and triggers auto-sync notify.
func (s *BoardApplicationService) SetPartnerBoardTarget(guildID string, target files.EmbedUpdateTargetConfig) error {
	if err := s.validate(); err != nil {
		return err
	}
	if err := s.configManager.SetPartnerBoardTarget(guildID, target); err != nil {
		return err
	}
	s.notifyMutation(guildID, "set_target")
	return nil
}

// GetPartnerBoardTemplate reads the board render template for a guild.
func (s *BoardApplicationService) GetPartnerBoardTemplate(guildID string) (files.PartnerBoardTemplateConfig, error) {
	if err := s.validate(); err != nil {
		return files.PartnerBoardTemplateConfig{}, err
	}
	return s.configManager.GetPartnerBoardTemplate(guildID)
}

// SetPartnerBoardTemplate persists template changes and triggers auto-sync notify.
func (s *BoardApplicationService) SetPartnerBoardTemplate(guildID string, template files.PartnerBoardTemplateConfig) error {
	if err := s.validate(); err != nil {
		return err
	}
	if err := s.configManager.SetPartnerBoardTemplate(guildID, template); err != nil {
		return err
	}
	s.notifyMutation(guildID, "set_template")
	return nil
}

// ListPartners reads partner records for a guild.
func (s *BoardApplicationService) ListPartners(guildID string) ([]files.PartnerEntryConfig, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s.configManager.ListPartners(guildID)
}

// GetPartner reads one partner by name for a guild.
func (s *BoardApplicationService) GetPartner(guildID, name string) (files.PartnerEntryConfig, error) {
	if err := s.validate(); err != nil {
		return files.PartnerEntryConfig{}, err
	}
	return s.configManager.GetPartner(guildID, name)
}

// CreatePartner persists one partner and triggers auto-sync notify.
func (s *BoardApplicationService) CreatePartner(guildID string, partner files.PartnerEntryConfig) error {
	if err := s.validate(); err != nil {
		return err
	}
	if err := s.configManager.CreatePartner(guildID, partner); err != nil {
		return err
	}
	s.notifyMutation(guildID, "create_partner")
	return nil
}

// UpdatePartner persists one partner update and triggers auto-sync notify.
func (s *BoardApplicationService) UpdatePartner(guildID, currentName string, partner files.PartnerEntryConfig) error {
	if err := s.validate(); err != nil {
		return err
	}
	if err := s.configManager.UpdatePartner(guildID, currentName, partner); err != nil {
		return err
	}
	s.notifyMutation(guildID, "update_partner")
	return nil
}

// DeletePartner removes one partner and triggers auto-sync notify.
func (s *BoardApplicationService) DeletePartner(guildID, name string) error {
	if err := s.validate(); err != nil {
		return err
	}
	if err := s.configManager.DeletePartner(guildID, name); err != nil {
		return err
	}
	s.notifyMutation(guildID, "delete_partner")
	return nil
}

func (s *BoardApplicationService) validate() error {
	if s == nil {
		return fmt.Errorf("board application service is nil")
	}
	if s.configManager == nil {
		return fmt.Errorf("board application service: config manager is nil")
	}
	return nil
}

func (s *BoardApplicationService) notifyMutation(guildID, operation string) {
	if s == nil || s.notifier == nil {
		return
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return
	}

	if err := s.notifier.Notify(guildID); err != nil {
		log.ApplicationLogger().Warn(
			"Partner board auto-sync notify failed",
			"guild_id", guildID,
			"operation", operation,
			"err", err,
		)
	}
}
