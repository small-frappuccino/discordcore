package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	// ErrInvalidQOTDInput indicates invalid QOTD configuration payloads.
	ErrInvalidQOTDInput = errors.New("invalid qotd input")
)

const (
	LegacyQOTDDefaultDeckID   = "default"
	LegacyQOTDDefaultDeckName = "Default"
	qotdPublishDateLayout     = "2006-01-02"
)

// QOTDSelectionStrategy enumerates the supported question-selection strategies
// for automatic publish. The string values are persisted in the deck config
// and mirror the QOTDQuestionSelector vocabulary used by the storage layer.
type QOTDSelectionStrategy string

const (
	QOTDSelectionStrategyQueue  QOTDSelectionStrategy = "queue"
	QOTDSelectionStrategyRandom QOTDSelectionStrategy = "random"
)

// IsZero reports whether all QOTD deck fields are unset.
func (cfg QOTDDeckConfig) IsZero() bool {
	return strings.TrimSpace(cfg.ID) == "" &&
		strings.TrimSpace(cfg.Name) == "" &&
		!cfg.Enabled &&
		strings.TrimSpace(cfg.ChannelID) == "" &&
		strings.TrimSpace(cfg.SelectionStrategy) == ""
}

// EffectiveSelectionStrategy returns the deck's configured strategy, falling
// back to "queue" when unset or unrecognized.
func (cfg QOTDDeckConfig) EffectiveSelectionStrategy() QOTDSelectionStrategy {
	switch strings.ToLower(strings.TrimSpace(cfg.SelectionStrategy)) {
	case string(QOTDSelectionStrategyRandom):
		return QOTDSelectionStrategyRandom
	default:
		return QOTDSelectionStrategyQueue
	}
}

// IsZero reports whether both schedule components are unset.
func (cfg QOTDPublishScheduleConfig) IsZero() bool {
	return cfg.HourUTC == nil && cfg.MinuteUTC == nil
}

// IsComplete reports whether both schedule components are present.
func (cfg QOTDPublishScheduleConfig) IsComplete() bool {
	return cfg.HourUTC != nil && cfg.MinuteUTC != nil
}

// Values returns the configured UTC schedule when both components are present.
func (cfg QOTDPublishScheduleConfig) Values() (hour int, minute int, ok bool) {
	if !cfg.IsComplete() {
		return 0, 0, false
	}
	return *cfg.HourUTC, *cfg.MinuteUTC, true
}

// IsZero reports whether all QOTD fields are unset.
func (cfg QOTDConfig) IsZero() bool {
	if len(cfg.deckConfigs()) > 0 {
		if len(cfg.deckConfigs()) == 1 &&
			isImplicitDefaultQOTDDeck(cfg.deckConfigs()[0], strings.TrimSpace(cfg.ActiveDeckID)) &&
			cfg.Schedule.IsZero() &&
			len(cfg.SuppressScheduledPublishDatesUTC) == 0 {
			return true
		}
		return false
	}
	if !cfg.Schedule.IsZero() || len(cfg.SuppressScheduledPublishDatesUTC) != 0 {
		return false
	}
	return true
}

// SuppressesScheduledPublishDate reports whether the given UTC publish date
// is in the suppression set. The set membership semantic replaces the old
// single-string field, so callers can suppress today AND tomorrow at the
// same time (e.g. a manual publish that occupies tomorrow's slot while a
// maintenance flow pauses today's automatic publish).
func (cfg QOTDConfig) SuppressesScheduledPublishDate(publishDate time.Time) bool {
	publishDate = publishDate.UTC()
	if publishDate.IsZero() {
		return false
	}
	target := publishDate.Format(qotdPublishDateLayout)
	for _, raw := range cfg.SuppressScheduledPublishDatesUTC {
		if strings.TrimSpace(raw) == target {
			return true
		}
	}
	return false
}

// WithSuppressedScheduledPublishDate returns a config with the publish date
// added to the suppression set. Idempotent: passing a date that is already
// suppressed returns the config unchanged.
func (cfg QOTDConfig) WithSuppressedScheduledPublishDate(publishDate time.Time) QOTDConfig {
	publishDate = publishDate.UTC()
	if publishDate.IsZero() {
		return cfg
	}
	if cfg.SuppressesScheduledPublishDate(publishDate) {
		return cfg
	}
	formatted := publishDate.Format(qotdPublishDateLayout)
	cfg.SuppressScheduledPublishDatesUTC = append(append([]string(nil), cfg.SuppressScheduledPublishDatesUTC...), formatted)
	sortSuppressedPublishDates(cfg.SuppressScheduledPublishDatesUTC)
	return cfg
}

