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

func TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want QOTDSelectionStrategy
	}{
		{name: "empty falls back to queue", raw: "", want: QOTDSelectionStrategyQueue},
		{name: "explicit queue stays queue", raw: "queue", want: QOTDSelectionStrategyQueue},
		{name: "random is honored", raw: "random", want: QOTDSelectionStrategyRandom},
		{name: "case-insensitive random", raw: "RANDOM", want: QOTDSelectionStrategyRandom},
		{name: "whitespace tolerated", raw: "  random  ", want: QOTDSelectionStrategyRandom},
		{name: "unknown values fall back to queue", raw: "shuffle", want: QOTDSelectionStrategyQueue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := QOTDDeckConfig{SelectionStrategy: tc.raw}.EffectiveSelectionStrategy()
			if got != tc.want {
				t.Fatalf("EffectiveSelectionStrategy(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNormalizeQOTDConfigPreservesSelectionStrategy(t *testing.T) {
	t.Parallel()

	hourUTC, minuteUTC := 12, 0
	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		Schedule: QOTDPublishScheduleConfig{HourUTC: &hourUTC, MinuteUTC: &minuteUTC},
		Decks: []QOTDDeckConfig{{
			ID:                LegacyQOTDDefaultDeckID,
			Name:              LegacyQOTDDefaultDeckName,
			Enabled:           true,
			ChannelID:         "123456789012345678",
			SelectionStrategy: "random",
		}},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	if len(normalized.Decks) != 1 || normalized.Decks[0].SelectionStrategy != string(QOTDSelectionStrategyRandom) {
		t.Fatalf("expected normalized deck to keep selection_strategy=random, got %+v", normalized.Decks)
	}
}

func TestNormalizeQOTDConfigDropsUnknownSelectionStrategy(t *testing.T) {
	t.Parallel()

	hourUTC, minuteUTC := 12, 0
	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		Schedule: QOTDPublishScheduleConfig{HourUTC: &hourUTC, MinuteUTC: &minuteUTC},
		Decks: []QOTDDeckConfig{{
			ID:                LegacyQOTDDefaultDeckID,
			Name:              LegacyQOTDDefaultDeckName,
			Enabled:           true,
			ChannelID:         "123456789012345678",
			SelectionStrategy: "shuffle",
		}},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	if len(normalized.Decks) != 1 || normalized.Decks[0].SelectionStrategy != "" {
		t.Fatalf("expected unknown selection_strategy to be dropped to empty, got %+v", normalized.Decks)
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
		SuppressScheduledPublishDatesUTC: []string{" 2026-04-03 "},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	if got := normalized.SuppressScheduledPublishDatesUTC; len(got) != 1 || got[0] != "2026-04-03" {
		t.Fatalf("expected canonical suppressed publish date, got %+v", normalized)
	}
	if !normalized.SuppressesScheduledPublishDate(time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)) {
		t.Fatalf("expected normalized config to suppress the matching slot date, got %+v", normalized)
	}
	cleared := normalized.ClearSuppressedScheduledPublishDate(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if len(cleared.SuppressScheduledPublishDatesUTC) != 0 {
		t.Fatalf("expected matching clear to remove suppressed slot date, got %+v", cleared)
	}
	unchanged := normalized.ClearSuppressedScheduledPublishDate(time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	if got := unchanged.SuppressScheduledPublishDatesUTC; len(got) != 1 || got[0] != "2026-04-03" {
		t.Fatalf("expected non-matching clear to preserve suppressed slot date, got %+v", unchanged)
	}
	shifted := normalized.WithSuppressedScheduledPublishDate(time.Date(2026, 4, 5, 13, 15, 0, 0, time.UTC))
	if got := shifted.SuppressScheduledPublishDatesUTC; len(got) != 2 || got[0] != "2026-04-03" || got[1] != "2026-04-05" {
		t.Fatalf("expected suppression helper to add a second canonical date, got %+v", shifted)
	}
}

func TestNormalizeQOTDConfigDedupesAndSortsSuppressionList(t *testing.T) {
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
		SuppressScheduledPublishDatesUTC: []string{
			"2026-04-05",
			" 2026-04-03 ",
			"2026-04-03",
			"",
			"2026-04-05",
		},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	got := normalized.SuppressScheduledPublishDatesUTC
	want := []string{"2026-04-03", "2026-04-05"}
	if len(got) != len(want) {
		t.Fatalf("expected canonical sorted dedupe, got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected dates[%d]=%q, got %q (full=%+v)", i, want[i], got[i], got)
		}
	}
}

func TestQOTDConfigUnmarshalMigratesLegacySingleSuppressionField(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"suppress_scheduled_publish_date_utc":"2026-04-04"}`)
	var cfg QOTDConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("Unmarshal(legacy field) failed: %v", err)
	}
	if got := cfg.SuppressScheduledPublishDatesUTC; len(got) != 1 || got[0] != "2026-04-04" {
		t.Fatalf("expected legacy single-string suppression to migrate into the list form, got %+v", got)
	}
}

func TestQOTDConfigUnmarshalPrefersNewListWhenBothFieldsPresent(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"suppress_scheduled_publish_date_utc":"2026-04-04","suppress_scheduled_publish_dates_utc":["2026-05-01","2026-05-02"]}`)
	var cfg QOTDConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("Unmarshal(both fields) failed: %v", err)
	}
	got := cfg.SuppressScheduledPublishDatesUTC
	want := []string{"2026-05-01", "2026-05-02"}
	if len(got) != len(want) {
		t.Fatalf("expected the new list to take precedence over the legacy single value, got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected dates[%d]=%q, got %q", i, want[i], got[i])
		}
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
		SuppressScheduledPublishDatesUTC: []string{"04/03/2026"},
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

func TestQOTDConfigLegacyJSONTableMappings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		check   func(t *testing.T, cfg QOTDConfig)
	}{
		{
			name:    "legacy question_channel_id maps to default deck channel_id",
			payload: `{"enabled": true, "question_channel_id": "111"}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if len(cfg.Decks) != 1 || cfg.Decks[0].ChannelID != "111" {
					t.Fatalf("expected channel_id mapped to 111, got %+v", cfg.Decks)
				}
			},
		},
		{
			name:    "legacy forum_channel_id maps to default deck forum_channel_id",
			payload: `{"enabled": true, "forum_channel_id": "222"}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if len(cfg.Decks) != 1 || cfg.Decks[0].ChannelID != "222" {
					t.Fatalf("expected forum_channel_id mapped to ChannelID 222, got %+v", cfg.Decks)
				}
			},
		},
		{
			name:    "legacy qotd_time_hour_utc and minute maps to Schedule",
			payload: `{"qotd_time_hour_utc": 15, "qotd_time_minute_utc": 30}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if cfg.Schedule.HourUTC == nil || *cfg.Schedule.HourUTC != 15 || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.MinuteUTC != 30 {
					t.Fatalf("expected schedule 15:30, got %+v", cfg.Schedule)
				}
			},
		},
		{
			name:    "legacy publish_hour_utc and minute maps to Schedule",
			payload: `{"publish_hour_utc": 10, "publish_minute_utc": 45}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if cfg.Schedule.HourUTC == nil || *cfg.Schedule.HourUTC != 10 || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.MinuteUTC != 45 {
					t.Fatalf("expected schedule 10:45, got %+v", cfg.Schedule)
				}
			},
		},
		{
			name:    "legacy suppress_scheduled_publish_date_utc maps to list",
			payload: `{"suppress_scheduled_publish_date_utc": "2026-06-04"}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if len(cfg.SuppressScheduledPublishDatesUTC) != 1 || cfg.SuppressScheduledPublishDatesUTC[0] != "2026-06-04" {
					t.Fatalf("expected suppressed dates list length 1, got %+v", cfg.SuppressScheduledPublishDatesUTC)
				}
			},
		},
		{
			name:    "canonical schedule shadows legacy",
			payload: `{"publish_hour_utc": 10, "publish_minute_utc": 45, "schedule": {"hour_utc": 11, "minute_utc": 50}}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if cfg.Schedule.HourUTC == nil || *cfg.Schedule.HourUTC != 11 || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.MinuteUTC != 50 {
					t.Fatalf("expected canonical schedule 11:50 to win, got %+v", cfg.Schedule)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var cfg QOTDConfig
			if err := json.Unmarshal([]byte(tc.payload), &cfg); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			tc.check(t, cfg)
		})
	}
}
