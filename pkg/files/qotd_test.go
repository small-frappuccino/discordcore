package files

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func testQOTDSchedule(hour, minute int) QOTDPublishScheduleConfig {
	return QOTDPublishScheduleConfig{
		HourUTC:   &hour,
		MinuteUTC: &minute,
	}
}

func TestNormalizeQOTDConfigRequiresDeliveryTargetsWhenEnabled(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeQOTDConfig(QOTDConfig{
		Decks: []QOTDDeckConfig{{
			ID:      LegacyQOTDDefaultDeckID,
			Name:    LegacyQOTDDefaultDeckName,
			Enabled: true,
		}},
	}); err == nil {
		t.Fatal("expected enabled qotd config without delivery targets to fail")
	}
}

func TestNormalizeQOTDConfigRequiresScheduleWhenEnabled(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeQOTDConfig(QOTDConfig{
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
	}); err == nil {
		t.Fatal("expected enabled qotd config without schedule to fail")
	}
}

func TestNormalizeQOTDConfigAllowsPartialScheduleWhileDisabled(t *testing.T) {
	t.Parallel()

	hourUTC := 7
	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		Schedule: QOTDPublishScheduleConfig{HourUTC: &hourUTC},
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			ChannelID: "123456789012345678",
		}},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	if normalized.Schedule.HourUTC == nil || *normalized.Schedule.HourUTC != 7 {
		t.Fatalf("expected partial schedule to persist, got %+v", normalized.Schedule)
	}
	if normalized.Schedule.MinuteUTC != nil {
		t.Fatalf("expected minute to remain unset, got %+v", normalized.Schedule)
	}
}

func TestNormalizeQOTDConfigNormalizesSuppressedScheduledPublishDate(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		ActiveDeckID: LegacyQOTDDefaultDeckID,
		Schedule:     testQOTDSchedule(12, 43),
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
		SuppressScheduledPublishDateUTC: " 2026-04-03 ",
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	if normalized.SuppressScheduledPublishDateUTC != "2026-04-03" {
		t.Fatalf("expected canonical suppressed publish date, got %+v", normalized)
	}
	if !normalized.SuppressesScheduledPublishDate(time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)) {
		t.Fatalf("expected normalized config to suppress the matching slot date, got %+v", normalized)
	}
	cleared := normalized.ClearSuppressedScheduledPublishDate(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if cleared.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected matching clear to remove suppressed slot date, got %+v", cleared)
	}
	unchanged := normalized.ClearSuppressedScheduledPublishDate(time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	if unchanged.SuppressScheduledPublishDateUTC != "2026-04-03" {
		t.Fatalf("expected non-matching clear to preserve suppressed slot date, got %+v", unchanged)
	}
	shifted := normalized.WithSuppressedScheduledPublishDate(time.Date(2026, 4, 5, 13, 15, 0, 0, time.UTC))
	if shifted.SuppressScheduledPublishDateUTC != "2026-04-05" {
		t.Fatalf("expected slot suppression helper to normalize to date only, got %+v", shifted)
	}
}

func TestNormalizeQOTDConfigRejectsInvalidSuppressedScheduledPublishDate(t *testing.T) {
	t.Parallel()

	_, err := NormalizeQOTDConfig(QOTDConfig{
		ActiveDeckID: LegacyQOTDDefaultDeckID,
		Schedule:     testQOTDSchedule(12, 43),
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
		SuppressScheduledPublishDateUTC: "04/03/2026",
	})
	if err == nil {
		t.Fatal("expected invalid suppressed scheduled publish date to fail normalization")
	}
}

func TestSetQOTDConfigCanonicalizesMessageChannelFields(t *testing.T) {
	t.Parallel()

	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, nil)

	err := mgr.SetQOTDConfig("g1", QOTDConfig{
		VerifiedRoleID: " 987654321098765432 ",
		ActiveDeckID:   LegacyQOTDDefaultDeckID,
		Schedule:       testQOTDSchedule(12, 43),
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: " 123456789012345678 ",
		}},
	})
	if err != nil {
		t.Fatalf("SetQOTDConfig() failed: %v", err)
	}

	cfg, err := mgr.QOTDConfig("g1")
	if err != nil {
		t.Fatalf("QOTDConfig() failed: %v", err)
	}
	deck, ok := cfg.ActiveDeck()
	if !ok {
		t.Fatal("expected qotd config to expose an active deck")
	}
	if !deck.Enabled {
		t.Fatal("expected qotd deck to remain enabled")
	}
	if deck.ChannelID != "123456789012345678" {
		t.Fatalf("expected canonical channel id, got %q", deck.ChannelID)
	}
	if cfg.VerifiedRoleID != "987654321098765432" {
		t.Fatalf("expected canonical verified role id, got %q", cfg.VerifiedRoleID)
	}
	if cfg.ActiveDeckID != LegacyQOTDDefaultDeckID {
		t.Fatalf("expected default deck to become active, got %q", cfg.ActiveDeckID)
	}
	if cfg.Schedule.HourUTC == nil || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.HourUTC != 12 || *cfg.Schedule.MinuteUTC != 43 {
		t.Fatalf("expected canonical schedule, got %+v", cfg.Schedule)
	}
}

func TestSetQOTDConfigRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, saveErr)

	err := mgr.SetQOTDConfig("g1", QOTDConfig{
		ActiveDeckID: LegacyQOTDDefaultDeckID,
		Schedule:     testQOTDSchedule(12, 43),
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	cfg := mgr.SnapshotConfig()
	if len(cfg.Guilds) != 1 {
		t.Fatalf("expected guild config to remain intact, got %+v", cfg.Guilds)
	}
	if !cfg.Guilds[0].QOTD.IsZero() {
		t.Fatalf("expected qotd config rollback, got %+v", cfg.Guilds[0].QOTD)
	}
}

func TestNormalizeQOTDConfigPreservesCollectorWithoutDecks(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		Collector: QOTDCollectorConfig{
			SourceChannelID: "123456789012345678",
			AuthorIDs:       []string{"111111111111111111", "111111111111111111", "222222222222222222"},
			TitlePatterns:   []string{"Question Of The Day", "question of the day", "question!!"},
			StartDate:       "2026-01-04",
		},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}

	if len(normalized.Decks) != 0 {
		t.Fatalf("expected collector-only config to avoid persisted decks, got %+v", normalized.Decks)
	}
	if normalized.Collector.SourceChannelID != "123456789012345678" {
		t.Fatalf("expected trimmed source channel id, got %+v", normalized.Collector)
	}
	if len(normalized.Collector.AuthorIDs) != 2 {
		t.Fatalf("expected duplicate author ids to be removed, got %+v", normalized.Collector.AuthorIDs)
	}
	if len(normalized.Collector.TitlePatterns) != 2 {
		t.Fatalf("expected duplicate title patterns to be removed case-insensitively, got %+v", normalized.Collector.TitlePatterns)
	}
}

func TestDashboardQOTDConfigProvidesDefaultDeckForCollectorOnlySettings(t *testing.T) {
	t.Parallel()

	display := DashboardQOTDConfig(QOTDConfig{
		Collector: QOTDCollectorConfig{
			SourceChannelID: "123456789012345678",
			TitlePatterns:   []string{"Question Of The Day"},
		},
	})

	deck, ok := display.ActiveDeck()
	if !ok {
		t.Fatal("expected dashboard config to expose a default deck for collector-only settings")
	}
	if deck.ID != LegacyQOTDDefaultDeckID || deck.Name != LegacyQOTDDefaultDeckName {
		t.Fatalf("expected implicit default deck, got %+v", deck)
	}
	if display.Collector.SourceChannelID != "123456789012345678" {
		t.Fatalf("expected collector settings to be preserved, got %+v", display.Collector)
	}
}

func TestQOTDConfigUnmarshalMigratesLegacyChannelFields(t *testing.T) {
	t.Parallel()

	var cfg QOTDConfig
	if err := json.Unmarshal([]byte(`{
		"enabled": true,
		"question_channel_id": "123456789012345678",
		"qotd_time_hour_utc": 7,
		"qotd_time_minute_utc": 5
	}`), &cfg); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	deck, ok := cfg.ActiveDeck()
	if !ok {
		t.Fatal("expected legacy payload to produce a default deck")
	}
	if !deck.Enabled {
		t.Fatalf("expected legacy enabled flag to carry over, got %+v", deck)
	}
	if deck.ChannelID != "123456789012345678" {
		t.Fatalf("expected legacy question channel to map to channel_id, got %+v", deck)
	}
	if cfg.Schedule.HourUTC == nil || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.HourUTC != 7 || *cfg.Schedule.MinuteUTC != 5 {
		t.Fatalf("expected legacy schedule fields to map to canonical schedule, got %+v", cfg.Schedule)
	}
}
