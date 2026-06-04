package qotd

import (
	"context"
	"fmt"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// PrepareSettingsUpdate prepares settings update.
func PrepareSettingsUpdate(current, next files.QOTDConfig, now time.Time) (files.QOTDConfig, error) {
	normalized, err := files.NormalizeQOTDConfig(next)
	if err != nil {
		return files.QOTDConfig{}, fmt.Errorf("PrepareSettingsUpdate: %w", err)
	}
	// Clear suppression only on the ON -> OFF transition: disabling auto
	// publish means the slot the suppression was guarding no longer exists
	// under the new config, so carrying it forward would silently re-suppress
	// once the deck is re-enabled. For OFF -> OFF (operator seeding a guild
	// without auto-publish) and fresh writes we preserve whatever the caller
	// sent — clearExpiredScheduledPublishSuppression on the runtime path is
	// the cleanup hook for stale dates.
	if qotdAutomaticPublishConfigured(current) && !qotdAutomaticPublishConfigured(normalized) {
		normalized.SuppressScheduledPublishDatesUTC = nil
		return normalized, nil
	}
	if !qotdAutomaticPublishConfigured(normalized) {
		return normalized, nil
	}
	// Auto-suppression on OFF -> ON transitions only kicks in when the
	// caller has not already provided their own suppression date. Otherwise
	// we'd silently overwrite an explicit operator decision (for example a
	// legacy stale value the operator wants the runtime to clean up later)
	// with today's slot.
	if len(normalized.SuppressScheduledPublishDatesUTC) > 0 {
		return normalized, nil
	}
	if publishDate, suppress := suppressedPublishDateOnEnable(current, normalized, now); suppress {
		normalized = suppressScheduledPublishDate(normalized, publishDate)
	}
	return normalized, nil
}

func (s *Service) updateSettingsLocked(guildID string, cfg files.QOTDConfig) (files.QOTDConfig, error) {
	current, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return files.QOTDConfig{}, fmt.Errorf("Service.updateSettingsLocked: %w", err)
	}
	normalized, err := PrepareSettingsUpdate(current, cfg, s.clock())
	if err != nil {
		return files.QOTDConfig{}, fmt.Errorf("Service.updateSettingsLocked: %w", err)
	}
	currentDashboard := files.DashboardQOTDConfig(current)
	nextDashboard := files.DashboardQOTDConfig(normalized)
	if err := s.configManager.SetQOTDConfig(guildID, normalized); err != nil {
		return files.QOTDConfig{}, fmt.Errorf("Service.updateSettingsLocked: %w", err)
	}
	if err := s.deleteRemovedDeckQuestions(context.Background(), guildID, currentDashboard, nextDashboard); err != nil {
		if rollbackErr := s.configManager.SetQOTDConfig(guildID, current); rollbackErr != nil {
			return files.QOTDConfig{}, fmt.Errorf("delete removed qotd deck questions: %w (rollback qotd config: %v)", err, rollbackErr)
		}
		return files.QOTDConfig{}, fmt.Errorf("Service.updateSettingsLocked: %w", err)
	}
	updated, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return files.QOTDConfig{}, fmt.Errorf("Service.updateSettingsLocked: %w", err)
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
	return DuePublishDateUTC(schedule, now), true
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
