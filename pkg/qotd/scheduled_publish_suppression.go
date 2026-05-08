package qotd

import (
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

func isScheduledPublishSuppressed(cfg files.QOTDConfig, publishDate time.Time) bool {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return false
	}
	return cfg.SuppressesScheduledPublishDate(date)
}

func suppressScheduledPublishDate(cfg files.QOTDConfig, publishDate time.Time) files.QOTDConfig {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		cfg.SuppressScheduledPublishDateUTC = ""
		return cfg
	}
	return cfg.WithSuppressedScheduledPublishDate(date)
}

func clearSuppressedScheduledPublishDate(cfg files.QOTDConfig, publishDate time.Time) files.QOTDConfig {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		cfg.SuppressScheduledPublishDateUTC = ""
		return cfg
	}
	return cfg.ClearSuppressedScheduledPublishDate(date)
}

func (s *Service) suppressScheduledPublishForDate(guildID string, publishDate time.Time) error {
	return s.updateScheduledPublishSuppression(guildID, func(cfg files.QOTDConfig) (files.QOTDConfig, bool) {
		updated := suppressScheduledPublishDate(cfg, publishDate)
		return updated, strings.TrimSpace(updated.SuppressScheduledPublishDateUTC) != strings.TrimSpace(cfg.SuppressScheduledPublishDateUTC)
	})
}

func (s *Service) clearScheduledPublishSuppressionForDate(guildID string, publishDate time.Time) {
	err := s.updateScheduledPublishSuppression(guildID, func(cfg files.QOTDConfig) (files.QOTDConfig, bool) {
		updated := clearSuppressedScheduledPublishDate(cfg, publishDate)
		return updated, strings.TrimSpace(updated.SuppressScheduledPublishDateUTC) != strings.TrimSpace(cfg.SuppressScheduledPublishDateUTC)
	})
	if err != nil {
		log.ApplicationLogger().Warn("QOTD scheduled publish suppression update failed", "guildID", guildID, "publishDateUTC", NormalizePublishDateUTC(publishDate), "err", err)
	}
}

func parseSuppressedScheduledPublishDate(cfg files.QOTDConfig) (time.Time, bool) {
	raw := strings.TrimSpace(cfg.SuppressScheduledPublishDateUTC)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, false
	}
	return NormalizePublishDateUTC(parsed), true
}

func (s *Service) clearExpiredScheduledPublishSuppression(guildID string, cfg files.QOTDConfig, now time.Time) {
	raw := strings.TrimSpace(cfg.SuppressScheduledPublishDateUTC)
	if raw == "" {
		return
	}
	suppressedDate, ok := parseSuppressedScheduledPublishDate(cfg)
	if !ok {
		log.ApplicationLogger().Warn("QOTD suppression date is invalid; clearing stale token", "guildID", guildID, "value", raw)
		s.clearScheduledPublishSuppressionForDate(guildID, time.Time{})
		return
	}
	todayUTC := NormalizePublishDateUTC(now)
	if todayUTC.IsZero() || !suppressedDate.Before(todayUTC) {
		return
	}
	s.clearScheduledPublishSuppressionForDate(guildID, suppressedDate)
}

func (s *Service) updateScheduledPublishSuppression(guildID string, mutate func(files.QOTDConfig) (files.QOTDConfig, bool)) error {
	if s == nil || s.configManager == nil || mutate == nil {
		return nil
	}
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return err
	}
	updated, changed := mutate(cfg)
	if !changed {
		return nil
	}
	return s.configManager.SetQOTDConfig(guildID, updated)
}
