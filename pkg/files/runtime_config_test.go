package files

import (
	"encoding/json"
	"testing"
)

func TestRuntimeConfigModerationLoggingEnabled(t *testing.T) {
	t.Parallel()

	if got := (RuntimeConfig{}).ModerationLoggingEnabled(); !got {
		t.Fatal("expected default moderation logging to be enabled")
	}

	enabled := true
	if got := (RuntimeConfig{ModerationLogging: &enabled}).ModerationLoggingEnabled(); !got {
		t.Fatal("expected moderation_logging=true to enable logs")
	}

	disabled := false
	if got := (RuntimeConfig{ModerationLogging: &disabled}).ModerationLoggingEnabled(); got {
		t.Fatal("expected moderation_logging=false to disable logs")
	}
}

func TestRuntimeConfigUnmarshalMigratesLegacyModerationLogMode(t *testing.T) {
	t.Parallel()

	var off RuntimeConfig
	if err := json.Unmarshal([]byte(`{"moderation_log_mode":"off"}`), &off); err != nil {
		t.Fatalf("unmarshal legacy off: %v", err)
	}
	if off.ModerationLogging == nil || *off.ModerationLogging {
		t.Fatalf("expected legacy moderation_log_mode=off to migrate to moderation_logging=false, got %+v", off.ModerationLogging)
	}

	var aliceOnly RuntimeConfig
	if err := json.Unmarshal([]byte(`{"moderation_log_mode":"alice_only"}`), &aliceOnly); err != nil {
		t.Fatalf("unmarshal legacy alice_only: %v", err)
	}
	if aliceOnly.ModerationLogging == nil || !*aliceOnly.ModerationLogging {
		t.Fatalf("expected legacy moderation_log_mode=alice_only to migrate to moderation_logging=true, got %+v", aliceOnly.ModerationLogging)
	}

	// Canonical value wins over the legacy key when both are present.
	var both RuntimeConfig
	if err := json.Unmarshal([]byte(`{"moderation_logging":true,"moderation_log_mode":"off"}`), &both); err != nil {
		t.Fatalf("unmarshal both: %v", err)
	}
	if both.ModerationLogging == nil || !*both.ModerationLogging {
		t.Fatalf("expected canonical moderation_logging=true to win, got %+v", both.ModerationLogging)
	}
}

func TestResolveRuntimeConfigModerationLoggingMerge(t *testing.T) {
	t.Parallel()

	globalEnabled := true
	guildDisabled := false

	cfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			ModerationLogging: &globalEnabled,
		},
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				RuntimeConfig: RuntimeConfig{
					ModerationLogging: &guildDisabled,
				},
			},
		},
	}

	rc := cfg.ResolveRuntimeConfig("g1")
	if rc.ModerationLogging == nil || *rc.ModerationLogging {
		t.Fatal("expected guild moderation_logging=false to override global true")
	}

	disabled := false
	legacyCfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			ModerationLogging: &disabled,
		},
		Guilds: []GuildConfig{{GuildID: "g2"}},
	}
	legacyRC := legacyCfg.ResolveRuntimeConfig("g2")
	if legacyRC.ModerationLogging == nil || *legacyRC.ModerationLogging {
		t.Fatal("expected global moderation_logging=false to resolve as disabled")
	}
}

func TestResolveRuntimeConfigGlobalMaxWorkersMerge(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			GlobalMaxWorkers: 8,
		},
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				RuntimeConfig: RuntimeConfig{
					GlobalMaxWorkers: 3,
				},
			},
		},
	}

	if got := cfg.ResolveRuntimeConfig("g1").GlobalMaxWorkers; got != 3 {
		t.Fatalf("expected guild override to win for global_max_workers, got %d", got)
	}
	if got := cfg.ResolveRuntimeConfig("g2").GlobalMaxWorkers; got != 8 {
		t.Fatalf("expected global fallback for unknown guild, got %d", got)
	}
}

func TestRuntimeConfigNormalizedWebhookEmbedUpdates(t *testing.T) {
	t.Parallel()

	list := RuntimeConfig{
		WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
			{},
			{
				MessageID:  "m2",
				WebhookURL: "https://discord.com/api/webhooks/2/token",
				Embed:      json.RawMessage(`{"title":"list"}`),
			},
		},
	}
	listUpdates := list.NormalizedWebhookEmbedUpdates()
	if len(listUpdates) != 1 || listUpdates[0].MessageID != "m2" {
		t.Fatalf("expected normalized list to drop empty placeholders, got %+v", listUpdates)
	}
}

