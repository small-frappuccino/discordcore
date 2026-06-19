package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func setTestBotGuildBindings(srv *Server, bindings ...BotGuildBinding) {
	if srv == nil {
		return
	}
	srv.SetBotGuildBindingsProvider(func(context.Context) ([]BotGuildBinding, error) {
		return append([]BotGuildBinding(nil), bindings...), nil
	})
}

func setTestKnownBotInstanceIDs(srv *Server, ids ...string) {
	if srv == nil {
		return
	}
	srv.SetKnownBotInstanceIDs(ids)
}

func TestSettingsOverviewReturnsCatalogGlobalWorkspaceAndGuildSummaries(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"})
	_, err := cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Features = files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: testBoolPtr(true),
			},
		}
		cfg.RuntimeConfig = files.RuntimeConfig{
			BotTheme: "bnnuy",
		}
		cfg.Guilds[0].Channels = files.ChannelsConfig{
			Commands:    "100",
			MemberJoin:  "200",
			MemberLeave: "300",
		}
		cfg.Guilds[0].PartnerBoard = files.PartnerBoardConfig{
			Partners: []files.PartnerEntryConfig{{
				Name: "Alpha",
				Link: "https://discord.gg/alpha",
			}},
		}
		return nil
	})

	if err != nil {
		t.Fatalf("seed config: %v", err)
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/settings status=%d body=%q", rec.Code, rec.Body.String())
	}

	var payload settingsOverviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode settings overview: %v", err)
	}

	if payload.Workspace.ConfigPath == "" {
		t.Fatalf("expected config path in overview: %+v", payload.Workspace)
	}
	if payload.Workspace.Global.Sections.Runtime.Appearance.BotTheme != "bnnuy" {
		t.Fatalf("unexpected global runtime appearance: %+v", payload.Workspace.Global.Sections.Runtime.Appearance)
	}
	if len(payload.Workspace.Catalog.Global) == 0 || len(payload.Workspace.Catalog.Guild) == 0 {
		t.Fatalf("expected populated settings catalog: %+v", payload.Workspace.Catalog)
	}
	for _, section := range payload.Workspace.Catalog.Guild {
		if section.ID == "moderation" {
			t.Fatalf("did not expect legacy moderation section in guild catalog: %+v", payload.Workspace.Catalog.Guild)
		}
	}
	if len(payload.Workspace.Guilds) != 1 {
		t.Fatalf("expected one configured guild summary, got %+v", payload.Workspace.Guilds)
	}
	if payload.Workspace.Registry.ConfiguredCount != 1 || payload.Workspace.Registry.AvailableCount != 0 {
		t.Fatalf("unexpected registry counts: %+v", payload.Workspace.Registry)
	}
	if payload.Workspace.Guilds[0].ConfiguredChannels != 3 {
		t.Fatalf("unexpected configured channel count: %+v", payload.Workspace.Guilds[0])
	}
	if payload.Workspace.Guilds[0].Partners != 1 {
		t.Fatalf("unexpected partner count: %+v", payload.Workspace.Guilds[0])
	}
}

func TestSettingsOverviewRejectsWhenGuildDiscoveryUnavailable(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/settings", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when guild discovery is unavailable, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestGlobalSettingsPutPersistsGroupedRuntimeAndFeatures(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	cv := cm.SnapshotConfig().ConfigVersion

	payload := updateGlobalSettingsRequest{
		ConfigVersion: &cv,
		Features: &files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: testBoolPtr(false),
			},
		},
		Runtime: &runtimeSettingsSections{
			Appearance: runtimeAppearanceSection{
				BotTheme: "soft-oat",
			},
			Logging: runtimeLoggingSection{
				ModerationLogging: testBoolPtr(false),
			},
			Webhook: runtimeWebhookSection{
				Validation: files.WebhookEmbedValidationConfig{
					Mode:      files.WebhookEmbedValidationModeStrict,
					TimeoutMS: 4500,
				},
			},
		},
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPut, "/v1/settings/global", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /v1/settings/global status=%d body=%q", rec.Code, rec.Body.String())
	}

	var response globalSettingsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode global settings response: %v", err)
	}

	if response.Workspace.Sections.Runtime.Appearance.BotTheme != "soft-oat" {
		t.Fatalf("unexpected runtime appearance: %+v", response.Workspace.Sections.Runtime.Appearance)
	}
	if response.Workspace.Sections.Runtime.Webhook.Validation.Mode != files.WebhookEmbedValidationModeStrict {
		t.Fatalf("unexpected webhook validation: %+v", response.Workspace.Sections.Runtime.Webhook.Validation)
	}
	if response.Workspace.Effective.Features.Services.Monitoring {
		t.Fatalf("expected effective monitoring=false, got %+v", response.Workspace.Effective.Features.Services)
	}

	cfg := cm.SnapshotConfig()
	if cfg.RuntimeConfig.BotTheme != "soft-oat" {
		t.Fatalf("expected persisted bot_theme, got %+v", cfg.RuntimeConfig)
	}
	if cfg.Features.Services.Monitoring == nil || *cfg.Features.Services.Monitoring {
		t.Fatalf("expected persisted monitoring=false, got %+v", cfg.Features.Services)
	}
}