// ClearSuppressedScheduledPublishDate returns a config with the publish
// date removed from the suppression set. Idempotent: passing a date that
// is not in the set returns the config unchanged.
func (cfg QOTDConfig) ClearSuppressedScheduledPublishDate(publishDate time.Time) QOTDConfig {
	publishDate = publishDate.UTC()
	if publishDate.IsZero() || !cfg.SuppressesScheduledPublishDate(publishDate) {
		return cfg
	}
	target := publishDate.Format(qotdPublishDateLayout)
	next := make([]string, 0, len(cfg.SuppressScheduledPublishDatesUTC))
	for _, raw := range cfg.SuppressScheduledPublishDatesUTC {
		if strings.TrimSpace(raw) == target {
			continue
		}
		next = append(next, raw)
	}
	if len(next) == 0 {
		cfg.SuppressScheduledPublishDatesUTC = nil
	} else {
		cfg.SuppressScheduledPublishDatesUTC = next
	}
	return cfg
}

// SuppressedScheduledPublishDates returns the canonical sorted set of
// suppressed UTC publish dates as time.Time values. Convenience for
// callers that want to iterate the set without re-parsing the strings.
func (cfg QOTDConfig) SuppressedScheduledPublishDates() []time.Time {
	if len(cfg.SuppressScheduledPublishDatesUTC) == 0 {
		return nil
	}
	out := make([]time.Time, 0, len(cfg.SuppressScheduledPublishDatesUTC))
	for _, raw := range cfg.SuppressScheduledPublishDatesUTC {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(qotdPublishDateLayout, raw)
		if err != nil {
			continue
		}
		out = append(out, parsed.UTC())
	}
	return out
}

func sortSuppressedPublishDates(dates []string) {
	sort.SliceStable(dates, func(i, j int) bool {
		return strings.TrimSpace(dates[i]) < strings.TrimSpace(dates[j])
	})
}

// DashboardQOTDConfig returns a stable deck-aware settings payload for the
// dashboard, even when the persisted config is still empty.
func DashboardQOTDConfig(cfg QOTDConfig) QOTDConfig {
	out := cloneQOTDConfig(cfg)
	decks := out.deckConfigs()
	if len(decks) == 0 {
		out.ActiveDeckID = LegacyQOTDDefaultDeckID
		out.Decks = []QOTDDeckConfig{{
			ID:   LegacyQOTDDefaultDeckID,
			Name: LegacyQOTDDefaultDeckName,
		}}
		return out
	}

	out.Decks = decks
	if strings.TrimSpace(out.ActiveDeckID) == "" {
		if activeDeck, ok := (QOTDConfig{
			ActiveDeckID: out.ActiveDeckID,
			Decks:        decks,
		}).ActiveDeck(); ok {
			out.ActiveDeckID = activeDeck.ID
		}
	}
	return out
}

// DeckByID resolves one configured deck by ID.
func (cfg QOTDConfig) DeckByID(deckID string) (QOTDDeckConfig, bool) {
	deckID = strings.TrimSpace(deckID)
	for _, deck := range cfg.deckConfigs() {
		if strings.TrimSpace(deck.ID) == deckID {
			return deck, true
		}
	}
	return QOTDDeckConfig{}, false
}

// ActiveDeck resolves the active configured deck, if any.
func (cfg QOTDConfig) ActiveDeck() (QOTDDeckConfig, bool) {
	decks := cfg.deckConfigs()
	if len(decks) == 0 {
		return QOTDDeckConfig{}, false
	}
	activeDeckID := strings.TrimSpace(cfg.ActiveDeckID)
	if activeDeckID != "" {
		for _, deck := range decks {
			if strings.TrimSpace(deck.ID) == activeDeckID {
				return deck, true
			}
		}
	}
	for _, deck := range decks {
		if deck.Enabled {
			return deck, true
		}
	}
	return decks[0], true
}

func (cfg QOTDConfig) deckConfigs() []QOTDDeckConfig {
	if len(cfg.Decks) > 0 {
		return cloneQOTDDeckConfigs(cfg.Decks)
	}
	return nil
}

