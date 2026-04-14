package files

import (
	"fmt"
	"strings"
	"time"

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

	updates := out.NormalizedWebhookEmbedUpdates()
	if len(updates) > 0 {
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
		out.WebhookEmbedUpdate = WebhookEmbedUpdateConfig{}
	} else if !out.WebhookEmbedUpdate.IsZero() {
		normalized, err := normalizeWebhookEmbedUpdateConfig(out.WebhookEmbedUpdate)
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("webhook_embed_update: %w", err)
		}
		out.WebhookEmbedUpdate = normalized
	}

	out.WebhookEmbedValidation = out.WebhookEmbedValidation.Normalized()

	return out, nil
}

// NormalizePartnerBoardConfig canonicalizes the partner board config so broad
// config writes share the same validation rules as the dedicated board service.
func NormalizePartnerBoardConfig(in PartnerBoardConfig) (PartnerBoardConfig, error) {
	target, err := normalizeEmbedUpdateTargetConfig(in.Target)
	if err != nil {
		return PartnerBoardConfig{}, fmt.Errorf("target: %w", err)
	}

	partners, err := canonicalizePartnerEntries(in.Partners)
	if err != nil {
		return PartnerBoardConfig{}, fmt.Errorf("partners: %w", err)
	}

	return PartnerBoardConfig{
		Target:   target,
		Template: normalizePartnerBoardTemplate(in.Template),
		Partners: clonePartnerEntries(partners),
	}, nil
}

// NormalizeQOTDConfig canonicalizes guild QOTD settings so dedicated routes and
// broad config writes can share the same validation behavior.
func NormalizeQOTDConfig(in QOTDConfig) (QOTDConfig, error) {
	activeDeckID := strings.TrimSpace(in.ActiveDeckID)
	decks := cloneQOTDDeckConfigs(in.Decks)
	collector, err := normalizeQOTDCollectorConfig(in.Collector)
	if err != nil {
		return QOTDConfig{}, invalidQOTDInput("collector: %v", err)
	}

	if len(decks) == 0 {
		if collector.IsZero() {
			return QOTDConfig{}, nil
		}
		return QOTDConfig{
			Collector: collector,
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

	if len(normalizedDecks) == 1 &&
		isImplicitDefaultQOTDDeck(normalizedDecks[0], activeDeckID) &&
		collector.IsZero() {
		return QOTDConfig{}, nil
	}

	return QOTDConfig{
		ActiveDeckID: activeDeckID,
		Decks:        normalizedDecks,
		Collector:    collector,
	}, nil
}

func normalizeQOTDDeckConfig(in QOTDDeckConfig) (QOTDDeckConfig, error) {
	out := QOTDDeckConfig{
		ID:             strings.TrimSpace(in.ID),
		Name:           strings.TrimSpace(in.Name),
		Enabled:        in.Enabled,
		ForumChannelID: strings.TrimSpace(in.ForumChannelID),
	}

	if out.ID == "" {
		return QOTDDeckConfig{}, fmt.Errorf("id is required")
	}
	if out.Name == "" {
		return QOTDDeckConfig{}, fmt.Errorf("name is required")
	}
	if out.ForumChannelID != "" && !isAllDigits(out.ForumChannelID) {
		return QOTDDeckConfig{}, fmt.Errorf("forum_channel_id must be numeric")
	}
	if out.Enabled {
		if out.ForumChannelID == "" {
			return QOTDDeckConfig{}, fmt.Errorf("forum_channel_id is required when enabled")
		}
	}
	return out, nil
}

func firstEnabledQOTDDeckID(decks []QOTDDeckConfig) string {
	for _, deck := range decks {
		if deck.Enabled {
			return deck.ID
		}
	}
	return ""
}

func normalizeQOTDCollectorConfig(in QOTDCollectorConfig) (QOTDCollectorConfig, error) {
	out := QOTDCollectorConfig{
		SourceChannelID: strings.TrimSpace(in.SourceChannelID),
		StartDate:       strings.TrimSpace(in.StartDate),
	}

	if out.SourceChannelID != "" && !isAllDigits(out.SourceChannelID) {
		return QOTDCollectorConfig{}, fmt.Errorf("source_channel_id must be numeric")
	}
	if out.StartDate != "" {
		parsed, err := time.Parse("2006-01-02", out.StartDate)
		if err != nil {
			return QOTDCollectorConfig{}, fmt.Errorf("start_date must be YYYY-MM-DD")
		}
		out.StartDate = parsed.UTC().Format("2006-01-02")
	}

	seenAuthorIDs := make(map[string]struct{}, len(in.AuthorIDs))
	for idx, authorID := range in.AuthorIDs {
		authorID = strings.TrimSpace(authorID)
		if authorID == "" {
			continue
		}
		if !isAllDigits(authorID) {
			return QOTDCollectorConfig{}, fmt.Errorf("author_ids[%d] must be numeric", idx)
		}
		if _, exists := seenAuthorIDs[authorID]; exists {
			continue
		}
		seenAuthorIDs[authorID] = struct{}{}
		out.AuthorIDs = append(out.AuthorIDs, authorID)
	}

	seenTitlePatterns := make(map[string]struct{}, len(in.TitlePatterns))
	for _, pattern := range in.TitlePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		key := strings.ToLower(pattern)
		if _, exists := seenTitlePatterns[key]; exists {
			continue
		}
		seenTitlePatterns[key] = struct{}{}
		out.TitlePatterns = append(out.TitlePatterns, pattern)
	}

	return out, nil
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
		return DatabaseRuntimeConfig{}, false, err
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
