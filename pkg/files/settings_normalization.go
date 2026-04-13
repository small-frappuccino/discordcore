package files

import (
	"fmt"
	"strings"

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
	if len(decks) == 0 {
		legacy := QOTDDeckConfig{
			ID:                LegacyQOTDDefaultDeckID,
			Name:              LegacyQOTDDefaultDeckName,
			Enabled:           in.Enabled,
			QuestionChannelID: strings.TrimSpace(in.QuestionChannelID),
			ResponseChannelID: strings.TrimSpace(in.ResponseChannelID),
		}
		if !legacy.IsZero() {
			decks = []QOTDDeckConfig{legacy}
		}
	}

	if len(decks) == 0 {
		return QOTDConfig{}, nil
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

	if len(normalizedDecks) == 1 && isImplicitDefaultQOTDDeck(normalizedDecks[0], activeDeckID) {
		return QOTDConfig{}, nil
	}

	return QOTDConfig{
		ActiveDeckID: activeDeckID,
		Decks:        normalizedDecks,
	}, nil
}

func normalizeQOTDDeckConfig(in QOTDDeckConfig) (QOTDDeckConfig, error) {
	out := QOTDDeckConfig{
		ID:                strings.TrimSpace(in.ID),
		Name:              strings.TrimSpace(in.Name),
		Enabled:           in.Enabled,
		QuestionChannelID: strings.TrimSpace(in.QuestionChannelID),
		ResponseChannelID: strings.TrimSpace(in.ResponseChannelID),
	}

	if out.ID == "" {
		return QOTDDeckConfig{}, fmt.Errorf("id is required")
	}
	if out.Name == "" {
		return QOTDDeckConfig{}, fmt.Errorf("name is required")
	}
	if out.QuestionChannelID != "" && !isAllDigits(out.QuestionChannelID) {
		return QOTDDeckConfig{}, fmt.Errorf("question_channel_id must be numeric")
	}
	if out.ResponseChannelID != "" && !isAllDigits(out.ResponseChannelID) {
		return QOTDDeckConfig{}, fmt.Errorf("response_channel_id must be numeric")
	}
	if out.QuestionChannelID == "" && out.ResponseChannelID != "" {
		return QOTDDeckConfig{}, fmt.Errorf("question_channel_id is required when response_channel_id is set")
	}
	if out.ResponseChannelID == "" && out.QuestionChannelID != "" {
		return QOTDDeckConfig{}, fmt.Errorf("response_channel_id is required when question_channel_id is set")
	}
	if out.Enabled {
		if out.QuestionChannelID == "" {
			return QOTDDeckConfig{}, fmt.Errorf("question_channel_id is required when enabled")
		}
		if out.ResponseChannelID == "" {
			return QOTDDeckConfig{}, fmt.Errorf("response_channel_id is required when enabled")
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
