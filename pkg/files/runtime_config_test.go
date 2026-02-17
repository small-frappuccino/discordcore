package files

import "testing"

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
