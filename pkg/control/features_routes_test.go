package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type featureCatalogResponse struct {
	Status  string                `json:"status"`
	Catalog []featureCatalogEntry `json:"catalog"`
}

type featureWorkspaceResponse struct {
	Status    string           `json:"status"`
	GuildID   string           `json:"guild_id"`
	Workspace featureWorkspace `json:"workspace"`
}

type featureRecordResponse struct {
	Status  string        `json:"status"`
	GuildID string        `json:"guild_id"`
	Feature featureRecord `json:"feature"`
}

func TestFeatureCatalogAndWorkspaceRoutes(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	srv.partnerBoardService = nil
	handler := srv.httpServer.Handler

	catalogRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/features/catalog", nil)
	if catalogRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/features/catalog status=%d body=%q", catalogRec.Code, catalogRec.Body.String())
	}

	catalog := decodeFeatureResponse[featureCatalogResponse](t, catalogRec)
	if len(catalog.Catalog) != len(featureDefinitions) {
		t.Fatalf("unexpected catalog size: got=%d want=%d", len(catalog.Catalog), len(featureDefinitions))
	}
	if catalog.Catalog[0].ID == "" || catalog.Catalog[0].Label == "" {
		t.Fatalf("expected populated catalog entry, got %+v", catalog.Catalog[0])
	}

	listRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/features", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/features status=%d body=%q", listRec.Code, listRec.Body.String())
	}

	globalWorkspace := decodeFeatureResponse[featureWorkspaceResponse](t, listRec)
	if globalWorkspace.Workspace.Scope != "global" {
		t.Fatalf("unexpected global workspace scope: %+v", globalWorkspace.Workspace)
	}
	if len(globalWorkspace.Workspace.Features) != len(featureDefinitions) {
		t.Fatalf("unexpected global feature count: got=%d want=%d", len(globalWorkspace.Workspace.Features), len(featureDefinitions))
	}

	guildRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g1/features", nil)
	if guildRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/guilds/g1/features status=%d body=%q", guildRec.Code, guildRec.Body.String())
	}

	guildWorkspace := decodeFeatureResponse[featureWorkspaceResponse](t, guildRec)
	if guildWorkspace.Workspace.Scope != "guild" || guildWorkspace.Workspace.GuildID != "g1" {
		t.Fatalf("unexpected guild workspace: %+v", guildWorkspace.Workspace)
	}
	if len(guildWorkspace.Workspace.Features) != len(featureDefinitions) {
		t.Fatalf("unexpected guild feature count: got=%d want=%d", len(guildWorkspace.Workspace.Features), len(featureDefinitions))
	}

	singleRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/features/services.monitoring", nil)
	if singleRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/features/services.monitoring status=%d body=%q", singleRec.Code, singleRec.Body.String())
	}

	single := decodeFeatureResponse[featureRecordResponse](t, singleRec)
	if single.Feature.ID != "services.monitoring" {
		t.Fatalf("unexpected feature payload: %+v", single.Feature)
	}
}

func TestFeaturePatchInheritanceAndClears(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	handler := srv.httpServer.Handler

	globalDisable := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/features/services.monitoring", map[string]any{
		"enabled": false,
	})
	if globalDisable.Code != http.StatusOK {
		t.Fatalf("PATCH global disable status=%d body=%q", globalDisable.Code, globalDisable.Body.String())
	}

	globalDisabled := decodeFeatureResponse[featureRecordResponse](t, globalDisable)
	if globalDisabled.Feature.OverrideState != "disabled" || globalDisabled.Feature.EffectiveEnabled || globalDisabled.Feature.EffectiveSource != "global" {
		t.Fatalf("unexpected global disabled feature: %+v", globalDisabled.Feature)
	}

	guildEnable := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/services.monitoring", map[string]any{
		"enabled": true,
	})
	if guildEnable.Code != http.StatusOK {
		t.Fatalf("PATCH guild enable status=%d body=%q", guildEnable.Code, guildEnable.Body.String())
	}

	guildEnabled := decodeFeatureResponse[featureRecordResponse](t, guildEnable)
	if guildEnabled.Feature.OverrideState != "enabled" || !guildEnabled.Feature.EffectiveEnabled || guildEnabled.Feature.EffectiveSource != "guild" {
		t.Fatalf("unexpected guild enabled feature: %+v", guildEnabled.Feature)
	}

	guildClear := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g1/features/services.monitoring", map[string]any{
		"enabled": nil,
	})
	if guildClear.Code != http.StatusOK {
		t.Fatalf("PATCH guild clear status=%d body=%q", guildClear.Code, guildClear.Body.String())
	}

	guildInherited := decodeFeatureResponse[featureRecordResponse](t, guildClear)
	if guildInherited.Feature.OverrideState != "inherit" || guildInherited.Feature.EffectiveEnabled || guildInherited.Feature.EffectiveSource != "global" {
		t.Fatalf("unexpected guild inherited feature: %+v", guildInherited.Feature)
	}

	globalClear := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/features/services.monitoring", map[string]any{
		"enabled": nil,
	})
	if globalClear.Code != http.StatusOK {
		t.Fatalf("PATCH global clear status=%d body=%q", globalClear.Code, globalClear.Body.String())
	}

	globalDefault := decodeFeatureResponse[featureRecordResponse](t, globalClear)
	if globalDefault.Feature.OverrideState != "default" || !globalDefault.Feature.EffectiveEnabled || globalDefault.Feature.EffectiveSource != "built_in" {
		t.Fatalf("unexpected global default feature: %+v", globalDefault.Feature)
	}
}