func isImplicitDefaultQOTDDeck(deck QOTDDeckConfig, activeDeckID string) bool {
	return strings.TrimSpace(deck.ID) == LegacyQOTDDefaultDeckID &&
		strings.TrimSpace(deck.Name) == LegacyQOTDDefaultDeckName &&
		!deck.Enabled &&
		strings.TrimSpace(deck.ChannelID) == "" &&
		(activeDeckID == "" || activeDeckID == LegacyQOTDDefaultDeckID)
}

func (cfg *QOTDDeckConfig) UnmarshalJSON(data []byte) error {
	type rawQOTDDeckConfig struct {
		ID                string `json:"id,omitempty"`
		Name              string `json:"name,omitempty"`
		Enabled           bool   `json:"enabled,omitempty"`
		ChannelID         string `json:"channel_id,omitempty"`
		ForumChannelID    string `json:"forum_channel_id,omitempty"`
		QuestionChannelID string `json:"question_channel_id,omitempty"`
		ResponseChannelID string `json:"response_channel_id,omitempty"`
		SelectionStrategy string `json:"selection_strategy,omitempty"`
	}

	var raw rawQOTDDeckConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("QOTDDeckConfig.UnmarshalJSON: %w", err)
	}

	channelID := strings.TrimSpace(raw.ChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(raw.ForumChannelID)
	}
	if channelID == "" {
		channelID = strings.TrimSpace(raw.QuestionChannelID)
	}
	if channelID == "" {
		channelID = strings.TrimSpace(raw.ResponseChannelID)
	}

	*cfg = QOTDDeckConfig{
		ID:                raw.ID,
		Name:              raw.Name,
		Enabled:           raw.Enabled,
		ChannelID:         channelID,
		SelectionStrategy: strings.TrimSpace(raw.SelectionStrategy),
	}
	return nil
}