func TestGlobalSettingsPutRejectsDuplicateWebhookEmbedUpdates(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	cv := cm.SnapshotConfig().ConfigVersion

	payload := updateGlobalSettingsRequest{
		ConfigVersion: &cv,
		Runtime: &runtimeSettingsSections{
			Webhook: runtimeWebhookSection{
				Updates: []files.WebhookEmbedUpdateConfig{
					{
						MessageID:  "123456789012345678",
						WebhookURL: "https://discord.com/api/webhooks/123456789012345678/token-one",
						Embed:      json.RawMessage(`{"title":"one"}`),
					},
					{
						MessageID:  "123456789012345678",
						WebhookURL: "https://discord.com/api/webhooks/123456789012345679/token-two",
						Embed:      json.RawMessage(`{"title":"two"}`),
					},
				},
			},
		},
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPut, "/v1/settings/global", payload)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate webhook embed updates, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestGuildSettingsPutGetListAndDelete(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"}, BotGuildBinding{GuildID: "g2"})
	handler := srv.httpServer.Handler
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "g2"}); err != nil {
		t.Fatalf("seed second guild: %v", err)
	}

	getCV := func(guildID string) *int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == guildID {
				v := g.ConfigVersion
				return &v
			}
		}
		return nil
	}

	payload := updateGuildSettingsRequest{
		ConfigVersion: getCV("g2"),
		Channels: &files.ChannelsConfig{
			Commands:       "100",
			AutomodAction:  "200",
			ModerationCase: "300",
			EntryBackfill:  "400",
		},
		Cache: &guildCacheSettingsSection{
			RolesCacheTTL:   "10m",
			MemberCacheTTL:  "15m",
			GuildCacheTTL:   "30m",
			ChannelCacheTTL: "45m",
		},
		Runtime: &runtimeSettingsSections{
			Backfill: runtimeBackfillSection{
				BackfillInitialDate: "2026-03-01",
			},
		},
	}

	putRec := performHandlerJSONRequest(t, handler, http.MethodPut, "/v1/guilds/g2/settings", payload)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT /v1/guilds/g2/settings status=%d body=%q", putRec.Code, putRec.Body.String())
	}

	var putResp guildSettingsResponse
	if err := json.NewDecoder(putRec.Body).Decode(&putResp); err != nil {
		t.Fatalf("decode guild settings response: %v", err)
	}
	if putResp.Workspace.GuildID != "g2" {
		t.Fatalf("unexpected guild id in workspace: %+v", putResp.Workspace)
	}
	if putResp.Workspace.Sections.Cache.MemberCacheTTL != "15m" {
		t.Fatalf("unexpected cache section: %+v", putResp.Workspace.Sections.Cache)
	}
	if putResp.Workspace.Sections.Runtime.Backfill.BackfillInitialDate != "2026-03-01" {
		t.Fatalf("unexpected runtime backfill section: %+v", putResp.Workspace.Sections.Runtime.Backfill)
	}

	getRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g2/settings", nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/guilds/g2/settings status=%d body=%q", getRec.Code, getRec.Body.String())
	}

	listRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/settings/guilds", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/settings/guilds status=%d body=%q", listRec.Code, listRec.Body.String())
	}
	var listResp guildRegistryResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode configured guilds response: %v", err)
	}
	if len(listResp.Guilds) != 2 {
		t.Fatalf("expected two configured guilds, got %+v", listResp.Guilds)
	}
	if listResp.Workspace.ConfiguredCount != 2 {
		t.Fatalf("expected registry configured_count=2, got %+v", listResp.Workspace)
	}

	deleteRec := performHandlerJSONRequest(t, handler, http.MethodDelete, "/v1/guilds/g2/settings", nil)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("DELETE /v1/guilds/g2/settings status=%d body=%q", deleteRec.Code, deleteRec.Body.String())
	}

	getDeletedRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g2/settings", nil)
	if getDeletedRec.Code != http.StatusNotFound {
		t.Fatalf("expected deleted guild settings to return 404, got %d body=%q", getDeletedRec.Code, getDeletedRec.Body.String())
	}
}

