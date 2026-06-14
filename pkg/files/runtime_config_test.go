package files

import (
	"encoding/json"
	"reflect"
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

	var botOnly RuntimeConfig
	if err := json.Unmarshal([]byte(`{"moderation_log_mode":"bot_only"}`), &botOnly); err != nil {
		t.Fatalf("unmarshal legacy bot_only: %v", err)
	}
	if botOnly.ModerationLogging == nil || !*botOnly.ModerationLogging {
		t.Fatalf("expected legacy moderation_log_mode=bot_only to migrate to moderation_logging=true, got %+v", botOnly.ModerationLogging)
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

// TestResolveRuntimeConfigAdoptsEveryGuildField guards against the silent-merge-gap
// failure mode of ResolveRuntimeConfig. That merge is ~30 hand-written
// "if guildRC.X != zero { resolved.X = guildRC.X }" blocks, so adding a field to
// RuntimeConfig without its matching merge line compiles cleanly and breaks no other
// test — the per-guild override is simply dropped at runtime. The merge stays explicit
// on purpose ("clear is better than clever"); this test, not reflection in production
// code, is the guard.
//
// Default rule: a non-zero guild sentinel must survive into the resolved config. Fields
// whose semantics differ are listed in exceptions and covered by dedicated assertions.
//
// Adding a new RuntimeConfig field? Either (a) merge it in ResolveRuntimeConfig so it
// satisfies the default adoption rule, or (b) add it to exceptions with its own
// assertion here. A field that is neither merged nor classified fails this test.
func TestResolveRuntimeConfigAdoptsEveryGuildField(t *testing.T) {
	t.Parallel()

	// Fields not covered by the default "non-zero guild sentinel is adopted" rule;
	// each has a dedicated assertion below.
	exceptions := map[string]string{
		"ModerationLogging":    "*bool with normalization (global defaults to non-nil)",
		"BackfillInitialDate":  "GuildOnly: adopts the guild value even when zero, no global fallback",
		"WebhookEmbedUpdates":  "slice merged via NormalizedWebhookEmbedUpdates (empty entries filtered)",
		"PastebinDevKey":       "global-only credential, intentionally not per-guild overridable",
		"PastebinUserName":     "global-only credential, intentionally not per-guild overridable",
		"PastebinUserPassword": "global-only credential, intentionally not per-guild overridable",
	}

	recurse := map[reflect.Type]bool{
		reflect.TypeOf(DatabaseRuntimeConfig{}):        true,
		reflect.TypeOf(WebhookEmbedValidationConfig{}): true,
	}
	leaves := runtimeConfigLeaves(reflect.TypeOf(RuntimeConfig{}), "", nil, recurse)

	// A stale exception (e.g. a renamed field) would let a new field slip past the
	// default loop unguarded, so fail if an exception no longer matches a real field.
	leafNames := make(map[string]bool, len(leaves))
	for _, lf := range leaves {
		leafNames[lf.name] = true
	}
	for name := range exceptions {
		if !leafNames[name] {
			t.Errorf("exceptions lists %q, which is not a RuntimeConfig field; remove or rename the stale entry", name)
		}
	}

	const testGuildID = "guild-under-test"

	for _, lf := range leaves {
		if _, skip := exceptions[lf.name]; skip {
			continue
		}
		t.Run(lf.name, func(t *testing.T) {
			var guildRC RuntimeConfig
			lf.setSentinel(t, reflect.ValueOf(&guildRC).Elem().FieldByIndex(lf.index))

			cfg := &BotConfig{
				Guilds: []GuildConfig{{GuildID: testGuildID, RuntimeConfig: guildRC}},
			}
			resolved := cfg.ResolveRuntimeConfig(testGuildID)
			lf.assertAdopted(t, reflect.ValueOf(resolved).FieldByIndex(lf.index))
		})
	}

	// Dedicated assertions for the exception fields.

	t.Run("ModerationLogging", func(t *testing.T) {
		cfg := &BotConfig{
			RuntimeConfig: RuntimeConfig{ModerationLogging: boolPtr(true)},
			Guilds: []GuildConfig{{
				GuildID:       testGuildID,
				RuntimeConfig: RuntimeConfig{ModerationLogging: boolPtr(false)},
			}},
		}
		resolved := cfg.ResolveRuntimeConfig(testGuildID)
		if resolved.ModerationLogging == nil || *resolved.ModerationLogging {
			t.Fatalf("expected guild moderation_logging=false to override global true, got %v", resolved.ModerationLogging)
		}
	})

	t.Run("BackfillInitialDate", func(t *testing.T) {
		adopted := &BotConfig{
			RuntimeConfig: RuntimeConfig{BackfillInitialDate: "2020-01-01"},
			Guilds: []GuildConfig{{
				GuildID:       testGuildID,
				RuntimeConfig: RuntimeConfig{BackfillInitialDate: "2024-12-31"},
			}},
		}
		if got := adopted.ResolveRuntimeConfig(testGuildID).BackfillInitialDate; got != "2024-12-31" {
			t.Fatalf("expected guild backfill_initial_date to be adopted, got %q", got)
		}
		// GuildOnly: an empty guild value clears the global instead of falling back.
		cleared := &BotConfig{
			RuntimeConfig: RuntimeConfig{BackfillInitialDate: "2020-01-01"},
			Guilds:        []GuildConfig{{GuildID: testGuildID}},
		}
		if got := cleared.ResolveRuntimeConfig(testGuildID).BackfillInitialDate; got != "" {
			t.Fatalf("expected GuildOnly backfill_initial_date to ignore the global fallback, got %q", got)
		}
	})

	t.Run("WebhookEmbedUpdates", func(t *testing.T) {
		cfg := &BotConfig{
			RuntimeConfig: RuntimeConfig{WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
				{MessageID: "global", WebhookURL: "https://discord.com/api/webhooks/1/token"},
			}},
			Guilds: []GuildConfig{{
				GuildID: testGuildID,
				RuntimeConfig: RuntimeConfig{WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
					{MessageID: "guild", WebhookURL: "https://discord.com/api/webhooks/2/token"},
				}},
			}},
		}
		updates := cfg.ResolveRuntimeConfig(testGuildID).NormalizedWebhookEmbedUpdates()
		if len(updates) != 1 || updates[0].MessageID != "guild" {
			t.Fatalf("expected guild webhook_embed_updates to override global, got %+v", updates)
		}
	})

	t.Run("PastebinGlobalOnly", func(t *testing.T) {
		cfg := &BotConfig{
			RuntimeConfig: RuntimeConfig{
				PastebinDevKey:       EncryptedString("global-dev-key"),
				PastebinUserName:     EncryptedString("global-user"),
				PastebinUserPassword: EncryptedString("global-pass"),
			},
			Guilds: []GuildConfig{{
				GuildID: testGuildID,
				RuntimeConfig: RuntimeConfig{
					PastebinDevKey:       EncryptedString("guild-dev-key"),
					PastebinUserName:     EncryptedString("guild-user"),
					PastebinUserPassword: EncryptedString("guild-pass"),
				},
			}},
		}
		resolved := cfg.ResolveRuntimeConfig(testGuildID)
		if resolved.PastebinDevKey != "global-dev-key" ||
			resolved.PastebinUserName != "global-user" ||
			resolved.PastebinUserPassword != "global-pass" {
			t.Fatalf("expected pastebin credentials to remain global-only, got dev=%q user=%q pass=%q",
				resolved.PastebinDevKey, resolved.PastebinUserName, resolved.PastebinUserPassword)
		}
	})
}

