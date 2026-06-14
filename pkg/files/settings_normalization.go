package files

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
)

// NormalizeRuntimeConfig canonicalizes runtime config sections used by the
// control dashboard before they are persisted as part of broader config writes.
func NormalizeRuntimeConfig(in RuntimeConfig) (RuntimeConfig, error) {
	out := cloneRuntimeConfig(in)

	if normalizedDB, ok, err := normalizeRuntimeDatabaseConfig(out.Database); err != nil {
		return RuntimeConfig{}, fmt.Errorf("database: %w", err)
	} else if ok {
		out.Database = normalizedDB
	}
	if out.GlobalMaxWorkers < 0 {
		return RuntimeConfig{}, fmt.Errorf("global_max_workers must be >= 0")
	}

	if updates := out.NormalizedWebhookEmbedUpdates(); len(updates) > 0 {
		normalized := make([]WebhookEmbedUpdateConfig, 0, len(updates))
		seen := make(map[string]struct{}, len(updates))
		for idx, item := range updates {
			next, err := normalizeWebhookEmbedUpdateConfig(item)
			if err != nil {
				return RuntimeConfig{}, fmt.Errorf("webhook_embed_updates[%d]: %w", idx, err)
			}
			if _, exists := seen[next.MessageID]; exists {
				return RuntimeConfig{}, fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateAlreadyExists, next.MessageID)
			}
			seen[next.MessageID] = struct{}{}
			normalized = append(normalized, next)
		}
		out.WebhookEmbedUpdates = normalized
	} else {
		out.WebhookEmbedUpdates = nil
	}

	out.WebhookEmbedValidation = out.WebhookEmbedValidation.Normalized()

	return out, nil
}

// NormalizePartnerBoardConfig canonicalizes the partner board config so broad
// config writes share the same validation rules as the dedicated board service.
func NormalizePartnerBoardConfig(in PartnerBoardConfig) (PartnerBoardConfig, error) {
	partners, err := canonicalizePartnerEntries(in.Partners)
	if err != nil {
		return PartnerBoardConfig{}, fmt.Errorf("partners: %w", err)
	}

	return PartnerBoardConfig{
		Postings: cloneCustomEmbedPostings(in.Postings),
		Template: normalizePartnerBoardTemplate(in.Template),
		Partners: clonePartnerEntries(partners),
	}, nil
}

// NormalizeQOTDConfig canonicalizes guild QOTD settings so dedicated routes and
// broad config writes can share the same validation behavior.
func NormalizeQOTDConfig(in QOTDConfig) (QOTDConfig, error) {
	verifiedRoleID := strings.TrimSpace(in.VerifiedRoleID)
	activeDeckID := strings.TrimSpace(in.ActiveDeckID)
	decks := cloneQOTDDeckConfigs(in.Decks)
	suppressedPublishDatesUTC, err := normalizeQOTDSuppressedPublishDates(in.SuppressScheduledPublishDatesUTC)
	if err != nil {
		return QOTDConfig{}, invalidQOTDInput("suppress_scheduled_publish_dates_utc: %v", err)
	}
	schedule, err := normalizeQOTDPublishScheduleConfig(in.Schedule)
	if err != nil {
		return QOTDConfig{}, invalidQOTDInput("schedule: %v", err)
	}
	if verifiedRoleID != "" && !isAllDigits(verifiedRoleID) {
		return QOTDConfig{}, invalidQOTDInput("verified_role_id must be numeric")
	}

	if len(decks) == 0 {
		// suppressedPublishDateUTC must keep the config non-zero on the
		// no-deck path: a suppression-only config still carries meaningful
		// state (the scheduler reads it back to skip the suppressed slot).
		// QOTDConfig.IsZero handles the symmetric case on the read side.
		if verifiedRoleID == "" && schedule.IsZero() && len(suppressedPublishDatesUTC) == 0 {
			return QOTDConfig{}, nil
		}
		return QOTDConfig{
			VerifiedRoleID:                   verifiedRoleID,
			Schedule:                         schedule,
			SuppressScheduledPublishDatesUTC: suppressedPublishDatesUTC,
		}, nil
	}

	normalizedDecks := make([]QOTDDeckConfig, 0, len(decks))
	seenIDs := make(map[string]struct{}, len(decks))
	seenNames := make(map[string]struct{}, len(decks))
	for idx, deck := range decks {
		normalized, err := normalizeQOTDDeckConfig(deck)
		if err != nil {
			return QOTDConfig{}, invalidQOTDInput("decks[%d]: %v", idx, err)
		}
		if _, exists := seenIDs[normalized.ID]; exists {
			return QOTDConfig{}, invalidQOTDInput("deck ids must be unique")
		}
		seenIDs[normalized.ID] = struct{}{}
		nameKey := strings.ToLower(normalized.Name)
		if _, exists := seenNames[nameKey]; exists {
			return QOTDConfig{}, invalidQOTDInput("deck names must be unique")
		}
		seenNames[nameKey] = struct{}{}
		normalizedDecks = append(normalizedDecks, normalized)
	}

	if activeDeckID == "" {
		activeDeckID = firstEnabledQOTDDeckID(normalizedDecks)
	}
	if activeDeckID == "" && len(normalizedDecks) > 0 {
		activeDeckID = normalizedDecks[0].ID
	}
	if activeDeckID != "" {
		if _, ok := seenIDs[activeDeckID]; !ok {
			return QOTDConfig{}, invalidQOTDInput("active_deck_id must refer to a configured deck")
		}
	}

	if firstEnabledQOTDDeckID(normalizedDecks) != "" && !schedule.IsComplete() {
		return QOTDConfig{}, invalidQOTDInput("schedule.hour_utc and schedule.minute_utc are required when enabled")
	}

	if len(normalizedDecks) == 1 &&
		isImplicitDefaultQOTDDeck(normalizedDecks[0], activeDeckID) &&
		verifiedRoleID == "" &&
		schedule.IsZero() &&
		len(suppressedPublishDatesUTC) == 0 {
		return QOTDConfig{}, nil
	}

	return QOTDConfig{
		VerifiedRoleID:                   verifiedRoleID,
		ActiveDeckID:                     activeDeckID,
		Decks:                            normalizedDecks,
		Schedule:                         schedule,
		SuppressScheduledPublishDatesUTC: suppressedPublishDatesUTC,
	}, nil
}

