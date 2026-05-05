package config

import (
	"reflect"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestQOTDSmokeTestLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config files.QOTDConfig
		want   []string
	}{
		{
			name:   "missing channel and schedule",
			config: smokeTestQOTDConfig(false, "", files.QOTDPublishScheduleConfig{}),
			want: []string{
				"[PASS] Active QOTD deck: Default.",
				"[ACTION] QOTD currently follows the guild-wide/default bot binding. If you expect a separate QOTD bot, set Bot Routing -> QOTD in Control Panel first.",
				"[ACTION] QOTD channel is not configured. Run /config qotd_channel <channel>.",
				"[ACTION] QOTD publish schedule is not complete (— UTC). Run /config qotd_schedule <hour> <minute>.",
				"[ACTION] QOTD is not ready to enable yet. Set the QOTD channel and schedule first.",
			},
		},
		{
			name:   "channel only",
			config: smokeTestQOTDConfig(false, "qotd-123", files.QOTDPublishScheduleConfig{}),
			want: []string{
				"[PASS] Active QOTD deck: Default.",
				"[ACTION] QOTD currently follows the guild-wide/default bot binding. If you expect a separate QOTD bot, set Bot Routing -> QOTD in Control Panel first.",
				"[PASS] QOTD channel configured: <#qotd-123>.",
				"[ACTION] QOTD publish schedule is not complete (— UTC). Run /config qotd_schedule <hour> <minute>.",
				"[ACTION] QOTD is not ready to enable yet. Set the QOTD publish hour and minute first.",
			},
		},
		{
			name:   "schedule only",
			config: smokeTestQOTDConfig(false, "", testCommandSchedule()),
			want: []string{
				"[PASS] Active QOTD deck: Default.",
				"[ACTION] QOTD currently follows the guild-wide/default bot binding. If you expect a separate QOTD bot, set Bot Routing -> QOTD in Control Panel first.",
				"[ACTION] QOTD channel is not configured. Run /config qotd_channel <channel>.",
				"[PASS] QOTD publish schedule configured: 12:43 UTC.",
				"[ACTION] QOTD is not ready to enable yet. Set the QOTD channel first.",
			},
		},
		{
			name:   "ready to enable",
			config: smokeTestQOTDConfig(false, "qotd-123", testCommandSchedule()),
			want: []string{
				"[PASS] Active QOTD deck: Default.",
				"[ACTION] QOTD currently follows the guild-wide/default bot binding. If you expect a separate QOTD bot, set Bot Routing -> QOTD in Control Panel first.",
				"[PASS] QOTD channel configured: <#qotd-123>.",
				"[PASS] QOTD publish schedule configured: 12:43 UTC.",
				"[ACTION] QOTD is ready to enable. Run /config qotd_enabled true.",
			},
		},
		{
			name:   "enabled and complete",
			config: smokeTestQOTDConfig(true, "qotd-123", testCommandSchedule()),
			want: []string{
				"[PASS] Active QOTD deck: Default.",
				"[ACTION] QOTD currently follows the guild-wide/default bot binding. If you expect a separate QOTD bot, set Bot Routing -> QOTD in Control Panel first.",
				"[PASS] QOTD channel configured: <#qotd-123>.",
				"[PASS] QOTD publish schedule configured: 12:43 UTC.",
				"[PASS] QOTD publishing is enabled for deck Default.",
			},
		},
		{
			name:   "dedicated routing override configured",
			config: smokeTestQOTDConfig(false, "qotd-123", testCommandSchedule()),
			want: []string{
				"[PASS] Active QOTD deck: Default.",
				"[PASS] QOTD domain routing override is configured.",
				"[PASS] QOTD channel configured: <#qotd-123>.",
				"[PASS] QOTD publish schedule configured: 12:43 UTC.",
				"[ACTION] QOTD is ready to enable. Run /config qotd_enabled true.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guildConfig := files.GuildConfig{
				GuildID: "guild-1",
				QOTD:    tt.config,
			}
			if tt.name == "dedicated routing override configured" {
				guildConfig.DomainBotInstanceIDs = map[string]string{files.BotDomainQOTD: "companion"}
			}
			ctx := newSmokeTestContext(t, guildConfig)

			if got := qotdSmokeTestLines(ctx); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("qotdSmokeTestLines() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGeneralSmokeTestLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		commandsEnabled bool
		commandChannel  string
		allowedRoles    []string
		want            []string
	}{
		{
			name:            "guild dormant",
			commandsEnabled: false,
			want: []string{
				"[PASS] /config list remains available while this guild is still dormant.",
				"[ACTION] Command channel is not configured. Run /config command_channel <channel>.",
				"[ACTION] Full slash command surface is still disabled. Run /config commands_enabled true when bootstrap setup is complete.",
				"[PASS] Non-bootstrap routes remain blocked until /config commands_enabled true.",
				"[INFO] No allowed admin roles are configured. Guild owner / Administrator / Manage Guild can still bootstrap this guild.",
			},
		},
		{
			name:            "command channel absent while commands enabled",
			commandsEnabled: true,
			want: []string{
				"[PASS] /config list is in the bootstrap allowlist, and the full slash surface is already enabled.",
				"[ACTION] Command channel is not configured. Run /config command_channel <channel>.",
				"[PASS] Full slash command surface is enabled.",
				"[PASS] Non-bootstrap routes are no longer gated because commands are enabled.",
				"[INFO] No allowed admin roles are configured. Guild owner / Administrator / Manage Guild can still bootstrap this guild.",
			},
		},
		{
			name:            "commands enabled with command channel",
			commandsEnabled: true,
			commandChannel:  "command-123",
			want: []string{
				"[PASS] /config list is in the bootstrap allowlist, and the full slash surface is already enabled.",
				"[PASS] Command channel configured: <#command-123>.",
				"[PASS] Full slash command surface is enabled.",
				"[PASS] Non-bootstrap routes are no longer gated because commands are enabled.",
				"[INFO] No allowed admin roles are configured. Guild owner / Administrator / Manage Guild can still bootstrap this guild.",
			},
		},
		{
			name:            "roles configured",
			commandsEnabled: false,
			commandChannel:  "command-123",
			allowedRoles:    []string{"role-1", "role-2"},
			want: []string{
				"[PASS] /config list remains available while this guild is still dormant.",
				"[PASS] Command channel configured: <#command-123>.",
				"[ACTION] Full slash command surface is still disabled. Run /config commands_enabled true when bootstrap setup is complete.",
				"[PASS] Non-bootstrap routes remain blocked until /config commands_enabled true.",
				"[PASS] Allowed admin roles configured: 2.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newSmokeTestContext(t, files.GuildConfig{
				GuildID: "guild-1",
				Features: files.FeatureToggles{
						Services: files.FeatureServiceToggles{Commands: testBoolPtr(tt.commandsEnabled)},
				},
				Channels: files.ChannelsConfig{Commands: tt.commandChannel},
				Roles:    files.RolesConfig{Allowed: append([]string(nil), tt.allowedRoles...)},
			})

			if got := generalSmokeTestLines(ctx); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("generalSmokeTestLines() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func newSmokeTestContext(t *testing.T, guildConfig files.GuildConfig) *core.Context {
	t.Helper()

	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(guildConfig); err != nil {
		t.Fatalf("AddGuildConfig() failed: %v", err)
	}

	guildCopy := guildConfig
	return &core.Context{
		Config:      cm,
		GuildID:     guildConfig.GuildID,
		GuildConfig: &guildCopy,
	}
}

func smokeTestQOTDConfig(enabled bool, channelID string, schedule files.QOTDPublishScheduleConfig) files.QOTDConfig {
	return files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule:     schedule,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   enabled,
			ChannelID: channelID,
		}},
	}
}

func testBoolPtr(value bool) *bool {
	return &value
}