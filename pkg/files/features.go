package files

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

// FeatureServiceToggles holds optional overrides for runtime behavior.
// When unset, defaults preserve current behavior.
type FeatureServiceToggles struct {
	Monitoring *bool `json:"monitoring,omitempty"`
	Automod    *bool `json:"automod,omitempty"`
	Commands   *bool `json:"commands,omitempty"`
}

// FeatureLoggingToggles overrides individual log-event categories. A nil field
// leaves that category at its default; false disables emitting that event.
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

// FeatureModerationToggles enables or disables individual moderation commands.
// A nil field leaves that command at its default availability.
type FeatureModerationToggles struct {
	Ban      *bool `json:"ban,omitempty"`
	MassBan  *bool `json:"massban,omitempty"`
	Kick     *bool `json:"kick,omitempty"`
	Timeout  *bool `json:"timeout,omitempty"`
	Warn     *bool `json:"warn,omitempty"`
	Warnings *bool `json:"warnings,omitempty"`
	Clean    *bool `json:"clean,omitempty"`
}

// FeatureMessageCacheToggles controls message-cache maintenance behavior. A nil
// field leaves that behavior at its default.
type FeatureMessageCacheToggles struct {
	CleanupOnStartup *bool `json:"cleanup_on_startup,omitempty"`
	DeleteOnLog      *bool `json:"delete_on_log,omitempty"`
}

// FeaturePresenceWatchToggles selects which presences are watched. A nil field
// leaves that target at its default.
type FeaturePresenceWatchToggles struct {
	Bot  *bool `json:"bot,omitempty"`
	User *bool `json:"user,omitempty"`
}

// FeatureMaintenanceToggles controls background maintenance jobs. A nil field
// leaves the job at its default.
type FeatureMaintenanceToggles struct {
	DBCleanup *bool `json:"db_cleanup,omitempty"`
}

// FeatureSafetyToggles controls safety mechanisms such as mirroring bot role
// permissions. A nil field leaves the mechanism at its default.
type FeatureSafetyToggles struct {
	BotRolePermMirror *bool `json:"bot_role_perm_mirror,omitempty"`
}

// FeatureBackfillToggles controls the historical backfill subsystem. A nil
// Enabled leaves backfill at its default.
type FeatureBackfillToggles struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// FeatureToggles is the per-guild override surface for optional behavior,
// grouped by domain. Pointer fields are tri-state: nil means inherit the
// default, while a non-nil value forces the feature on or off. Resolve to
// concrete booleans via ResolvedFeatureToggles.
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
	RolePanels     *bool                       `json:"role_panels,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (ft *FeatureToggles) UnmarshalJSON(data []byte) error {
	type alias FeatureToggles
	var parsed alias

	slog.Debug("Inspeção granular: Iniciando extração de payload dinâmico para FeatureToggles",
		slog.Int("payload_bytes", len(data)),
	)

	if err := json.Unmarshal(data, &parsed); err != nil {
		errWrap := fmt.Errorf("FeatureToggles.UnmarshalJSON: %w", err)
		log.EmitBlockingError("Falha estrutural bloqueante restrita ao escopo de desserialização do payload I/O", errWrap, log.GenerateRequestID())
		return errWrap
	}
	*ft = FeatureToggles(parsed)
	return nil
}

// ResolvedFeatureToggles is FeatureToggles with every tri-state pointer
// collapsed to a concrete boolean by applying defaults. It is the form consumed
// by runtime code that must not deal with nil-means-default semantics.
type ResolvedFeatureToggles struct {
	Services struct {
		Monitoring bool
		Automod    bool
		Commands   bool
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
	RolePanels     bool
}

func boolPtr(v bool) *bool {
	return &v
}

func resolveFeatureBool(guildVal *bool, globalVal *bool, def bool) bool {
	if guildVal != nil {
		slog.Debug("Rastreamento de ramificação condicional: Adoção de estado transiente sobrescrito pela guilda",
			slog.Bool("resolved_value", *guildVal),
		)
		return *guildVal
	}
	if globalVal != nil {
		slog.Debug("Rastreamento de ramificação condicional: Adoção de estado transiente em nível global",
			slog.Bool("resolved_value", *globalVal),
		)
		return *globalVal
	}
	slog.Debug("Rastreamento de ramificação condicional: Recuo estrutural para valor padrão basal",
		slog.Bool("resolved_value", def),
	)
	return def
}

// ResolveFeatures merges global and guild feature toggles with defaults.
func (cfg *BotConfig) ResolveFeatures(guildID string) ResolvedFeatureToggles {
	global := FeatureToggles{}
	if cfg != nil {
		global = cfg.Features
	} else {
		slog.Warn("Degradação mitigada interceptada: Objeto BotConfig nulo durante resolução; o fluxo compensatório adotará um vetor global vazio",
			slog.String("guild_id", guildID),
		)
	}

	var guild FeatureToggles
	guildFound := false
	if cfg != nil && guildID != "" {
		for _, g := range cfg.Guilds {
			if g.GuildID == guildID {
				guild = g.Features
				guildFound = true
				break
			}
		}
	}

	if cfg != nil && guildID != "" && !guildFound {
		slog.Debug("Inspeção granular: Nenhuma árvore de características customizada localizada para a guilda; ramificação dependente da herança global",
			slog.String("guild_id", guildID),
		)
	}

	var out ResolvedFeatureToggles
	for _, spec := range featureRegistry {
		guildPtr := guild.LookupToggle(spec.ID)
		globalPtr := global.LookupToggle(spec.ID)
		resolved := resolveFeatureBool(guildPtr, globalPtr, spec.Default)
		spec.SetResolved(&out, resolved)
	}

	slog.Debug("Transição de sub-estado: Vetor hierárquico FeatureToggles consolidado",
		slog.String("guild_id", guildID),
	)

	return out
}
