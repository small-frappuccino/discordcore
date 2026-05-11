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
		return cfg
	}
	return cfg.WithSuppressedScheduledPublishDate(date)
}

func clearSuppressedScheduledPublishDate(cfg files.QOTDConfig, publishDate time.Time) files.QOTDConfig {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return cfg
	}
	return cfg.ClearSuppressedScheduledPublishDate(date)
}

func (s *Service) suppressScheduledPublishForDate(guildID string, publishDate time.Time) error {
	return s.updateScheduledPublishSuppression(guildID, func(cfg files.QOTDConfig) (files.QOTDConfig, bool) {
		updated := suppressScheduledPublishDate(cfg, publishDate)
		return updated, !sameSuppressionSet(cfg.SuppressScheduledPublishDatesUTC, updated.SuppressScheduledPublishDatesUTC)
	})
}

func (s *Service) clearScheduledPublishSuppressionForDate(guildID string, publishDate time.Time) {
	err := s.updateScheduledPublishSuppression(guildID, func(cfg files.QOTDConfig) (files.QOTDConfig, bool) {
		updated := clearSuppressedScheduledPublishDate(cfg, publishDate)
		return updated, !sameSuppressionSet(cfg.SuppressScheduledPublishDatesUTC, updated.SuppressScheduledPublishDatesUTC)
	})
	if err != nil {
		log.ApplicationLogger().Warn("QOTD scheduled publish suppression update failed", "guildID", guildID, "publishDateUTC", NormalizePublishDateUTC(publishDate), "err", err)
	}
}

// parseSuppressedScheduledPublishDates returns every suppression entry as a
// normalized UTC date. Malformed entries are dropped silently and reported
// via the returned hasInvalid flag so the cleanup pass can purge them.
func parseSuppressedScheduledPublishDates(cfg files.QOTDConfig) (dates []time.Time, invalid []string) {
	if len(cfg.SuppressScheduledPublishDatesUTC) == 0 {
		return nil, nil
	}
	for _, raw := range cfg.SuppressScheduledPublishDatesUTC {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			invalid = append(invalid, raw)
			continue
		}
		parsed, err := time.Parse("2006-01-02", trimmed)
		if err != nil {
			invalid = append(invalid, raw)
			continue
		}
		dates = append(dates, NormalizePublishDateUTC(parsed))
	}
	return dates, invalid
}

// clearExpiredScheduledPublishSuppression removes every suppression entry
// whose date is before today (UTC). Each removal is a separate write so the
// audit trail stays granular and a partial config-manager failure on one
// date does not leak into the others. Malformed entries are purged on
// sight so the slice cannot grow unbounded from typos. Each successful
// removal records a SuppressionCleared metric so operators can see the
// cleanup volume in /v1/health/qotd.
func (s *Service) clearExpiredScheduledPublishSuppression(guildID string, cfg files.QOTDConfig, now time.Time) {
	dates, invalid := parseSuppressedScheduledPublishDates(cfg)
	for _, raw := range invalid {
		log.ApplicationLogger().Warn("QOTD suppression date is invalid; clearing stale token", "guildID", guildID, "value", raw)
		s.clearInvalidSuppressionEntry(guildID, raw)
		s.observability().RecordSuppressionCleared()
	}
	todayUTC := NormalizePublishDateUTC(now)
	if todayUTC.IsZero() {
		return
	}
	for _, date := range dates {
		if !date.Before(todayUTC) {
			continue
		}
		s.clearScheduledPublishSuppressionForDate(guildID, date)
		s.observability().RecordSuppressionCleared()
	}
}

// clearInvalidSuppressionEntry removes one literal stale token from the
// suppression slice without trying to parse it as a date.
func (s *Service) clearInvalidSuppressionEntry(guildID, value string) {
	err := s.updateScheduledPublishSuppression(guildID, func(cfg files.QOTDConfig) (files.QOTDConfig, bool) {
		next := make([]string, 0, len(cfg.SuppressScheduledPublishDatesUTC))
		removed := false
		for _, entry := range cfg.SuppressScheduledPublishDatesUTC {
			if !removed && entry == value {
				removed = true
				continue
			}
			next = append(next, entry)
		}
		if !removed {
			return cfg, false
		}
		if len(next) == 0 {
			cfg.SuppressScheduledPublishDatesUTC = nil
		} else {
			cfg.SuppressScheduledPublishDatesUTC = next
		}
		return cfg, true
	})
	if err != nil {
		log.ApplicationLogger().Warn("QOTD scheduled publish suppression cleanup failed", "guildID", guildID, "value", value, "err", err)
	}
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

// sameSuppressionSet compares two suppression slices for set equality. The
// slices are stored sorted (normalizeQOTDSuppressedPublishDates and
// WithSuppressedScheduledPublishDate both sort) so a straight pairwise
// compare suffices.
func sameSuppressionSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}