func (cfg *QOTDConfig) UnmarshalJSON(data []byte) error {
	type rawQOTDPublishScheduleConfig struct {
		HourUTC          *int `json:"hour_utc,omitempty"`
		MinuteUTC        *int `json:"minute_utc,omitempty"`
		PublishHourUTC   *int `json:"publish_hour_utc,omitempty"`
		PublishMinuteUTC *int `json:"publish_minute_utc,omitempty"`
		LegacyHourUTC    *int `json:"qotd_time_hour_utc,omitempty"`
		LegacyMinuteUTC  *int `json:"qotd_time_minute_utc,omitempty"`
	}

	type rawQOTDConfig struct {
		VerifiedRoleID string                       `json:"verified_role_id,omitempty"`
		ActiveDeckID   string                       `json:"active_deck_id,omitempty"`
		Decks          []QOTDDeckConfig             `json:"decks,omitempty"`
		Schedule       rawQOTDPublishScheduleConfig `json:"schedule,omitempty"`
		// SuppressScheduledPublishDatesUTC is the new list form. Older configs
		// persisted only LegacySuppressDateUTC; the unmarshal migrates the
		// legacy value into the list when the new field is absent.
		SuppressScheduledPublishDatesUTC []string `json:"suppress_scheduled_publish_dates_utc,omitempty"`
		LegacySuppressDateUTC            string   `json:"suppress_scheduled_publish_date_utc,omitempty"`
		Enabled                          bool     `json:"enabled,omitempty"`
		ChannelID                        string   `json:"channel_id,omitempty"`
		ForumChannelID                   string   `json:"forum_channel_id,omitempty"`
		QuestionChannelID                string   `json:"question_channel_id,omitempty"`
		ResponseChannelID                string   `json:"response_channel_id,omitempty"`
		PublishHourUTC                   *int     `json:"publish_hour_utc,omitempty"`
		PublishMinuteUTC                 *int     `json:"publish_minute_utc,omitempty"`
		LegacyHourUTC                    *int     `json:"qotd_time_hour_utc,omitempty"`
		LegacyMinuteUTC                  *int     `json:"qotd_time_minute_utc,omitempty"`
	}

	var raw rawQOTDConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("QOTDConfig.UnmarshalJSON: %w", err)
	}

	schedule := QOTDPublishScheduleConfig{
		HourUTC:   cloneOptionalInt(raw.Schedule.HourUTC),
		MinuteUTC: cloneOptionalInt(raw.Schedule.MinuteUTC),
	}
	if schedule.HourUTC == nil {
		switch {
		case raw.Schedule.PublishHourUTC != nil:
			schedule.HourUTC = cloneOptionalInt(raw.Schedule.PublishHourUTC)
		case raw.PublishHourUTC != nil:
			schedule.HourUTC = cloneOptionalInt(raw.PublishHourUTC)
		case raw.Schedule.LegacyHourUTC != nil:
			schedule.HourUTC = cloneOptionalInt(raw.Schedule.LegacyHourUTC)
		case raw.LegacyHourUTC != nil:
			schedule.HourUTC = cloneOptionalInt(raw.LegacyHourUTC)
		}
	}
	if schedule.MinuteUTC == nil {
		switch {
		case raw.Schedule.PublishMinuteUTC != nil:
			schedule.MinuteUTC = cloneOptionalInt(raw.Schedule.PublishMinuteUTC)
		case raw.PublishMinuteUTC != nil:
			schedule.MinuteUTC = cloneOptionalInt(raw.PublishMinuteUTC)
		case raw.Schedule.LegacyMinuteUTC != nil:
			schedule.MinuteUTC = cloneOptionalInt(raw.Schedule.LegacyMinuteUTC)
		case raw.LegacyMinuteUTC != nil:
			schedule.MinuteUTC = cloneOptionalInt(raw.LegacyMinuteUTC)
		}
	}

	suppressedDates := raw.SuppressScheduledPublishDatesUTC
	if len(suppressedDates) == 0 && strings.TrimSpace(raw.LegacySuppressDateUTC) != "" {
		// Legacy single-string field migration: keep old persisted configs
		// loading until the next write replaces the legacy key with the
		// canonical list form.
		suppressedDates = []string{raw.LegacySuppressDateUTC}
	}

	*cfg = QOTDConfig{
		VerifiedRoleID:                   raw.VerifiedRoleID,
		ActiveDeckID:                     raw.ActiveDeckID,
		Decks:                            raw.Decks,
		Schedule:                         schedule,
		SuppressScheduledPublishDatesUTC: suppressedDates,
	}
	if len(raw.Decks) > 0 {
		return nil
	}

	channelID := strings.TrimSpace(raw.ChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(raw.ForumChannelID)
	}
	if channelID == "" {
		channelID = strings.TrimSpace(raw.QuestionChannelID)
	}
	if channelID == "" {
		channelID = strings.TrimSpace(raw.ResponseChannelID)
	}
	if !raw.Enabled && channelID == "" {
		return nil
	}

	cfg.Decks = []QOTDDeckConfig{{
		ID:        LegacyQOTDDefaultDeckID,
		Name:      LegacyQOTDDefaultDeckName,
		Enabled:   raw.Enabled,
		ChannelID: channelID,
	}}
	return nil
}

// QOTDConfig returns the canonical QOTD config for one guild.
func (mgr *ConfigManager) QOTDConfig(guildID string) (_ QOTDConfig, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get qotd config: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return QOTDConfig{}, invalidQOTDInput("guild_id is required")
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return QOTDConfig{}, fmt.Errorf("ConfigManager.QOTDConfig: %w", err)
	}

	normalized, err := NormalizeQOTDConfig(guildConfig.QOTD)
	if err != nil {
		return QOTDConfig{}, fmt.Errorf("ConfigManager.QOTDConfig: %w", err)
	}
	return normalized, nil
}

// GetQOTDConfig returns the canonical QOTD config for one guild.
func (mgr *ConfigManager) GetQOTDConfig(guildID string) (QOTDConfig, error) {
	return mgr.QOTDConfig(guildID)
}

// SetQOTDConfig validates and persists the QOTD config for one guild.
func (mgr *ConfigManager) SetQOTDConfig(guildID string, cfg QOTDConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("set qotd config: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidQOTDInput("guild_id is required")
	}

	normalized, err := NormalizeQOTDConfig(cfg)
	if err != nil {
		return fmt.Errorf("ConfigManager.SetQOTDConfig: %w", err)
	}
	return mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		guildConfig.QOTD = normalized
		return nil
	})
}

func invalidQOTDInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidQOTDInput, fmt.Sprintf(format, args...))
}
