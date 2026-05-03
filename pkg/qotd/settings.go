package qotd

import (
	"context"
	"fmt"
	"time"

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
	if publishDate, suppress := suppressedPublishDateOnEnable(current, normalized, s.clock()); suppress {
		normalized = suppressScheduledPublishDate(normalized, publishDate)
	}
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

func suppressedPublishDateOnEnable(current, next files.QOTDConfig, now time.Time) (time.Time, bool) {
	if qotdAutomaticPublishConfigured(current) || !qotdAutomaticPublishConfigured(next) {
		return time.Time{}, false
	}
	schedule, err := resolvePublishSchedule(next)
	if err != nil {
		return time.Time{}, false
	}
	return CurrentPublishDateUTC(schedule, now), true
}

func qotdAutomaticPublishConfigured(cfg files.QOTDConfig) bool {
	if !cfg.Schedule.IsComplete() {
		return false
	}
	activeDeck, ok := cfg.ActiveDeck()
	if !ok {
		return false
	}
	return activeDeck.Enabled && canPublishQOTD(activeDeck)
}