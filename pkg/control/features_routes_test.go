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

type guildRoleOptionsResponse struct {
	Status  string            `json:"status"`
	GuildID string            `json:"guild_id"`
	Roles   []guildRoleOption `json:"roles"`
}

type guildChannelOptionsResponse struct {
	Status   string               `json:"status"`
	GuildID  string               `json:"guild_id"`
	Channels []guildChannelOption `json:"channels"`
}

type guildMemberOptionsResponse struct {
	Status  string              `json:"status"`
	GuildID string              `json:"guild_id"`
	Members []guildMemberOption `json:"members"`
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
	commandsCatalog, ok := slicesIndexBy(catalog.Catalog, func(item featureCatalogEntry) bool {
		return item.ID == "services.commands"
	})
	if !ok {
		t.Fatal("expected services.commands in catalog response")
	}
	if commandsCatalog.Area != featureAreaCommands || len(commandsCatalog.Tags) != 2 {
		t.Fatalf("expected area/tags metadata for services.commands, got %+v", commandsCatalog)
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
	statsFeature, ok := slicesIndexBy(guildWorkspace.Workspace.Features, func(item featureRecord) bool {
		return item.ID == "stats_channels"
	})
	if !ok {
		t.Fatal("expected stats_channels in guild workspace response")
	}
	if statsFeature.Area != featureAreaStats || len(statsFeature.Tags) != 2 {
		t.Fatalf("expected area/tags metadata for stats_channels, got %+v", statsFeature)
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

func slicesIndexBy[T any](items []T, match func(T) bool) (T, bool) {
	var zero T
	for _, item := range items {
		if match(item) {
			return item, true
		}
	}
	return zero, false
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
		{path: "/v1/guilds/g1/features/moderation.mute_role", payload: map[string]any{"role_id": "mute-role"}},
		{path: "/v1/guilds/g1/features/moderation.ban", payload: map[string]any{"enabled": true}},
		{path: "/v1/guilds/g1/features/moderation.massban", payload: map[string]any{"enabled": true}},
		{path: "/v1/guilds/g1/features/moderation.kick", payload: map[string]any{"enabled": true}},
		{path: "/v1/guilds/g1/features/moderation.timeout", payload: map[string]any{"enabled": true}},
		{path: "/v1/guilds/g1/features/moderation.warn", payload: map[string]any{"enabled": true}},
		{path: "/v1/guilds/g1/features/moderation.warnings", payload: map[string]any{"enabled": true}},
		{path: "/v1/guilds/g1/features/logging.member_join", payload: map[string]any{"channel_id": "join-channel"}},
		{path: "/v1/guilds/g1/features/logging.clean_action", payload: map[string]any{"enabled": true, "channel_id": "clean-log"}},
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
	if guild.Roles.MuteRole != "mute-role" {
		t.Fatalf("expected mute role persisted, got %+v", guild.Roles)
	}
	if guild.Features.Moderation.Ban == nil || !*guild.Features.Moderation.Ban ||
		guild.Features.Moderation.MassBan == nil || !*guild.Features.Moderation.MassBan ||
		guild.Features.Moderation.Kick == nil || !*guild.Features.Moderation.Kick ||
		guild.Features.Moderation.Timeout == nil || !*guild.Features.Moderation.Timeout ||
		guild.Features.Moderation.Warn == nil || !*guild.Features.Moderation.Warn ||
		guild.Features.Moderation.Warnings == nil || !*guild.Features.Moderation.Warnings {
		t.Fatalf("expected moderation command feature toggles to persist, got %+v", guild.Features.Moderation)
	}
	if guild.Channels.MemberJoin != "join-channel" {
		t.Fatalf("expected member_join channel persisted, got %+v", guild.Channels)
	}
	if guild.Features.Logging.CleanAction == nil || !*guild.Features.Logging.CleanAction {
		t.Fatalf("expected clean_action feature toggle persisted, got %+v", guild.Features.Logging)
	}
	if guild.Channels.CleanAction != "clean-log" {
		t.Fatalf("expected clean_action channel persisted, got %+v", guild.Channels)
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

	t.Run("clean action runtime kill switch", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Features.Logging.CleanAction = testBoolPtr(true)
			cfg.Guilds[0].Channels.CleanAction = "clean-log"
			cfg.Guilds[0].RuntimeConfig.DisableCleanLog = true
			return nil
		})
		if err != nil {
			t.Fatalf("seed clean action config: %v", err)
		}

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/logging.clean_action", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET clean_action status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if response.Feature.Readiness != "blocked" {
			t.Fatalf("expected blocked readiness, got %+v", response.Feature)
		}
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "runtime_kill_switch" {
			t.Fatalf("expected runtime_kill_switch blocker, got %+v", response.Feature.Blockers)
		}
		if response.Feature.Details["channel_id"] != "clean-log" {
			t.Fatalf("expected clean_action channel detail, got %+v", response.Feature.Details)
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

func TestModerationCommandFeatureReadinessStates(t *testing.T) {
	t.Parallel()

	t.Run("blocked when commands service disabled", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Features.Moderation.Ban = testBoolPtr(true)
			cfg.Guilds[0].Features.Services.Commands = testBoolPtr(false)
			return nil
		})
		if err != nil {
			t.Fatalf("seed moderation command config: %v", err)
		}

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/moderation.ban", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET moderation.ban status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if response.Feature.Readiness != "blocked" {
			t.Fatalf("expected blocked readiness, got %+v", response.Feature)
		}
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "commands_disabled" {
			t.Fatalf("expected commands_disabled blocker, got %+v", response.Feature.Blockers)
		}
	})

	t.Run("ready when commands service enabled", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Features.Moderation.Ban = testBoolPtr(true)
			cfg.Guilds[0].Features.Services.Commands = testBoolPtr(true)
			return nil
		})
		if err != nil {
			t.Fatalf("seed moderation command config: %v", err)
		}

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/moderation.ban", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET moderation.ban status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if response.Feature.Readiness != "ready" || len(response.Feature.Blockers) != 0 {
			t.Fatalf("expected ready moderation command feature, got %+v", response.Feature)
		}
	})
}

