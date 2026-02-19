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

	if got := (RuntimeConfig{ModerationLogMode: "off"}).ModerationLoggingEnabled(); got {
		t.Fatal("expected legacy moderation_log_mode=off to disable logs")
	}

	if got := (RuntimeConfig{ModerationLogMode: "alice_only"}).ModerationLoggingEnabled(); !got {
		t.Fatal("expected legacy moderation_log_mode=alice_only to enable logs")
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

	legacyCfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			ModerationLogMode: "off",
		},
		Guilds: []GuildConfig{{GuildID: "g2"}},
	}
	legacyRC := legacyCfg.ResolveRuntimeConfig("g2")
	if legacyRC.ModerationLogging == nil || *legacyRC.ModerationLogging {
		t.Fatal("expected legacy global moderation_log_mode=off to resolve as disabled")
	}
}

func TestRuntimeConfigNormalizedWebhookEmbedUpdates(t *testing.T) {
	t.Parallel()

	legacy := RuntimeConfig{
		WebhookEmbedUpdate: WebhookEmbedUpdateConfig{
			MessageID:  "m1",
			WebhookURL: "https://discord.com/api/webhooks/1/token",
			Embed:      json.RawMessage(`{"title":"legacy"}`),
		},
	}
	legacyUpdates := legacy.NormalizedWebhookEmbedUpdates()
	if len(legacyUpdates) != 1 || legacyUpdates[0].MessageID != "m1" {
		t.Fatalf("expected legacy fallback with 1 item, got %+v", legacyUpdates)
	}

	list := RuntimeConfig{
		WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
			{},
			{
				MessageID:  "m2",
				WebhookURL: "https://discord.com/api/webhooks/2/token",
				Embed:      json.RawMessage(`{"title":"list"}`),
			},
		},
		WebhookEmbedUpdate: WebhookEmbedUpdateConfig{
			MessageID:  "legacy-ignored",
			WebhookURL: "https://discord.com/api/webhooks/9/token",
			Embed:      json.RawMessage(`{"title":"legacy"}`),
		},
	}
	listUpdates := list.NormalizedWebhookEmbedUpdates()
	if len(listUpdates) != 1 || listUpdates[0].MessageID != "m2" {
		t.Fatalf("expected list to win over legacy with filtered non-empty items, got %+v", listUpdates)
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