func TestGuildSettingsGetOmitsLegacyModerationWorkspaceSection(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"})

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/guilds/g1/settings status=%d body=%q", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode guild settings payload: %v", err)
	}

	workspace, ok := payload["workspace"].(map[string]any)
	if !ok {
		t.Fatalf("expected workspace object, got %+v", payload)
	}
	sections, ok := workspace["sections"].(map[string]any)
	if !ok {
		t.Fatalf("expected sections object, got %+v", workspace)
	}
	if _, exists := sections["moderation"]; exists {
		t.Fatalf("did not expect legacy moderation section in workspace: %+v", sections)
	}
}

func TestGuildSettingsPutAutoRegistersMissingGuild(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g2"})

	// Setup a mock guild registration function since auto-register depends on it
	srv.SetGuildRegistrationFunc(func(_ context.Context, guildID string) error {
		_, err := cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
			cfg.Guilds = append(cfg.Guilds, files.GuildConfig{
				GuildID: guildID,
			})
			return nil
		})

		return err
	})

	payload := updateGuildSettingsRequest{
		Channels: &files.ChannelsConfig{Commands: "100"},
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPut, "/v1/guilds/g2/settings", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after auto-registration, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestGuildSettingsPutSavesBotInstanceTokenForUndiscoveredGuild(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	// Bot bindings cover only g1; g2 has no live session yet, so resolving its
	// bot instances returns errGuildDiscoveryRequired. The bootstrap token write
	// must still succeed so the first token can ever be saved.
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"})
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "g2"}); err != nil {
		t.Fatalf("seed guild g2: %v", err)
	}

	getCV := func(guildID string) *int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == guildID {
				v := g.ConfigVersion
				return &v
			}
		}
		return nil
	}

	tokens := map[string]string{"generic": "bootstrap-token"}
	payload := updateGuildSettingsRequest{
		ConfigVersion:     getCV("g2"),
		BotInstanceTokens: &tokens,
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPut, "/v1/guilds/g2/settings", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK saving bootstrap token for undiscovered guild, got %d body=%q", rec.Code, rec.Body.String())
	}

	cfg := cm.SnapshotConfig()
	guild, ok := findGuildSettings(cfg, "g2")
	if !ok {
		t.Fatal("expected guild g2 in config")
	}
	if got := string(guild.BotInstanceTokens["generic"]); got != "bootstrap-token" {
		t.Fatalf("expected persisted bootstrap token, got %q", got)
	}
}

func TestGuildRegistrationPostCreatesGuildWorkspace(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g2"})
	callCount := 0
	srv.SetGuildRegistrationFunc(func(_ context.Context, guildID string) error {
		callCount++
		_, err := cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
			cfg.Guilds = append(cfg.Guilds, files.GuildConfig{
				GuildID: guildID,
				Channels: files.ChannelsConfig{
					Commands: "100",
				},
			})
			return nil
		})

		return err
	})

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPost, "/v1/settings/guilds", registerGuildRequest{GuildID: "g2"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/settings/guilds status=%d body=%q", rec.Code, rec.Body.String())
	}

	var response guildRegistrationResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode guild registration response: %v", err)
	}
	if !response.Created {
		t.Fatalf("expected created=true, got %+v", response)
	}
	if response.GuildID != "g2" || response.Workspace.GuildID != "g2" {
		t.Fatalf("unexpected guild id in registration response: %+v", response)
	}
	if callCount != 1 {
		t.Fatalf("expected one registration call, got %d", callCount)
	}
}