func TestGuildRoleOptionsRouteAndRoleBackedFeatureReadiness(t *testing.T) {
	t.Parallel()

	t.Run("lists guild role options", func(t *testing.T) {
		t.Parallel()

		srv, _ := newControlTestServer(t)
		srv.SetDiscordSessionProvider(func() *discordgo.Session {
			return newTestDiscordSessionWithGuildRoles("g1",
				&discordgo.Role{ID: "g1", Name: "@everyone", Position: 0},
				&discordgo.Role{ID: "booster-role", Name: "Booster", Position: 9},
				&discordgo.Role{ID: "target-role", Name: "Partner", Position: 3},
			)
		})

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/role-options", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /v1/guilds/g1/role-options status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[guildRoleOptionsResponse](t, rec)
		if len(response.Roles) != 3 {
			t.Fatalf("expected 3 role options, got %+v", response.Roles)
		}
		if response.Roles[0].ID != "booster-role" || response.Roles[1].ID != "target-role" || response.Roles[2].ID != "g1" {
			t.Fatalf("expected roles sorted by descending position, got %+v", response.Roles)
		}
		if !response.Roles[2].IsDefault {
			t.Fatalf("expected guild default role flagged, got %+v", response.Roles[2])
		}
	})

	t.Run("lists guild channel options", func(t *testing.T) {
		t.Parallel()

		srv, _ := newControlTestServer(t)
		srv.SetDiscordSessionProvider(func() *discordgo.Session {
			return newTestDiscordSessionWithGuildChannels("g1",
				&discordgo.Channel{ID: "cat-ops", Name: "Operations", Type: discordgo.ChannelTypeGuildCategory, Position: 5},
				&discordgo.Channel{ID: "logs", Name: "logs", Type: discordgo.ChannelTypeGuildText, Position: 4, ParentID: "cat-ops"},
				&discordgo.Channel{ID: "alerts", Name: "alerts", Type: discordgo.ChannelTypeGuildNews, Position: 3, ParentID: "cat-ops"},
				&discordgo.Channel{ID: "voice-hub", Name: "Voice Hub", Type: discordgo.ChannelTypeGuildVoice, Position: 2},
			)
		})

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/channel-options", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /v1/guilds/g1/channel-options status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[guildChannelOptionsResponse](t, rec)
		if len(response.Channels) != 3 {
			t.Fatalf("expected 3 channel options without categories, got %+v", response.Channels)
		}
		if response.Channels[0].ID != "logs" || response.Channels[1].ID != "alerts" || response.Channels[2].ID != "voice-hub" {
			t.Fatalf("expected channel-only ordering, got %+v", response.Channels)
		}
		if response.Channels[0].DisplayName != "#logs" {
			t.Fatalf("expected display name for text channel without category prefix, got %+v", response.Channels[0])
		}
		if !response.Channels[0].SupportsMessageRoute || !response.Channels[1].SupportsMessageRoute {
			t.Fatalf("expected text and announcement channels to support message routes, got %+v", response.Channels)
		}
		if response.Channels[2].SupportsMessageRoute {
			t.Fatalf("expected voice channel to be excluded from message routes, got %+v", response.Channels[2])
		}
	})

	t.Run("lists guild member options with selected member pinned", func(t *testing.T) {
		t.Parallel()

		srv, _ := newControlTestServer(t)
		srv.SetDiscordSessionProvider(func() *discordgo.Session {
			return newTestDiscordSessionWithGuildMembers("g1",
				&discordgo.Member{
					GuildID: "g1",
					Nick:    "Alice Alpha",
					User: &discordgo.User{
						ID:       "user-alice",
						Username: "alice",
					},
				},
				&discordgo.Member{
					GuildID: "g1",
					Nick:    "Bob",
					User: &discordgo.User{
						ID:       "user-bob",
						Username: "bob",
					},
				},
				&discordgo.Member{
					GuildID: "g1",
					Nick:    "Carol",
					User: &discordgo.User{
						ID:       "user-carol",
						Username: "carol",
					},
				},
			)
		})

		rec := performHandlerJSONRequest(
			t,
			srv.httpServer.Handler,
			http.MethodGet,
			"/v1/guilds/g1/member-options?query=ali&selected_id=user-bob&limit=3",
			nil,
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /v1/guilds/g1/member-options status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[guildMemberOptionsResponse](t, rec)
		if len(response.Members) != 2 {
			t.Fatalf("expected 2 member options, got %+v", response.Members)
		}
		if response.Members[0].ID != "user-bob" {
			t.Fatalf("expected selected member first, got %+v", response.Members)
		}
		if response.Members[1].ID != "user-alice" {
			t.Fatalf("expected query match after selected member, got %+v", response.Members)
		}
	})

	t.Run("mute role blocks missing role", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Features.MuteRole = testBoolPtr(true)
			cfg.Guilds[0].Roles.MuteRole = ""
			return nil
		})
		if err != nil {
			t.Fatalf("seed mute role config: %v", err)
		}

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/moderation.mute_role", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET moderation.mute_role status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "missing_role" {
			t.Fatalf("expected missing_role blocker, got %+v", response.Feature.Blockers)
		}
	})

	t.Run("mute role blocks invalid role", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Features.MuteRole = testBoolPtr(true)
			cfg.Guilds[0].Roles.MuteRole = "mute-role"
			return nil
		})
		if err != nil {
			t.Fatalf("seed mute role config: %v", err)
		}

		srv.SetDiscordSessionProvider(func() *discordgo.Session {
			return newTestDiscordSessionWithGuildRoles("g1",
				&discordgo.Role{ID: "g1", Name: "@everyone", Position: 0},
				&discordgo.Role{ID: "helper-role", Name: "Helper", Position: 4},
			)
		})

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/moderation.mute_role", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET moderation.mute_role status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "invalid_role" {
			t.Fatalf("expected invalid_role blocker, got %+v", response.Feature.Blockers)
		}
	})

	t.Run("mute role is ready when role exists", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Features.MuteRole = testBoolPtr(true)
			cfg.Guilds[0].Roles.MuteRole = "mute-role"
			return nil
		})
		if err != nil {
			t.Fatalf("seed mute role config: %v", err)
		}

		srv.SetDiscordSessionProvider(func() *discordgo.Session {
			return newTestDiscordSessionWithGuildRoles("g1",
				&discordgo.Role{ID: "g1", Name: "@everyone", Position: 0},
				&discordgo.Role{ID: "mute-role", Name: "Muted", Position: 2},
			)
		})

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/moderation.mute_role", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET moderation.mute_role status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if response.Feature.Readiness != "ready" || len(response.Feature.Blockers) != 0 {
			t.Fatalf("expected ready mute role feature, got %+v", response.Feature)
		}
		if response.Feature.Details["role_id"] != "mute-role" {
			t.Fatalf("expected role_id detail, got %+v", response.Feature.Details)
		}
	})

	t.Run("stats channels expose configured channel inventory", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Features.StatsChannels = testBoolPtr(true)
			cfg.Guilds[0].Stats.Enabled = true
			cfg.Guilds[0].Stats.UpdateIntervalMins = 45
			cfg.Guilds[0].Stats.Channels = []files.StatsChannelConfig{
				{
					ChannelID:    "stats-total",
					Label:        "Total members",
					NameTemplate: "{label} | {count}",
					MemberType:   "all",
				},
				{
					ChannelID:  "stats-bots",
					Label:      "Bots",
					MemberType: "bots",
				},
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
		if response.Feature.Readiness != "ready" || len(response.Feature.Blockers) != 0 {
			t.Fatalf("expected ready stats feature, got %+v", response.Feature)
		}
		if response.Feature.Details["config_enabled"] != true {
			t.Fatalf("expected config_enabled detail, got %+v", response.Feature.Details)
		}
		if response.Feature.Details["update_interval_mins"] != float64(45) {
			t.Fatalf("expected update_interval_mins detail, got %+v", response.Feature.Details)
		}
		if response.Feature.Details["configured_channel_count"] != float64(2) {
			t.Fatalf("expected configured_channel_count detail, got %+v", response.Feature.Details)
		}

		channels, ok := response.Feature.Details["channels"].([]any)
		if !ok || len(channels) != 2 {
			t.Fatalf("expected two stats channel details, got %+v", response.Feature.Details["channels"])
		}

		firstChannel, ok := channels[0].(map[string]any)
		if !ok {
			t.Fatalf("expected first stats channel as map, got %+v", channels[0])
		}
		if firstChannel["channel_id"] != "stats-total" || firstChannel["label"] != "Total members" || firstChannel["name_template"] != "{label} | {count}" {
			t.Fatalf("unexpected first stats channel detail: %+v", firstChannel)
		}
		if firstChannel["member_type"] != "all" {
			t.Fatalf("expected first stats channel member_type, got %+v", firstChannel)
		}
	})

	t.Run("auto role assignment blocks invalid target role", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Features.AutoRoleAssign = testBoolPtr(true)
			cfg.Guilds[0].Roles.AutoAssignment.Enabled = true
			cfg.Guilds[0].Roles.AutoAssignment.TargetRoleID = "target-role"
			cfg.Guilds[0].Roles.AutoAssignment.RequiredRoles = []string{"level-role", "booster-role"}
			cfg.Guilds[0].Roles.BoosterRole = "booster-role"
			return nil
		})
		if err != nil {
			t.Fatalf("seed auto role config: %v", err)
		}

		srv.SetDiscordSessionProvider(func() *discordgo.Session {
			return newTestDiscordSessionWithGuildRoles("g1",
				&discordgo.Role{ID: "g1", Name: "@everyone", Position: 0},
				&discordgo.Role{ID: "level-role", Name: "Level", Position: 8},
				&discordgo.Role{ID: "booster-role", Name: "Booster", Position: 7},
			)
		})

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/auto_role_assignment", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET auto_role_assignment status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "invalid_target_role" {
			t.Fatalf("expected invalid_target_role blocker, got %+v", response.Feature.Blockers)
		}
		if response.Feature.Details["booster_role_id"] != "booster-role" {
			t.Fatalf("expected booster_role_id detail, got %+v", response.Feature.Details)
		}
		if response.Feature.Details["level_role_id"] != "level-role" {
			t.Fatalf("expected level_role_id detail, got %+v", response.Feature.Details)
		}
	})

	t.Run("permission mirror blocks invalid actor role", func(t *testing.T) {
		t.Parallel()

		srv, cm := newControlTestServer(t)
		_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
			cfg.Guilds[0].Features.Safety.BotRolePermMirror = testBoolPtr(true)
			cfg.Guilds[0].RuntimeConfig.BotRolePermMirrorActorRoleID = "actor-role"
			cfg.Guilds[0].RuntimeConfig.DisableBotRolePermMirror = false
			return nil
		})
		if err != nil {
			t.Fatalf("seed permission mirror config: %v", err)
		}

		srv.SetDiscordSessionProvider(func() *discordgo.Session {
			return newTestDiscordSessionWithGuildRoles("g1",
				&discordgo.Role{ID: "g1", Name: "@everyone", Position: 0},
				&discordgo.Role{ID: "helper-role", Name: "Helper", Position: 4},
			)
		})

		rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodGet, "/v1/guilds/g1/features/safety.bot_role_perm_mirror", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET safety.bot_role_perm_mirror status=%d body=%q", rec.Code, rec.Body.String())
		}

		response := decodeFeatureResponse[featureRecordResponse](t, rec)
		if len(response.Feature.Blockers) != 1 || response.Feature.Blockers[0].Code != "invalid_actor_role" {
			t.Fatalf("expected invalid_actor_role blocker, got %+v", response.Feature.Blockers)
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

func newTestDiscordSessionWithGuildRoles(guildID string, roles ...*discordgo.Role) *discordgo.Session {
	session := &discordgo.Session{
		State: discordgo.NewState(),
	}
	_ = session.State.GuildAdd(&discordgo.Guild{
		ID:    guildID,
		Roles: roles,
	})
	return session
}

func newTestDiscordSessionWithGuildMembers(guildID string, members ...*discordgo.Member) *discordgo.Session {
	session := &discordgo.Session{
		State: discordgo.NewState(),
	}
	_ = session.State.GuildAdd(&discordgo.Guild{
		ID:      guildID,
		Members: members,
	})
	for _, member := range members {
		if member == nil {
			continue
		}
		member.GuildID = guildID
		_ = session.State.MemberAdd(member)
	}
	return session
}

func newTestDiscordSessionWithGuildChannels(guildID string, channels ...*discordgo.Channel) *discordgo.Session {
	session := &discordgo.Session{
		State: discordgo.NewState(),
	}
	_ = session.State.GuildAdd(&discordgo.Guild{
		ID:       guildID,
		Channels: channels,
	})
	return session
}
