package control

import (
	"context"
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
	_, err := cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {

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
	if d.ConfiguredChannelCount != 0 {
		t.Fatalf("expected configured_channel_count=0, got %d", d.ConfiguredChannelCount)
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
