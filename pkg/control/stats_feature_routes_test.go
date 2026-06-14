package control

import (
	"net/http"
	"strings"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// TestStatsChannelsWorkspaceExposesFullConfig verifies that the stats workspace
// response surfaces all user-facing configuration fields accurately, ensuring
// the dashboard renders stable and correct state.
func TestStatsChannelsWorkspaceExposesFullConfig(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds[0].Features.StatsChannels = testBoolPtr(true)
		cfg.Guilds[0].Stats.Enabled = true
		cfg.Guilds[0].Stats.UpdateIntervalMins = 20
		cfg.Guilds[0].Stats.Channels = []files.StatsChannelConfig{
			{ChannelID: "vc-total", Label: "Server Total", NameTemplate: "{label}: {count}", MemberType: "all"},
			{ChannelID: "vc-humans", Label: "Humans", MemberType: "humans"},
			{ChannelID: "vc-bots", Label: "Bots", NameTemplate: "🤖 {count}", MemberType: "bots"},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed stats config: %v", err)
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/stats_channels", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET stats_channels status=%d body=%q", rec.Code, rec.Body.String())
	}

	response := decodeFeatureResponse[featureRecordResponse](t, rec)
	if response.Feature.Readiness != "ready" {
		t.Fatalf("expected ready readiness, got %q", response.Feature.Readiness)
	}
	if len(response.Feature.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %+v", response.Feature.Blockers)
	}
	if response.Feature.Details == nil {
		t.Fatal("expected non-nil details")
	}
	d := response.Feature.Details
	if !d.ConfigEnabled {
		t.Fatal("expected config_enabled=true")
	}
	if d.UpdateIntervalMins != 20 {
		t.Fatalf("expected update_interval_mins=20, got %d", d.UpdateIntervalMins)
	}
	if d.ConfiguredChannelCount != 3 {
		t.Fatalf("expected configured_channel_count=3, got %d", d.ConfiguredChannelCount)
	}
	if len(d.Channels) != 3 {
		t.Fatalf("expected 3 channel details, got %d", len(d.Channels))
	}

	ch0 := d.Channels[0]
	if ch0.ChannelID != "vc-total" || ch0.Label != "Server Total" || ch0.NameTemplate != "{label}: {count}" || ch0.MemberType != "all" {
		t.Fatalf("unexpected first channel detail: %+v", ch0)
	}
	ch1 := d.Channels[1]
	if ch1.ChannelID != "vc-humans" || ch1.MemberType != "humans" {
		t.Fatalf("unexpected second channel detail: %+v", ch1)
	}
	ch2 := d.Channels[2]
	if ch2.ChannelID != "vc-bots" || ch2.MemberType != "bots" || ch2.NameTemplate != "🤖 {count}" {
		t.Fatalf("unexpected third channel detail: %+v", ch2)
	}
}

// TestStatsChannelsWorkspaceEmptyConfig verifies the workspace returns clean
// zero-value state when no stats channels are configured, preventing undefined
// or null-related rendering failures in the dashboard.
func TestStatsChannelsWorkspaceEmptyConfig(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/stats_channels", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET stats_channels status=%d body=%q", rec.Code, rec.Body.String())
	}

	response := decodeFeatureResponse[featureRecordResponse](t, rec)
	if response.Feature.Details == nil {
		t.Fatal("expected non-nil details even for empty config")
	}
	d := response.Feature.Details
	if d.ConfigEnabled {
		t.Fatal("expected config_enabled=false for default config")
	}
	if d.ConfiguredChannelCount != 0 {
		t.Fatalf("expected configured_channel_count=0, got %d", d.ConfiguredChannelCount)
	}
}

// TestStatsChannelsPatchIntervalRoundTrip verifies that a dashboard user can
// change the update interval via PATCH and immediately read it back via GET,
// confirming the full UI interaction round-trip is stable.
func TestStatsChannelsPatchIntervalRoundTrip(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	handler := srv.httpServer.Handler

	getCV := func() int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == "g1" {
				return g.ConfigVersion
			}
		}
		return 0
	}

	patchRec := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/stats_channels", map[string]any{
		"config_enabled":       true,
		"update_interval_mins": 90,
		"config_version":       getCV(),
	})
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH stats_channels status=%d body=%q", patchRec.Code, patchRec.Body.String())
	}

	getRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g1/features/stats_channels", nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET stats_channels status=%d body=%q", getRec.Code, getRec.Body.String())
	}

	response := decodeFeatureResponse[featureRecordResponse](t, getRec)
	d := response.Feature.Details
	if d == nil {
		t.Fatal("expected non-nil details")
	}
	if !d.ConfigEnabled {
		t.Fatal("expected config_enabled=true after PATCH")
	}
	if d.UpdateIntervalMins != 90 {
		t.Fatalf("expected update_interval_mins=90 after PATCH, got %d", d.UpdateIntervalMins)
	}
}