func TestGuildFeaturePatchPersistsConfigDetails(t *testing.T) {
	t.Parallel()

	srv, cm := newControlTestServer(t)
	handler := srv.httpServer.Handler

	requests := []struct {
		path    string
		payload map[string]any
	}{
		{path: "/v1/guilds/g1/features/services.commands", payload: map[string]any{"channel_id": "cmd-channel"}},
		{path: "/v1/guilds/g1/features/services.admin_commands", payload: map[string]any{"allowed_role_ids": []string{"admin-a", "admin-b", "admin-a", ""}}},
		{path: "/v1/guilds/g1/features/logging.member_join", payload: map[string]any{"channel_id": "join-channel"}},
		{path: "/v1/guilds/g1/features/presence_watch.user", payload: map[string]any{"user_id": "user-42"}},
		{path: "/v1/guilds/g1/features/safety.bot_role_perm_mirror", payload: map[string]any{"actor_role_id": "actor-7"}},
		{path: "/v1/guilds/g1/features/backfill.enabled", payload: map[string]any{"channel_id": "backfill-channel", "start_day": "2026-03-10", "initial_date": "2026-03-01"}},
		{path: "/v1/guilds/g1/features/stats_channels", payload: map[string]any{"config_enabled": false, "update_interval_mins": 15}},
		{path: "/v1/guilds/g1/features/auto_role_assignment", payload: map[string]any{"config_enabled": true, "target_role_id": "target-role", "required_role_ids": []string{"level-role", "booster-role"}}},
		{path: "/v1/guilds/g1/features/user_prune", payload: map[string]any{"config_enabled": true, "grace_days": 31, "scan_interval_mins": 45, "initial_delay_secs": 90, "kicks_per_second": 2, "max_kicks_per_run": 8, "exempt_role_ids": []string{"staff", "vip"}, "dry_run": true}},
	}

	for _, req := range requests {
		req := req
		t.Run(req.path, func(t *testing.T) {
			rec := performHandlerJSONRequest(t, handler, http.MethodPatch, req.path, req.payload)
			if rec.Code != http.StatusOK {
				t.Fatalf("PATCH %s status=%d body=%q", req.path, rec.Code, rec.Body.String())
			}
		})
	}

	cfg := cm.SnapshotConfig()
	guild, ok := findGuildSettings(cfg, "g1")
	if !ok {
		t.Fatal("expected guild g1 in snapshot")
	}

	if guild.Channels.Commands != "cmd-channel" {
		t.Fatalf("expected commands channel persisted, got %+v", guild.Channels)
	}
	if got := strings.Join(guild.Roles.Allowed, ","); got != "admin-a,admin-b" {
		t.Fatalf("expected allowed roles persisted without duplicates, got %q", got)
	}
	if guild.Channels.MemberJoin != "join-channel" {
		t.Fatalf("expected member_join channel persisted, got %+v", guild.Channels)
	}
	if guild.RuntimeConfig.PresenceWatchUserID != "user-42" {
		t.Fatalf("expected presence watch user persisted, got %+v", guild.RuntimeConfig)
	}
	if guild.RuntimeConfig.BotRolePermMirrorActorRoleID != "actor-7" {
		t.Fatalf("expected actor role persisted, got %+v", guild.RuntimeConfig)
	}
	if guild.Channels.EntryBackfill != "backfill-channel" || guild.RuntimeConfig.BackfillStartDay != "2026-03-10" || guild.RuntimeConfig.BackfillInitialDate != "2026-03-01" {
		t.Fatalf("expected backfill settings persisted, got channels=%+v runtime=%+v", guild.Channels, guild.RuntimeConfig)
	}
	if guild.Stats.Enabled || guild.Stats.UpdateIntervalMins != 15 {
		t.Fatalf("expected stats settings persisted, got %+v", guild.Stats)
	}
	if !guild.Roles.AutoAssignment.Enabled || guild.Roles.AutoAssignment.TargetRoleID != "target-role" {
		t.Fatalf("expected auto assignment persisted, got %+v", guild.Roles.AutoAssignment)
	}
	if got := strings.Join(guild.Roles.AutoAssignment.RequiredRoles, ","); got != "level-role,booster-role" {
		t.Fatalf("unexpected required roles: %q", got)
	}
	if guild.Roles.BoosterRole != "booster-role" {
		t.Fatalf("expected booster role backfilled from required roles, got %+v", guild.Roles)
	}
	if !guild.UserPrune.Enabled || guild.UserPrune.GraceDays != 31 || guild.UserPrune.ScanIntervalMins != 45 || guild.UserPrune.InitialDelaySecs != 90 || guild.UserPrune.KicksPerSecond != 2 || guild.UserPrune.MaxKicksPerRun != 8 || !guild.UserPrune.DryRun {
		t.Fatalf("expected user prune settings persisted, got %+v", guild.UserPrune)
	}
	if got := strings.Join(guild.UserPrune.ExemptRoleIDs, ","); got != "staff,vip" {
		t.Fatalf("unexpected exempt roles: %q", got)
	}
}