const (
	runtimeConfigSentinelString = "runtime-config-guard-sentinel"
	runtimeConfigSentinelInt    = int64(424242)
)

// runtimeConfigLeaf identifies one mergeable RuntimeConfig field by dotted name
// (for messages and exception lookup) and by reflect index path (for set/get).
type runtimeConfigLeaf struct {
	name  string
	index []int
}

// runtimeConfigLeaves flattens typ into its mergeable leaf fields, descending only
// into the nested struct types in recurse (Database, WebhookEmbedValidation) so the
// guard checks each concrete merged field rather than a parent struct.
func runtimeConfigLeaves(typ reflect.Type, prefix string, base []int, recurse map[reflect.Type]bool) []runtimeConfigLeaf {
	var leaves []runtimeConfigLeaf
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		index := append(append([]int(nil), base...), i)
		name := prefix + field.Name
		if recurse[field.Type] {
			leaves = append(leaves, runtimeConfigLeaves(field.Type, name+".", index, recurse)...)
			continue
		}
		leaves = append(leaves, runtimeConfigLeaf{name: name, index: index})
	}
	return leaves
}

// setSentinel writes a non-zero, type-appropriate sentinel into the guild-side field.
func (lf runtimeConfigLeaf) setSentinel(t *testing.T, field reflect.Value) {
	t.Helper()
	switch field.Kind() {
	case reflect.String:
		field.SetString(runtimeConfigSentinelString)
	case reflect.Bool:
		field.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(runtimeConfigSentinelInt)
	default:
		t.Fatalf("RuntimeConfig field %s has kind %s with no sentinel rule; merge it in ResolveRuntimeConfig and rely on the default adoption rule, or classify it in the exceptions map", lf.name, field.Kind())
	}
}

// assertAdopted fails when the resolved field did not take the guild sentinel, which
// is the signature of a missing merge line in ResolveRuntimeConfig.
func (lf runtimeConfigLeaf) assertAdopted(t *testing.T, got reflect.Value) {
	t.Helper()
	switch got.Kind() {
	case reflect.String:
		if got.String() != runtimeConfigSentinelString {
			t.Fatalf("ResolveRuntimeConfig dropped the guild override for %s: got %q, want %q; add the merge line or classify the field in the exceptions map", lf.name, got.String(), runtimeConfigSentinelString)
		}
	case reflect.Bool:
		if !got.Bool() {
			t.Fatalf("ResolveRuntimeConfig dropped the guild override for %s: got false, want true; add the merge line or classify the field in the exceptions map", lf.name)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if got.Int() != runtimeConfigSentinelInt {
			t.Fatalf("ResolveRuntimeConfig dropped the guild override for %s: got %d, want %d; add the merge line or classify the field in the exceptions map", lf.name, got.Int(), runtimeConfigSentinelInt)
		}
	default:
		t.Fatalf("RuntimeConfig field %s has kind %s with no sentinel rule", lf.name, got.Kind())
	}
}