// TestStatsChannelsPatchDisableRoundTrip verifies that toggling config_enabled
// to false is correctly persisted and reflected in the workspace response.
func TestStatsChannelsPatchDisableRoundTrip(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds[0].Stats.Enabled = true
		cfg.Guilds[0].Stats.UpdateIntervalMins = 45
		return nil
	})
	if err != nil {
		t.Fatalf("seed config: %v", err)
	}

	handler := srv.httpServer.Handler
	getCV := func() int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == "g1" {
				return g.ConfigVersion
			}
		}
		return 0
	}

	patchRec := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/stats_channels", map[string]any{
		"config_enabled": false,
		"config_version": getCV(),
	})
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH stats_channels status=%d body=%q", patchRec.Code, patchRec.Body.String())
	}

	getRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g1/features/stats_channels", nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET stats_channels status=%d body=%q", getRec.Code, getRec.Body.String())
	}

	response := decodeFeatureResponse[featureRecordResponse](t, getRec)
	if response.Feature.Details == nil || response.Feature.Details.ConfigEnabled {
		t.Fatal("expected config_enabled=false after disabling via PATCH")
	}
	if response.Feature.Details.UpdateIntervalMins != 45 {
		t.Fatalf("expected update_interval_mins preserved at 45 after toggling enabled, got %d", response.Feature.Details.UpdateIntervalMins)
	}
}

// TestStatsChannelsPatchRejectsStaleConfigVersion verifies that the optimistic
// concurrency control rejects PATCH requests with stale config_version values,
// preventing thundering-herd overwrites from the dashboard.
func TestStatsChannelsPatchRejectsStaleConfigVersion(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	handler := srv.httpServer.Handler

	getCV := func() int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == "g1" {
				return g.ConfigVersion
			}
		}
		return 0
	}

	firstPatch := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/stats_channels", map[string]any{
		"config_enabled": true,
		"config_version": getCV(),
	})
	if firstPatch.Code != http.StatusOK {
		t.Fatalf("first PATCH status=%d body=%q", firstPatch.Code, firstPatch.Body.String())
	}

	stalePatch := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/stats_channels", map[string]any{
		"config_enabled":       false,
		"update_interval_mins": 999,
		"config_version":       0,
	})
	if stalePatch.Code == http.StatusOK {
		t.Fatal("expected stale config_version PATCH to be rejected, but got 200 OK")
	}

	getRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g1/features/stats_channels", nil)
	response := decodeFeatureResponse[featureRecordResponse](t, getRec)
	if response.Feature.Details == nil || !response.Feature.Details.ConfigEnabled {
		t.Fatal("expected original config_enabled=true to be preserved after stale PATCH was rejected")
	}
}

// TestStatsChannelsPatchRejectsUnsupportedFields verifies that unknown fields
// in the PATCH payload are rejected, preventing silent data corruption.
func TestStatsChannelsPatchRejectsUnsupportedFields(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	handler := srv.httpServer.Handler

	rec := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/stats_channels", map[string]any{
		"nonexistent_field": "some_value",
	})
	if rec.Code == http.StatusOK {
		t.Fatal("expected unsupported field to be rejected")
	}
}