func TestFeaturePatchRejectsMissingAuthBadPayloadAndUnregisteredGuild(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	handler := srv.httpServer.Handler

	noAuth := performHandlerJSONRequestWithAuth(t, handler, http.MethodPatch, "/v1/features/services.monitoring", map[string]any{"enabled": false}, "")
	if noAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized patch, got status=%d body=%q", noAuth.Code, noAuth.Body.String())
	}

	emptyPayload := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/features/services.monitoring", map[string]any{})
	if emptyPayload.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty payload, got status=%d body=%q", emptyPayload.Code, emptyPayload.Body.String())
	}

	unsupportedField := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/features/services.monitoring", map[string]any{"channel_id": "nope"})
	if unsupportedField.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported field, got status=%d body=%q", unsupportedField.Code, unsupportedField.Body.String())
	}

	missingGuild := performHandlerJSONRequest(t, handler, http.MethodPatch, "/v1/guilds/g-missing/features/services.monitoring", map[string]any{"enabled": false})
	if missingGuild.Code != http.StatusConflict {
		t.Fatalf("expected 409 for unregistered guild, got status=%d body=%q", missingGuild.Code, missingGuild.Body.String())
	}
}

func TestLoggingFeatureReadinessStates(t *testing.T) {
	t.Parallel()

	t.Run("missing channel", func(t *testing.T) {
		t.Parallel()

		srv, _ := newControlTestServer(t)
		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/logging.member_join", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET member_join status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if response.Feature.Readiness != "blocked" {
			t.Fatalf("expected blocked readiness, got %+v", response.Feature)
		}
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "missing_channel" {
			t.Fatalf("expected missing_channel blocker, got %+v", response.Feature.Blockers)
		}
	})

	t.Run("runtime kill switch", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Channels.MemberJoin = "join-channel"
			cfg.Guilds[0].RuntimeConfig.DisableEntryExitLogs = true
			return nil
		})
		if err != nil {
			t.Fatalf("seed runtime kill switch: %v", err)
		}

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/logging.member_join", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET member_join status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "runtime_kill_switch" {
			t.Fatalf("expected runtime kill switch blocker, got %+v", response.Feature.Blockers)
		}
	})

	t.Run("missing intent", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Channels.MemberJoin = "join-channel"
			return nil
		})
		if err != nil {
			t.Fatalf("seed member_join channel: %v", err)
		}
		srv.SetDiscordSessionProvider(func() *discordgo.Session {
			return &discordgo.Session{Identify: discordgo.Identify{Intents: 0}}
		})

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/logging.member_join", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET member_join status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "missing_intent" {
			t.Fatalf("expected missing_intent blocker, got %+v", response.Feature.Blockers)
		}
	})

	t.Run("invalid moderation channel", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Channels.ModerationCase = "shared-channel"
			cfg.Guilds[0].Channels.AvatarLogging = "shared-channel"
			return nil
		})
		if err != nil {
			t.Fatalf("seed shared moderation channel: %v", err)
		}

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/logging.moderation_case", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET moderation_case status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "invalid_channel" {
			t.Fatalf("expected invalid_channel blocker, got %+v", response.Feature.Blockers)
		}
	})

	t.Run("ready", func(t *testing.T) {
		t.Parallel()

		srv, _ := newControlTestServer(t)
		srv.SetDiscordSessionProvider(func() *discordgo.Session {
			return &discordgo.Session{Identify: discordgo.Identify{Intents: discordgo.IntentsGuildMessages}}
		})

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/features/logging.message_process", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET message_process status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if response.Feature.Readiness != "ready" || len(response.Feature.Blockers) != 0 {
			t.Fatalf("expected ready feature, got %+v", response.Feature)
		}
	})
}

func decodeFeatureResponse[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var out T
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v body=%q", err, rec.Body.String())
	}
	return out
}