func TestRuntimeConfigUnmarshalMigratesLegacyWebhookEmbedUpdate(t *testing.T) {
	t.Parallel()

	// Sole legacy single-entry key — migrates into the canonical list.
	var legacyOnly RuntimeConfig
	legacyJSON := `{"webhook_embed_update":{"message_id":"m1","webhook_url":"https://discord.com/api/webhooks/1/token","embed":{"title":"legacy"}}}`
	if err := json.Unmarshal([]byte(legacyJSON), &legacyOnly); err != nil {
		t.Fatalf("unmarshal legacy-only: %v", err)
	}
	if len(legacyOnly.WebhookEmbedUpdates) != 1 || legacyOnly.WebhookEmbedUpdates[0].MessageID != "m1" {
		t.Fatalf("expected legacy single key migrated into canonical list, got %+v", legacyOnly.WebhookEmbedUpdates)
	}

	// Canonical list with a non-empty entry shadows the legacy key.
	var both RuntimeConfig
	bothJSON := `{"webhook_embed_updates":[{"message_id":"m2","webhook_url":"https://discord.com/api/webhooks/2/token","embed":{"title":"list"}}],"webhook_embed_update":{"message_id":"legacy-ignored","webhook_url":"https://discord.com/api/webhooks/9/token","embed":{"title":"legacy"}}}`
	if err := json.Unmarshal([]byte(bothJSON), &both); err != nil {
		t.Fatalf("unmarshal both: %v", err)
	}
	if len(both.WebhookEmbedUpdates) != 1 || both.WebhookEmbedUpdates[0].MessageID != "m2" {
		t.Fatalf("expected canonical list to shadow legacy key, got %+v", both.WebhookEmbedUpdates)
	}
}

func TestResolveRuntimeConfigWebhookEmbedUpdatesMerge(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
				{
					MessageID:  "global",
					WebhookURL: "https://discord.com/api/webhooks/1/token",
					Embed:      json.RawMessage(`{"title":"global"}`),
				},
			},
		},
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				RuntimeConfig: RuntimeConfig{
					WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
						{
							MessageID:  "guild",
							WebhookURL: "https://discord.com/api/webhooks/2/token",
							Embed:      json.RawMessage(`{"title":"guild"}`),
						},
					},
				},
			},
		},
	}

	resolved := cfg.ResolveRuntimeConfig("g1")
	updates := resolved.NormalizedWebhookEmbedUpdates()
	if len(updates) != 1 || updates[0].MessageID != "guild" {
		t.Fatalf("expected guild list to override global list, got %+v", updates)
	}
}

func TestRuntimeConfigWebhookEmbedValidationDefaultsAndNormalization(t *testing.T) {
	t.Parallel()

	effective := (RuntimeConfig{}).EffectiveWebhookEmbedValidation()
	if effective.Mode != WebhookEmbedValidationModeOff {
		t.Fatalf("expected default mode=off, got %q", effective.Mode)
	}
	if effective.TimeoutMS != DefaultWebhookEmbedValidationTimeoutMS {
		t.Fatalf("expected default timeout=%d, got %d", DefaultWebhookEmbedValidationTimeoutMS, effective.TimeoutMS)
	}

	invalid := RuntimeConfig{
		WebhookEmbedValidation: WebhookEmbedValidationConfig{
			Mode:      "unknown",
			TimeoutMS: -10,
		},
	}.EffectiveWebhookEmbedValidation()
	if invalid.Mode != WebhookEmbedValidationModeOff {
		t.Fatalf("expected invalid mode to normalize to off, got %q", invalid.Mode)
	}
	if invalid.TimeoutMS != DefaultWebhookEmbedValidationTimeoutMS {
		t.Fatalf("expected invalid timeout to normalize to default, got %d", invalid.TimeoutMS)
	}

	strict := RuntimeConfig{
		WebhookEmbedValidation: WebhookEmbedValidationConfig{
			Mode:      "STRICT",
			TimeoutMS: 4500,
		},
	}.EffectiveWebhookEmbedValidation()
	if strict.Mode != WebhookEmbedValidationModeStrict {
		t.Fatalf("expected mode strict, got %q", strict.Mode)
	}
	if strict.TimeoutMS != 4500 {
		t.Fatalf("expected timeout 4500, got %d", strict.TimeoutMS)
	}
}

func TestResolveRuntimeConfigWebhookEmbedValidationMerge(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			WebhookEmbedValidation: WebhookEmbedValidationConfig{
				Mode:      WebhookEmbedValidationModeSoft,
				TimeoutMS: 3000,
			},
		},
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				RuntimeConfig: RuntimeConfig{
					WebhookEmbedValidation: WebhookEmbedValidationConfig{
						Mode: WebhookEmbedValidationModeStrict,
					},
				},
			},
			{
				GuildID: "g2",
				RuntimeConfig: RuntimeConfig{
					WebhookEmbedValidation: WebhookEmbedValidationConfig{
						TimeoutMS: 9000,
					},
				},
			},
		},
	}

	g1 := cfg.ResolveRuntimeConfig("g1").EffectiveWebhookEmbedValidation()
	if g1.Mode != WebhookEmbedValidationModeStrict {
		t.Fatalf("expected g1 mode strict, got %q", g1.Mode)
	}
	if g1.TimeoutMS != 3000 {
		t.Fatalf("expected g1 timeout fallback to global 3000, got %d", g1.TimeoutMS)
	}

	g2 := cfg.ResolveRuntimeConfig("g2").EffectiveWebhookEmbedValidation()
	if g2.Mode != WebhookEmbedValidationModeSoft {
		t.Fatalf("expected g2 mode fallback to global soft, got %q", g2.Mode)
	}
	if g2.TimeoutMS != 9000 {
		t.Fatalf("expected g2 timeout override 9000, got %d", g2.TimeoutMS)
	}
}

func TestNormalizeRuntimeConfigRejectsNegativeGlobalMaxWorkers(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeRuntimeConfig(RuntimeConfig{GlobalMaxWorkers: -1}); err == nil {
		t.Fatal("expected negative global_max_workers to be rejected")
	}
}