// TestStatsChannelsPatchPreservesExistingChannels verifies that updating
// config_enabled or update_interval_mins via PATCH does not destroy the
// existing channel configuration — a critical stability guarantee for
// users who modify settings without touching channels.
func TestStatsChannelsPatchPreservesExistingChannels(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds[0].Stats.Enabled = true
		cfg.Guilds[0].Stats.UpdateIntervalMins = 30
		cfg.Guilds[0].Stats.Channels = []files.StatsChannelConfig{
			{ChannelID: "vc-alpha", Label: "Alpha", MemberType: "all"},
			{ChannelID: "vc-beta", Label: "Beta", MemberType: "bots"},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed config: %v", err)
	}

	handler := srv.httpServer.Handler
	getCV := func() int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == "g1" {
				return g.ConfigVersion
			}
		}
		return 0
	}

	patchRec := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/stats_channels", map[string]any{
		"update_interval_mins": 120,
		"config_version":       getCV(),
	})
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH stats_channels status=%d body=%q", patchRec.Code, patchRec.Body.String())
	}

	cfg := cm.GuildConfig("g1")
	if len(cfg.Stats.Channels) != 2 {
		t.Fatalf("expected 2 channels preserved after interval PATCH, got %d", len(cfg.Stats.Channels))
	}
	if cfg.Stats.Channels[0].ChannelID != "vc-alpha" || cfg.Stats.Channels[1].ChannelID != "vc-beta" {
		t.Fatalf("expected channel config preserved, got %+v", cfg.Stats.Channels)
	}
	if cfg.Stats.UpdateIntervalMins != 120 {
		t.Fatalf("expected interval updated to 120, got %d", cfg.Stats.UpdateIntervalMins)
	}
}

// TestStatsChannelsFeatureToggleInheritance verifies the feature toggle
// resolution chain: global disable → guild override enable → guild clear
// inherits from global. This ensures the dashboard renders the correct
// effective state at each step.
func TestStatsChannelsFeatureToggleInheritance(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	handler := srv.httpServer.Handler

	getCV := func() int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == "g1" {
				return g.ConfigVersion
			}
		}
		return 0
	}

	globalDisable := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/features/stats_channels", map[string]any{
		"enabled": false,
	})
	if globalDisable.Code != http.StatusOK {
		t.Fatalf("global disable status=%d body=%q", globalDisable.Code, globalDisable.Body.String())
	}

	disabledResp := decodeFeatureResponse[featureRecordResponse](t, globalDisable)
	if disabledResp.Feature.EffectiveEnabled {
		t.Fatal("expected effective_enabled=false after global disable")
	}

	guildEnable := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/stats_channels", map[string]any{
		"enabled":        true,
		"config_version": getCV(),
	})
	if guildEnable.Code != http.StatusOK {
		t.Fatalf("guild enable status=%d body=%q", guildEnable.Code, guildEnable.Body.String())
	}

	enabledResp := decodeFeatureResponse[featureRecordResponse](t, guildEnable)
	if !enabledResp.Feature.EffectiveEnabled {
		t.Fatal("expected effective_enabled=true after guild override")
	}
	if enabledResp.Feature.EffectiveSource != "guild" {
		t.Fatalf("expected effective_source=guild, got %q", enabledResp.Feature.EffectiveSource)
	}

	guildClear := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/stats_channels", map[string]any{
		"enabled":        nil,
		"config_version": getCV(),
	})
	if guildClear.Code != http.StatusOK {
		t.Fatalf("guild clear status=%d body=%q", guildClear.Code, guildClear.Body.String())
	}

	clearedResp := decodeFeatureResponse[featureRecordResponse](t, guildClear)
	if clearedResp.Feature.EffectiveEnabled {
		t.Fatal("expected effective_enabled=false after guild clear inherits from global")
	}
	if clearedResp.Feature.EffectiveSource != "global" {
		t.Fatalf("expected effective_source=global after guild clear, got %q", clearedResp.Feature.EffectiveSource)
	}
}

// TestStatsChannelsWorkspaceIncludesAreaAndTags verifies the catalog metadata
// is correct so the dashboard navigation system routes the feature correctly.
func TestStatsChannelsWorkspaceIncludesAreaAndTags(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET features status=%d body=%q", rec.Code, rec.Body.String())
	}

	workspace := decodeFeatureResponse[featureWorkspaceResponse](t, rec)
	statsFeature, ok := slicesIndexBy(workspace.Workspace.Features, func(f featureRecord) bool {
		return f.ID == "stats_channels"
	})
	if !ok {
		t.Fatal("expected stats_channels in workspace")
	}
	if statsFeature.Area != featureAreaStats {
		t.Fatalf("expected area=%q, got %q", featureAreaStats, statsFeature.Area)
	}
	if len(statsFeature.Tags) < 1 {
		t.Fatalf("expected at least 1 tag, got %+v", statsFeature.Tags)
	}
	if !strings.Contains(statsFeature.Category, "stats") {
		t.Fatalf("expected category containing 'stats', got %q", statsFeature.Category)
	}
}
