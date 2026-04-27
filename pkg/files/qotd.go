package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidQOTDInput indicates invalid QOTD configuration payloads.
	ErrInvalidQOTDInput = errors.New("invalid qotd input")
)

const (
	LegacyQOTDDefaultDeckID   = "default"
	LegacyQOTDDefaultDeckName = "Default"
)

// IsZero reports whether all QOTD deck fields are unset.
func (cfg QOTDDeckConfig) IsZero() bool {
	return strings.TrimSpace(cfg.ID) == "" &&
		strings.TrimSpace(cfg.Name) == "" &&
		!cfg.Enabled &&
		strings.TrimSpace(cfg.ChannelID) == ""
}

// IsZero reports whether all QOTD collector fields are unset.
func (cfg QOTDCollectorConfig) IsZero() bool {
	return strings.TrimSpace(cfg.SourceChannelID) == "" &&
		strings.TrimSpace(cfg.StartDate) == "" &&
		len(cfg.AuthorIDs) == 0 &&
		len(cfg.TitlePatterns) == 0
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
			cfg.Collector.IsZero() &&
			cfg.Schedule.IsZero() {
			return true
		}
		return false
	}
	if !cfg.Collector.IsZero() || !cfg.Schedule.IsZero() {
		return false
	}
	return true
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
	}

	var raw rawQOTDDeckConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
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
		ID:        raw.ID,
		Name:      raw.Name,
		Enabled:   raw.Enabled,
		ChannelID: channelID,
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
		VerifiedRoleID    string                       `json:"verified_role_id,omitempty"`
		ActiveDeckID      string                       `json:"active_deck_id,omitempty"`
		Decks             []QOTDDeckConfig             `json:"decks,omitempty"`
		Collector         QOTDCollectorConfig          `json:"collector,omitempty"`
		Schedule          rawQOTDPublishScheduleConfig `json:"schedule,omitempty"`
		Enabled           bool                         `json:"enabled,omitempty"`
		ChannelID         string                       `json:"channel_id,omitempty"`
		ForumChannelID    string                       `json:"forum_channel_id,omitempty"`
		QuestionChannelID string                       `json:"question_channel_id,omitempty"`
		ResponseChannelID string                       `json:"response_channel_id,omitempty"`
		PublishHourUTC    *int                         `json:"publish_hour_utc,omitempty"`
		PublishMinuteUTC  *int                         `json:"publish_minute_utc,omitempty"`
		LegacyHourUTC     *int                         `json:"qotd_time_hour_utc,omitempty"`
		LegacyMinuteUTC   *int                         `json:"qotd_time_minute_utc,omitempty"`
	}

	var raw rawQOTDConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
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

	*cfg = QOTDConfig{
		VerifiedRoleID: raw.VerifiedRoleID,
		ActiveDeckID:   raw.ActiveDeckID,
		Decks:          raw.Decks,
		Collector:      raw.Collector,
		Schedule:       schedule,
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
func (mgr *ConfigManager) QOTDConfig(guildID string) (QOTDConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return QOTDConfig{}, fmt.Errorf("get qotd config: %w", invalidQOTDInput("guild_id is required"))
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return QOTDConfig{}, err
	}

	normalized, err := NormalizeQOTDConfig(guildConfig.QOTD)
	if err != nil {
		return QOTDConfig{}, fmt.Errorf("get qotd config: %w", err)
	}
	return normalized, nil
}

// GetQOTDConfig returns the canonical QOTD config for one guild.
func (mgr *ConfigManager) GetQOTDConfig(guildID string) (QOTDConfig, error) {
	return mgr.QOTDConfig(guildID)
}

// SetQOTDConfig validates and persists the QOTD config for one guild.
func (mgr *ConfigManager) SetQOTDConfig(guildID string, cfg QOTDConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return fmt.Errorf("set qotd config: %w", invalidQOTDInput("guild_id is required"))
	}

	normalized, err := NormalizeQOTDConfig(cfg)
	if err != nil {
		return fmt.Errorf("set qotd config: %w", err)
	}
	if err := mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		guildConfig.QOTD = normalized
		return nil
	}); err != nil {
		return fmt.Errorf("set qotd config: %w", err)
	}
	return nil
}

func invalidQOTDInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidQOTDInput, fmt.Sprintf(format, args...))
}
