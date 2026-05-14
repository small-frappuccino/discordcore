package files

import "encoding/json"

// Feature toggles are optional overrides for runtime behavior.
// When unset, defaults preserve current behavior.
type FeatureServiceToggles struct {
	Monitoring    *bool `json:"monitoring,omitempty"`
	Automod       *bool `json:"automod,omitempty"`
	Commands      *bool `json:"commands,omitempty"`
	AdminCommands *bool `json:"admin_commands,omitempty"`
}

type FeatureLoggingToggles struct {
	AvatarLogging  *bool `json:"avatar_logging,omitempty"`
	RoleUpdate     *bool `json:"role_update,omitempty"`
	MemberJoin     *bool `json:"member_join,omitempty"`
	MemberLeave    *bool `json:"member_leave,omitempty"`
	MessageProcess *bool `json:"message_process,omitempty"`
	MessageEdit    *bool `json:"message_edit,omitempty"`
	MessageDelete  *bool `json:"message_delete,omitempty"`
	ReactionMetric *bool `json:"reaction_metric,omitempty"`
	AutomodAction  *bool `json:"automod_action,omitempty"`
	ModerationCase *bool `json:"moderation_case,omitempty"`
	CleanAction    *bool `json:"clean_action,omitempty"`
}

type FeatureModerationToggles struct {
	Ban      *bool `json:"ban,omitempty"`
	MassBan  *bool `json:"massban,omitempty"`
	Kick     *bool `json:"kick,omitempty"`
	Timeout  *bool `json:"timeout,omitempty"`
	Warn     *bool `json:"warn,omitempty"`
	Warnings *bool `json:"warnings,omitempty"`
	Clean    *bool `json:"clean,omitempty"`
}

type FeatureMessageCacheToggles struct {
	CleanupOnStartup *bool `json:"cleanup_on_startup,omitempty"`
	DeleteOnLog      *bool `json:"delete_on_log,omitempty"`
}

type FeaturePresenceWatchToggles struct {
	Bot  *bool `json:"bot,omitempty"`
	User *bool `json:"user,omitempty"`
}

type FeatureMaintenanceToggles struct {
	DBCleanup *bool `json:"db_cleanup,omitempty"`
}

type FeatureSafetyToggles struct {
	BotRolePermMirror *bool `json:"bot_role_perm_mirror,omitempty"`
}

type FeatureBackfillToggles struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type FeatureToggles struct {
	Services       FeatureServiceToggles       `json:"services,omitempty"`
	Logging        FeatureLoggingToggles       `json:"logging,omitempty"`
	Moderation     FeatureModerationToggles    `json:"moderation,omitempty"`
	MessageCache   FeatureMessageCacheToggles  `json:"message_cache,omitempty"`
	PresenceWatch  FeaturePresenceWatchToggles `json:"presence_watch,omitempty"`
	Maintenance    FeatureMaintenanceToggles   `json:"maintenance,omitempty"`
	Safety         FeatureSafetyToggles        `json:"safety,omitempty"`
	Backfill       FeatureBackfillToggles      `json:"backfill,omitempty"`
	MuteRole       *bool                       `json:"mute_role,omitempty"`
	StatsChannels  *bool                       `json:"stats_channels,omitempty"`
	AutoRoleAssign *bool                       `json:"auto_role_assignment,omitempty"`
	UserPrune      *bool                       `json:"user_prune,omitempty"`
}

func (ft *FeatureToggles) UnmarshalJSON(data []byte) error {
	type alias FeatureToggles
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*ft = FeatureToggles(parsed)
	return nil
}

type ResolvedFeatureToggles struct {
	Services struct {
		Monitoring    bool
		Automod       bool
		Commands      bool
		AdminCommands bool
	}
	Logging struct {
		AvatarLogging  bool
		RoleUpdate     bool
		MemberJoin     bool
		MemberLeave    bool
		MessageProcess bool
		MessageEdit    bool
		MessageDelete  bool
		ReactionMetric bool
		AutomodAction  bool
		ModerationCase bool
		CleanAction    bool
	}
	Moderation struct {
		Ban      bool
		MassBan  bool
		Kick     bool
		Timeout  bool
		Warn     bool
		Warnings bool
		Clean    bool
	}
	MessageCache struct {
		CleanupOnStartup bool
		DeleteOnLog      bool
	}
	PresenceWatch struct {
		Bot  bool
		User bool
	}
	Maintenance struct {
		DBCleanup bool
	}
	Safety struct {
		BotRolePermMirror bool
	}
	Backfill struct {
		Enabled bool
	}
	MuteRole       bool
	StatsChannels  bool
	AutoRoleAssign bool
	UserPrune      bool
}

func boolPtr(v bool) *bool {
	return &v
}

func resolveFeatureBool(guildVal *bool, globalVal *bool, def bool) bool {
	if guildVal != nil {
		return *guildVal
	}
	if globalVal != nil {
		return *globalVal
	}
	return def
}

// ResolveFeatures merges global and guild feature toggles with defaults.
func (cfg *BotConfig) ResolveFeatures(guildID string) ResolvedFeatureToggles {
	global := FeatureToggles{}
	if cfg != nil {
		global = cfg.Features
	}

	var guild FeatureToggles
	if cfg != nil && guildID != "" {
		for _, g := range cfg.Guilds {
			if g.GuildID == guildID {
				guild = g.Features
				break
			}
		}
	}

	var out ResolvedFeatureToggles
	for _, spec := range featureRegistry {
		guildPtr := guild.LookupToggle(spec.ID)
		globalPtr := global.LookupToggle(spec.ID)
		resolved := resolveFeatureBool(guildPtr, globalPtr, spec.Default)
		resolvedToggleValue(&out, spec.Path).SetBool(resolved)
	}
	return out
}
