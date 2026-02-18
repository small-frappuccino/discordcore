package logging

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestShouldRunMemberEventService(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	tests := []struct {
		name     string
		cfg      *files.BotConfig
		globalRC files.RuntimeConfig
		want     bool
	}{
		{
			name: "nil config",
			cfg:  nil,
			want: false,
		},
		{
			name: "global entry exit enabled",
			cfg: &files.BotConfig{
				Features: files.FeatureToggles{
					Logging: files.FeatureLoggingToggles{
						MemberJoin: boolPtr(true),
					},
				},
			},
			want: true,
		},
		{
			name: "global entry exit disabled and no auto role",
			cfg: &files.BotConfig{
				Features: files.FeatureToggles{
					Logging: files.FeatureLoggingToggles{
						MemberJoin:  boolPtr(false),
						MemberLeave: boolPtr(false),
					},
					AutoRoleAssign: boolPtr(false),
				},
				Guilds: []files.GuildConfig{
					{
						GuildID: "1",
						Roles: files.RolesConfig{
							AutoAssignment: files.AutoAssignmentConfig{Enabled: false},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "auto role enabled by guild even with entry exit disabled",
			cfg: &files.BotConfig{
				Features: files.FeatureToggles{
					Logging: files.FeatureLoggingToggles{
						MemberJoin:  boolPtr(false),
						MemberLeave: boolPtr(false),
					},
					AutoRoleAssign: boolPtr(false),
				},
				Guilds: []files.GuildConfig{
					{
						GuildID: "1",
						Features: files.FeatureToggles{
							AutoRoleAssign: boolPtr(true),
						},
						Roles: files.RolesConfig{
							AutoAssignment: files.AutoAssignmentConfig{Enabled: true},
						},
					},
				},
			},
			globalRC: files.RuntimeConfig{DisableEntryExitLogs: true},
			want:     true,
		},
		{
			name: "auto role disabled by guild feature",
			cfg: &files.BotConfig{
				Features: files.FeatureToggles{
					Logging: files.FeatureLoggingToggles{
						MemberJoin:  boolPtr(false),
						MemberLeave: boolPtr(false),
					},
					AutoRoleAssign: boolPtr(false),
				},
				Guilds: []files.GuildConfig{
					{
						GuildID: "1",
						Features: files.FeatureToggles{
							AutoRoleAssign: boolPtr(false),
						},
						Roles: files.RolesConfig{
							AutoAssignment: files.AutoAssignmentConfig{Enabled: true},
						},
					},
				},
			},
			globalRC: files.RuntimeConfig{DisableEntryExitLogs: true},
			want:     false,
		},
		{
			name: "guild entry exit override enabled while global disabled",
			cfg: &files.BotConfig{
				Features: files.FeatureToggles{
					Logging: files.FeatureLoggingToggles{
						MemberJoin:  boolPtr(false),
						MemberLeave: boolPtr(false),
					},
				},
				Guilds: []files.GuildConfig{
					{
						GuildID: "1",
						Features: files.FeatureToggles{
							Logging: files.FeatureLoggingToggles{
								MemberJoin: boolPtr(true),
							},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRunMemberEventService(tt.cfg, tt.globalRC)
			if got != tt.want {
				t.Fatalf("shouldRunMemberEventService()=%v, want %v", got, tt.want)
			}
		})
	}
}