// normalizeQOTDSuppressedPublishDates validates each entry, dedupes (case
// insensitive whitespace), and returns the canonical sorted list. Empty
// entries are silently dropped; a malformed entry fails the whole config so
// callers learn about the typo at write time instead of at runtime when the
// scheduler tries to compare against a corrupt date.
func normalizeQOTDSuppressedPublishDates(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(qotdPublishDateLayout, raw)
		if err != nil {
			return nil, fmt.Errorf("must be UTC publish dates in YYYY-MM-DD format")
		}
		canonical := parsed.UTC().Format(qotdPublishDateLayout)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	if len(out) == 0 {
		return nil, nil
	}
	sort.Strings(out)
	return out, nil
}

func normalizeQOTDPublishScheduleConfig(in QOTDPublishScheduleConfig) (QOTDPublishScheduleConfig, error) {
	out := QOTDPublishScheduleConfig{
		HourUTC:   cloneOptionalInt(in.HourUTC),
		MinuteUTC: cloneOptionalInt(in.MinuteUTC),
	}
	if out.HourUTC != nil {
		if *out.HourUTC < 0 || *out.HourUTC > 23 {
			return QOTDPublishScheduleConfig{}, fmt.Errorf("hour_utc must be between 0 and 23")
		}
	}
	if out.MinuteUTC != nil {
		if *out.MinuteUTC < 0 || *out.MinuteUTC > 59 {
			return QOTDPublishScheduleConfig{}, fmt.Errorf("minute_utc must be between 0 and 59")
		}
	}
	return out, nil
}

func normalizeQOTDDeckConfig(in QOTDDeckConfig) (QOTDDeckConfig, error) {
	out := QOTDDeckConfig{
		ID:                strings.TrimSpace(in.ID),
		Name:              strings.TrimSpace(in.Name),
		Enabled:           in.Enabled,
		ChannelID:         strings.TrimSpace(in.ChannelID),
		SelectionStrategy: normalizeQOTDSelectionStrategy(in.SelectionStrategy),
	}

	if out.ID == "" {
		out.ID = idgen.GenerateString()
	}
	if out.Name == "" {
		return QOTDDeckConfig{}, fmt.Errorf("name is required")
	}
	if out.ChannelID != "" && !isAllDigits(out.ChannelID) {
		return QOTDDeckConfig{}, fmt.Errorf("channel_id must be numeric")
	}
	if out.Enabled {
		if out.ChannelID == "" {
			return QOTDDeckConfig{}, fmt.Errorf("channel_id is required when enabled")
		}
	}
	return out, nil
}

// normalizeQOTDSelectionStrategy coerces persisted values into the supported
// vocabulary. Empty / unknown values fall back to "" so the consumer can
// apply its own default; only "random" is propagated as a non-default
// selection.
func normalizeQOTDSelectionStrategy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(QOTDSelectionStrategyRandom):
		return string(QOTDSelectionStrategyRandom)
	case string(QOTDSelectionStrategyQueue):
		return string(QOTDSelectionStrategyQueue)
	default:
		return ""
	}
}

func firstEnabledQOTDDeckID(decks []QOTDDeckConfig) string {
	for _, deck := range decks {
		if deck.Enabled {
			return deck.ID
		}
	}
	return ""
}

func normalizeRuntimeDatabaseConfig(in DatabaseRuntimeConfig) (DatabaseRuntimeConfig, bool, error) {
	cfg := persistence.Config{
		Driver:              in.Driver,
		DatabaseURL:         in.DatabaseURL,
		MaxOpenConns:        in.MaxOpenConns,
		MaxIdleConns:        in.MaxIdleConns,
		ConnMaxLifetimeSecs: in.ConnMaxLifetimeSecs,
		ConnMaxIdleTimeSecs: in.ConnMaxIdleTimeSecs,
		PingTimeoutMS:       in.PingTimeoutMS,
	}

	if cfg == (persistence.Config{}) {
		return DatabaseRuntimeConfig{}, false, nil
	}

	normalized := cfg.Normalized()
	if err := normalized.Validate(); err != nil {
		return DatabaseRuntimeConfig{}, false, fmt.Errorf("normalizeRuntimeDatabaseConfig: %w", err)
	}

	return DatabaseRuntimeConfig{
		Driver:              normalized.Driver,
		DatabaseURL:         normalized.DatabaseURL,
		MaxOpenConns:        normalized.MaxOpenConns,
		MaxIdleConns:        normalized.MaxIdleConns,
		ConnMaxLifetimeSecs: normalized.ConnMaxLifetimeSecs,
		ConnMaxIdleTimeSecs: normalized.ConnMaxIdleTimeSecs,
		PingTimeoutMS:       normalized.PingTimeoutMS,
	}, true, nil
}