func TestGuildRegistrationPostCreatesDormantGuildWorkspace(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g2", BotInstanceID: "generic"})
	srv.SetGuildRegistrationResolver(func(_ context.Context, guildID string) error {
		return cm.EnsureMinimalGuildConfig(guildID)
	})

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPost, "/v1/settings/guilds", registerGuildRequest{GuildID: "g2"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/settings/guilds status=%d body=%q", rec.Code, rec.Body.String())
	}

	var response guildRegistrationResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode guild registration response: %v", err)
	}
	if !response.Created {
		t.Fatalf("expected created=true, got %+v", response)
	}

	if response.Workspace.Sections.Channels != (files.ChannelsConfig{}) {
		t.Fatalf("expected empty channels for dormant guild, got %+v", response.Workspace.Sections.Channels)
	}
	if len(response.Workspace.Sections.Roles.Allowed) != 0 ||
		response.Workspace.Sections.Roles.AutoAssignment.Enabled ||
		response.Workspace.Sections.Roles.AutoAssignment.TargetRoleID != "" ||
		len(response.Workspace.Sections.Roles.AutoAssignment.RequiredRoles) != 0 {
		t.Fatalf("expected empty roles for dormant guild, got %+v", response.Workspace.Sections.Roles)
	}
	if response.Workspace.Effective.Features.Services.Monitoring ||
		response.Workspace.Effective.Features.Logging.MemberJoin ||
		response.Workspace.Effective.Features.MuteRole {
		t.Fatalf("expected dormant effective features to stay disabled, got %+v", response.Workspace.Effective.Features)
	}

	cfg := cm.SnapshotConfig()
	guild, ok := findGuildSettings(cfg, "g2")
	if !ok {
		t.Fatal("expected registered guild g2 in config")
	}

	if guild.Channels != (files.ChannelsConfig{}) {
		t.Fatalf("expected persisted dormant guild channels to remain empty, got %+v", guild.Channels)
	}
}

func TestGuildRegistrationPostReturnsExistingWorkspaceWhenAlreadyConfigured(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"})
	callCount := 0
	srv.SetGuildRegistrationFunc(func(context.Context, string) error {
		callCount++
		return nil
	})

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPost, "/v1/settings/guilds", registerGuildRequest{GuildID: "g1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for existing guild registration, got %d body=%q", rec.Code, rec.Body.String())
	}

	var response guildRegistrationResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode existing registration response: %v", err)
	}
	if response.Created {
		t.Fatalf("expected created=false, got %+v", response)
	}
	if callCount != 0 {
		t.Fatalf("expected no registration call for existing guild, got %d", callCount)
	}
}

func TestGuildRegistrationPostRejectsWhenBootstrapUnavailable(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g2"})

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPost, "/v1/settings/guilds", registerGuildRequest{GuildID: "g2"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when registration bootstrap is unavailable, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestGuildRegistrationPostRejectsUndiscoveredGuild(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"})
	srv.SetGuildRegistrationFunc(func(context.Context, string) error {
		t.Fatal("registration should not run for undiscovered guild")
		return nil
	})

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPost, "/v1/settings/guilds", registerGuildRequest{GuildID: "g2"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for undiscovered guild registration, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestGuildRegistryWorkspaceIncludesAvailableGuilds(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1", "g2", "g3"}, nil
	})

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/settings/guilds", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/settings/guilds status=%d body=%q", rec.Code, rec.Body.String())
	}

	var response guildRegistryResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode registry response: %v", err)
	}
	if response.Workspace.ConfiguredCount != 1 || response.Workspace.AvailableCount != 2 {
		t.Fatalf("unexpected registry counts: %+v", response.Workspace)
	}
	if len(response.Workspace.Entries) != 3 {
		t.Fatalf("expected three registry entries, got %+v", response.Workspace.Entries)
	}
	if len(response.Guilds) != 1 {
		t.Fatalf("expected configured guild alias to contain one entry, got %+v", response.Guilds)
	}

	configured := make(map[string]bool, len(response.Workspace.Entries))
	for _, entry := range response.Workspace.Entries {
		configured[entry.GuildID] = entry.Configured
	}
	if !configured["g1"] || configured["g2"] || configured["g3"] {
		t.Fatalf("unexpected configured flags: %+v", response.Workspace.Entries)
	}
}

func TestGuildSettingsPutRejectsInvalidAutoAssignmentOrdering(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"})

	getCV := func(guildID string) *int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == guildID {
				v := g.ConfigVersion
				return &v
			}
		}
		return nil
	}

	payload := updateGuildSettingsRequest{
		ConfigVersion: getCV("g1"),
		Roles: &files.RolesConfig{
			AutoAssignment: files.AutoAssignmentConfig{
				Enabled:       true,
				TargetRoleID:  "target-role",
				RequiredRoles: []string{"stable-role"},
			},
		},
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPut, "/v1/guilds/g1/settings", payload)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid auto-assignment ordering, got %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), files.ErrValidationFailed) {
		t.Fatalf("expected validation error body, got %q", rec.Body.String())
	}
}

