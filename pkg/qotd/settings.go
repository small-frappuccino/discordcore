package qotd

import (
	"context"
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func (s *Service) updateSettingsLocked(guildID string, cfg files.QOTDConfig) (files.QOTDConfig, error) {
	normalized, err := files.NormalizeQOTDConfig(cfg)
	if err != nil {
		return files.QOTDConfig{}, err
	}
	current, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return files.QOTDConfig{}, err
	}
	currentDashboard := files.DashboardQOTDConfig(current)
	nextDashboard := files.DashboardQOTDConfig(normalized)
	if err := s.configManager.SetQOTDConfig(guildID, normalized); err != nil {
		return files.QOTDConfig{}, err
	}
	if err := s.deleteRemovedDeckQuestions(context.Background(), guildID, currentDashboard, nextDashboard); err != nil {
		if rollbackErr := s.configManager.SetQOTDConfig(guildID, current); rollbackErr != nil {
			return files.QOTDConfig{}, fmt.Errorf("delete removed qotd deck questions: %w (rollback qotd config: %v)", err, rollbackErr)
		}
		return files.QOTDConfig{}, err
	}
	updated, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return files.QOTDConfig{}, err
	}
	return files.DashboardQOTDConfig(updated), nil
}