func TestStatusForSettingsMutationErrorTreatsWrappedValidationAsBadRequest(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf(
		"update settings: %w",
		fmt.Errorf(
			"save config: %w",
			files.NewValidationError(
				"guilds[0].roles.auto_assignment.required_roles",
				[]string{"stable-role"},
				"required_roles must contain exactly 2 role IDs",
			),
		),
	)

	if got := statusForSettingsMutationError(err); got != http.StatusBadRequest {
		t.Fatalf("statusForSettingsMutationError() = %d, want %d", got, http.StatusBadRequest)
	}
}

func TestSettingsRoutesRequireAuthorization(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	rec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/v1/settings", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func testBoolPtr(v bool) *bool {
	return &v
}

func TestGuildSettingsPutScrubsDanglingFeatureRouting(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"})

	_, err := cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		guild, ok := findGuildSettingsMutable(cfg, "g1")
		if !ok {
			return fmt.Errorf("guild g1 not found")
		}
		guild.BotInstanceTokens = map[string]files.EncryptedString{
			"bot-1": "token-1",
			"bot-2": "token-2",
		}
		guild.FeatureRouting = map[string]string{
			"qotd":  "bot-2",
			"music": "bot-1",
		}
		return nil
	})

	if err != nil {
		t.Fatalf("seed config: %v", err)
	}

	getCV := func(guildID string) *int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == guildID {
				v := g.ConfigVersion
				return &v
			}
		}
		return nil
	}

	tokens := map[string]string{"bot-2": ""}
	payload := updateGuildSettingsRequest{
		ConfigVersion:     getCV("g1"),
		BotInstanceTokens: &tokens,
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPut, "/v1/guilds/g1/settings", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%q", rec.Code, rec.Body.String())
	}

	cfg := cm.SnapshotConfig()
	guild, _ := findGuildSettings(cfg, "g1")
	if _, ok := guild.FeatureRouting["qotd"]; ok {
		t.Fatalf("expected qotd routing to be scrubbed")
	}
	if guild.FeatureRouting["music"] != "bot-1" {
		t.Fatalf("expected music routing to remain intact")
	}

	tokens2 := map[string]string{"bot-1": ""}
	payload2 := updateGuildSettingsRequest{
		ConfigVersion:     getCV("g1"),
		BotInstanceTokens: &tokens2,
	}
	rec2 := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPut, "/v1/guilds/g1/settings", payload2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("PUT 2 status=%d body=%q", rec2.Code, rec2.Body.String())
	}

	cfg = cm.SnapshotConfig()
	guild, _ = findGuildSettings(cfg, "g1")
	if _, ok := guild.FeatureRouting["music"]; ok {
		t.Fatalf("expected music routing to be scrubbed")
	}
}

func TestGuildSettingsPut_TokenUniqueness(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"}, BotGuildBinding{GuildID: "g2"})

	_, err := cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "g1",
				BotInstanceTokens: map[string]files.EncryptedString{
					"bot-1": "secret-token",
				},
			},
			{
				GuildID: "g2",
			},
		}
		return nil
	})

	if err != nil {
		t.Fatalf("seed config: %v", err)
	}

	getCV := func(guildID string) *int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == guildID {
				v := g.ConfigVersion
				return &v
			}
		}
		return nil
	}

	tokens := map[string]string{"bot-2": "secret-token"}
	payload := updateGuildSettingsRequest{
		ConfigVersion:     getCV("g2"),
		BotInstanceTokens: &tokens,
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPut, "/v1/guilds/g2/settings", payload)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request due to token conflict, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestGuildSettingsPut_StatusCleanup(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	setTestBotGuildBindings(srv, BotGuildBinding{GuildID: "g1"})

	_, err := cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "g1",
				BotInstanceTokens: map[string]files.EncryptedString{
					"bot-1": "secret-token",
				},
				BotInstanceStatuses: map[string]string{
					"bot-1": "idle",
				},
			},
		}
		return nil
	})

	if err != nil {
		t.Fatalf("seed config: %v", err)
	}

	getCV := func(guildID string) *int64 {
		for _, g := range cm.SnapshotConfig().Guilds {
			if g.GuildID == guildID {
				v := g.ConfigVersion
				return &v
			}
		}
		return nil
	}

	tokens := map[string]string{"bot-1": ""}
	payload := updateGuildSettingsRequest{
		ConfigVersion:     getCV("g1"),
		BotInstanceTokens: &tokens,
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPut, "/v1/guilds/g1/settings", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%q", rec.Code, rec.Body.String())
	}

	cfg := cm.SnapshotConfig()
	guild, _ := findGuildSettings(cfg, "g1")
	if _, ok := guild.BotInstanceStatuses["bot-1"]; ok {
		t.Fatalf("expected bot-1 status to be scrubbed when token is deleted")
	}
}
