package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// RuntimeConfig centralizes operational toggles/parameters that were previously
// controlled via environment variables. These values are meant to be edited
// from Discord via an interactive embed and persisted in settings.json.
//
// Keep names in CAPS to mirror the previous env var names and make auditing easy.
type RuntimeConfig struct {
	// THEME
	BotTheme string `json:"bot_theme,omitempty"`

	// SERVICES (LOGGING)
	DisableDBCleanup     bool `json:"disable_db_cleanup,omitempty"`
	DisableAutomodLogs   bool `json:"disable_automod_logs,omitempty"`
	DisableMessageLogs   bool `json:"disable_message_logs,omitempty"`
	DisableEntryExitLogs bool `json:"disable_entry_exit_logs,omitempty"`
	DisableReactionLogs  bool `json:"disable_reaction_logs,omitempty"`
	DisableUserLogs      bool `json:"disable_user_logs,omitempty"`
	DisableCleanLog      bool `json:"disable_clean_log,omitempty"`
	// MODERATION LOGS
	// true/nil: send moderation logs automatically
	// false: do not send moderation logs
	ModerationLogging *bool `json:"moderation_logging,omitempty"`
	// Deprecated: legacy mode key kept for backward compatibility with older settings.
	// New behavior ignores mode semantics and only uses moderation_logging true/false.
	ModerationLogMode string `json:"moderation_log_mode,omitempty"`

	// PRESENCE WATCH
	PresenceWatchUserID string `json:"presence_watch_user_id,omitempty"`
	PresenceWatchBot    bool   `json:"presence_watch_bot,omitempty"`

	// MESSAGE CACHE
	MessageCacheTTLHours int  `json:"message_cache_ttl_hours,omitempty"`
	MessageDeleteOnLog   bool `json:"message_delete_on_log,omitempty"`
	MessageCacheCleanup  bool `json:"message_cache_cleanup,omitempty"`

	// BACKFILL (ENTRY/EXIT)
	BackfillChannelID   string `json:"backfill_channel_id,omitempty"`
	BackfillStartDay    string `json:"backfill_start_day,omitempty"` // YYYY-MM-DD, default: today UTC when empty
	BackfillInitialDate string `json:"backfill_initial_date,omitempty"`

	// BOT ROLE PERMISSION MIRRORING (SAFETY)
	// Previously controllable via env vars:
	//   - ALICE_DISABLE_BOT_ROLE_PERM_MIRROR
	//   - ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID
	DisableBotRolePermMirror     bool   `json:"disable_bot_role_perm_mirror,omitempty"`
	BotRolePermMirrorActorRoleID string `json:"bot_role_perm_mirror_actor_role_id,omitempty"`

	// Webhook embed message patch (global or per-guild override).
	// Intended for editing an existing webhook message embed by ID.
	WebhookEmbedUpdates []WebhookEmbedUpdateConfig `json:"webhook_embed_updates,omitempty"`
	// Deprecated: single-item legacy key kept for backward compatibility.
	WebhookEmbedUpdate WebhookEmbedUpdateConfig `json:"webhook_embed_update,omitempty"`
	// Remote validation behavior for webhook embed targets used by CRUD commands.
	WebhookEmbedValidation WebhookEmbedValidationConfig `json:"webhook_embed_validation,omitempty"`
}

// WebhookEmbedUpdateConfig defines how to patch an existing webhook message embed.
type WebhookEmbedUpdateConfig struct {
	MessageID  string          `json:"message_id,omitempty"`
	WebhookURL string          `json:"webhook_url,omitempty"`
	Embed      json.RawMessage `json:"embed,omitempty"`
}

const (
	WebhookEmbedValidationModeOff    = "off"
	WebhookEmbedValidationModeSoft   = "soft"
	WebhookEmbedValidationModeStrict = "strict"

	DefaultWebhookEmbedValidationTimeoutMS = 3000
)

// WebhookEmbedValidationConfig controls remote validation behavior for webhook targets.
// mode:
// - off: no remote validation
// - soft: validate remotely, but persist even on failure
// - strict: validate remotely and fail before persisting when validation fails
type WebhookEmbedValidationConfig struct {
	Mode      string `json:"mode,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

// Normalized returns a canonical config with safe defaults.
func (c WebhookEmbedValidationConfig) Normalized() WebhookEmbedValidationConfig {
	mode := normalizeWebhookEmbedValidationMode(c.Mode)
	timeout := c.TimeoutMS
	if timeout <= 0 {
		timeout = DefaultWebhookEmbedValidationTimeoutMS
	}
	return WebhookEmbedValidationConfig{
		Mode:      mode,
		TimeoutMS: timeout,
	}
}

func normalizeWebhookEmbedValidationMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case WebhookEmbedValidationModeSoft:
		return WebhookEmbedValidationModeSoft
	case WebhookEmbedValidationModeStrict:
		return WebhookEmbedValidationModeStrict
	default:
		return WebhookEmbedValidationModeOff
	}
}

// IsZero reports whether all fields are unset.
func (c WebhookEmbedUpdateConfig) IsZero() bool {
	return strings.TrimSpace(c.MessageID) == "" &&
		strings.TrimSpace(c.WebhookURL) == "" &&
		len(bytes.TrimSpace(c.Embed)) == 0
}

// NormalizedWebhookEmbedUpdates returns a normalized list:
// - prefers webhook_embed_updates when any non-empty entries exist
// - falls back to legacy webhook_embed_update when list is empty
func (rc RuntimeConfig) NormalizedWebhookEmbedUpdates() []WebhookEmbedUpdateConfig {
	updates := make([]WebhookEmbedUpdateConfig, 0, len(rc.WebhookEmbedUpdates))
	for _, item := range rc.WebhookEmbedUpdates {
		if item.IsZero() {
			continue
		}
		updates = append(updates, item)
	}
	if len(updates) > 0 {
		return updates
	}
	if !rc.WebhookEmbedUpdate.IsZero() {
		return []WebhookEmbedUpdateConfig{rc.WebhookEmbedUpdate}
	}
	return nil
}

// EffectiveWebhookEmbedValidation resolves webhook_embed_validation defaults.
func (rc RuntimeConfig) EffectiveWebhookEmbedValidation() WebhookEmbedValidationConfig {
	return rc.WebhookEmbedValidation.Normalized()
}

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
	MessageCache   FeatureMessageCacheToggles  `json:"message_cache,omitempty"`
	PresenceWatch  FeaturePresenceWatchToggles `json:"presence_watch,omitempty"`
	Maintenance    FeatureMaintenanceToggles   `json:"maintenance,omitempty"`
	Safety         FeatureSafetyToggles        `json:"safety,omitempty"`
	Backfill       FeatureBackfillToggles      `json:"backfill,omitempty"`
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
	StatsChannels  bool
	AutoRoleAssign bool
	UserPrune      bool
}

// ## Config Types

// ChannelsConfig groups channel IDs per guild.
type ChannelsConfig struct {
	Commands string `json:"commands,omitempty"`

	// Event/feature-scoped channels (canonical settings schema).
	AvatarLogging       string `json:"avatar_logging,omitempty"`
	RoleUpdate          string `json:"role_update,omitempty"`
	MemberJoin          string `json:"member_join,omitempty"`
	MemberLeave         string `json:"member_leave,omitempty"`
	MessageEdit         string `json:"message_edit,omitempty"`
	MessageDelete       string `json:"message_delete,omitempty"`
	AutomodAction       string `json:"automod_action,omitempty"`
	ModerationCase      string `json:"moderation_case,omitempty"`
	CleanAction         string `json:"clean_action,omitempty"`
	EntryBackfill       string `json:"entry_backfill,omitempty"`
	VerificationCleanup string `json:"verification_cleanup,omitempty"`
}

func (cc ChannelsConfig) BackfillChannelID() string {
	return strings.TrimSpace(cc.EntryBackfill)
}

func (cc ChannelsConfig) VerificationCleanupChannelID() string {
	return strings.TrimSpace(cc.VerificationCleanup)
}

// StatsChannelConfig defines a channel that should reflect a member count.
type StatsChannelConfig struct {
	ChannelID    string `json:"channel_id,omitempty"`
	Label        string `json:"label,omitempty"`
	NameTemplate string `json:"name_template,omitempty"` // Supports {label} and {count}
	MemberType   string `json:"member_type,omitempty"`   // all|humans|bots (default: all)
	RoleID       string `json:"role_id,omitempty"`       // Optional role filter
}

// StatsConfig groups the periodic stats channel updates for a guild.
type StatsConfig struct {
	Enabled            bool                 `json:"enabled,omitempty"`
	UpdateIntervalMins int                  `json:"update_interval_mins,omitempty"` // default: 30
	Channels           []StatsChannelConfig `json:"channels,omitempty"`
}

// AutoAssignmentConfig defines automatic role assignment rules.
type AutoAssignmentConfig struct {
	Enabled       bool     `json:"enabled,omitempty"`
	TargetRoleID  string   `json:"target_role,omitempty"`
	RequiredRoles []string `json:"required_roles,omitempty"`
}

// RolesConfig groups role-related settings per guild.
type RolesConfig struct {
	Allowed          []string             `json:"allowed,omitempty"`
	AutoAssignment   AutoAssignmentConfig `json:"auto_assignment,omitempty"`
	VerificationRole string               `json:"verification_role,omitempty"`
	BoosterRole      string               `json:"booster_role,omitempty"`
}

// UserPruneConfig controls periodic user pruning per guild.
type UserPruneConfig struct {
	// Enabled toggles the automatic monthly prune.
	// true: execute native Discord prune automatically on day 28 (30-day inactivity window)
	// false: do not execute automatically
	Enabled bool `json:"enabled,omitempty"`

	// Deprecated: ignored. Kept only for backward compatibility with older settings files.
	GraceDays int `json:"grace_days,omitempty"`
	// Deprecated: ignored. Kept only for backward compatibility with older settings files.
	ScanIntervalMins int `json:"scan_interval_mins,omitempty"`
	// Deprecated: ignored. Kept only for backward compatibility with older settings files.
	InitialDelaySecs int `json:"initial_delay_secs,omitempty"`
	// Deprecated: ignored. Kept only for backward compatibility with older settings files.
	KicksPerSecond int `json:"kps,omitempty"`
	// Deprecated: ignored. Kept only for backward compatibility with older settings files.
	MaxKicksPerRun int `json:"max_kicks_per_run,omitempty"`
	// Deprecated: ignored. Kept only for backward compatibility with older settings files.
	ExemptRoleIDs []string `json:"exempt_role_ids,omitempty"`
	// Deprecated: ignored. Kept only for backward compatibility with older settings files.
	DryRun bool `json:"dry_run,omitempty"`
}

// GuildConfig holds the configuration for a specific guild.
type GuildConfig struct {
	GuildID    string         `json:"guild_id"`
	Features   FeatureToggles `json:"features,omitempty"`
	Channels   ChannelsConfig `json:"channels,omitempty"`
	Roles      RolesConfig    `json:"roles,omitempty"`
	Stats      StatsConfig    `json:"stats,omitempty"`
	Rulesets   []Ruleset      `json:"rulesets,omitempty"`
	LooseLists []Rule         `json:"loose_rules,omitempty"` // Loose rules not associated with any ruleset
	Blocklist  []string       `json:"blocklist,omitempty"`

	// Cache TTL configuration (per-guild tuning)
	RolesCacheTTL   string `json:"roles_cache_ttl,omitempty"`   // e.g.: "5m", "1h" (default: "5m")
	MemberCacheTTL  string `json:"member_cache_ttl,omitempty"`  // e.g.: "5m", "10m" (default: "5m")
	GuildCacheTTL   string `json:"guild_cache_ttl,omitempty"`   // e.g.: "15m", "30m" (default: "15m")
	ChannelCacheTTL string `json:"channel_cache_ttl,omitempty"` // e.g.: "15m", "30m" (default: "15m")

	UserPrune UserPruneConfig `json:"user_prune,omitempty"`

	// RuntimeConfig allows per-guild overrides for certain settings.
	RuntimeConfig RuntimeConfig `json:"runtime_config,omitempty"`
}

func (gc *GuildConfig) UnmarshalJSON(data []byte) error {
	type alias GuildConfig
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*gc = GuildConfig(parsed)
	return nil
}

// BotConfig holds the configuration for the bot.
type BotConfig struct {
	Guilds []GuildConfig `json:"guilds"`

	// Features holds optional toggles for runtime behavior overrides.
	Features FeatureToggles `json:"features,omitempty"`

	// RuntimeConfig holds bot-level runtime overrides editable from Discord.
	// This intentionally replaces the previous "env var toggles" for operational
	// behavior (except for token), so settings can be managed in-app.
	//
	// NOTE: These are NOT environment variables. They are persisted in settings.json.
	RuntimeConfig RuntimeConfig `json:"runtime_config,omitempty"`
}

// CustomRPCConfig holds profiles for local Discord Rich Presence.
type CustomRPCConfig struct {
	DefaultProfile string             `json:"default_profile,omitempty"`
	UserProfiles   map[string]string  `json:"user_profiles,omitempty"`
	Profiles       []CustomRPCProfile `json:"profiles,omitempty"`
}

// CustomRPCProfile defines a single Rich Presence profile.
type CustomRPCProfile struct {
	Name                  string             `json:"name"`
	Disabled              bool               `json:"disabled,omitempty"`
	User                  string             `json:"user,omitempty"`
	ApplicationID         string             `json:"application_id"`
	Type                  string             `json:"type,omitempty"`
	URL                   string             `json:"url,omitempty"`
	Details               string             `json:"details,omitempty"`
	State                 string             `json:"state,omitempty"`
	Party                 RPCPartyConfig     `json:"party,omitempty"`
	Timestamp             RPCTimestampConfig `json:"timestamp,omitempty"`
	Assets                RPCAssetsConfig    `json:"assets,omitempty"`
	Buttons               []RPCButtonConfig  `json:"buttons,omitempty"`
	UpdateIntervalSeconds int                `json:"update_interval_seconds,omitempty"`
}

// RPCPartyConfig controls the optional party size display.
type RPCPartyConfig struct {
	ID      string `json:"id,omitempty"`
	Current int    `json:"current,omitempty"`
	Max     int    `json:"max,omitempty"`
}

// RPCTimestampConfig controls timestamp behavior.
type RPCTimestampConfig struct {
	Mode      string `json:"mode,omitempty"`
	StartUnix int64  `json:"start_unix,omitempty"`
	EndUnix   int64  `json:"end_unix,omitempty"`
	Start     string `json:"start,omitempty"`
	End       string `json:"end,omitempty"`
}

// RPCAssetsConfig controls asset keys and hover text.
type RPCAssetsConfig struct {
	LargeImageKey string `json:"large_image_key,omitempty"`
	LargeText     string `json:"large_text,omitempty"`
	SmallImageKey string `json:"small_image_key,omitempty"`
	SmallText     string `json:"small_text,omitempty"`
}

// RPCButtonConfig defines a label + URL button.
type RPCButtonConfig struct {
	Label string `json:"label,omitempty"`
	URL   string `json:"url,omitempty"`
}

// ResolveRuntimeConfig retorna a configuração de runtime para uma guilda,
// caindo para o global se o campo não estiver definido (zero-value).
func (cfg *BotConfig) ResolveRuntimeConfig(guildID string) RuntimeConfig {
	global := cfg.RuntimeConfig
	if global.ModerationLogging == nil {
		global.ModerationLogging = boolPtr(global.ModerationLoggingEnabled())
	}
	if guildID == "" {
		return global
	}

	var guildRC RuntimeConfig
	found := false
	for _, g := range cfg.Guilds {
		if g.GuildID == guildID {
			guildRC = g.RuntimeConfig
			found = true
			break
		}
	}

	if !found {
		return global
	}

	// Manual merging logic. Fields that are zero-value in guildRC will use global values.
	// This is better than a generic library for such a small struct and specific rules.
	resolved := global

	if guildRC.BotTheme != "" {
		resolved.BotTheme = guildRC.BotTheme
	}

	if guildRC.DisableDBCleanup {
		resolved.DisableDBCleanup = true
	}
	if guildRC.DisableAutomodLogs {
		resolved.DisableAutomodLogs = true
	}
	if guildRC.DisableMessageLogs {
		resolved.DisableMessageLogs = true
	}
	if guildRC.DisableEntryExitLogs {
		resolved.DisableEntryExitLogs = true
	}
	if guildRC.DisableReactionLogs {
		resolved.DisableReactionLogs = true
	}
	if guildRC.DisableUserLogs {
		resolved.DisableUserLogs = true
	}
	if guildRC.DisableCleanLog {
		resolved.DisableCleanLog = true
	}
	if guildRC.ModerationLogging != nil {
		resolved.ModerationLogging = boolPtr(*guildRC.ModerationLogging)
	} else if guildRC.ModerationLogMode != "" {
		resolved.ModerationLogging = boolPtr(guildRC.ModerationLoggingEnabled())
	}
	if guildRC.PresenceWatchUserID != "" {
		resolved.PresenceWatchUserID = guildRC.PresenceWatchUserID
	}
	if guildRC.PresenceWatchBot {
		resolved.PresenceWatchBot = true
	}

	if guildRC.MessageCacheTTLHours != 0 {
		resolved.MessageCacheTTLHours = guildRC.MessageCacheTTLHours
	}
	if guildRC.MessageDeleteOnLog {
		resolved.MessageDeleteOnLog = true
	}
	if guildRC.MessageCacheCleanup {
		resolved.MessageCacheCleanup = true
	}

	if guildRC.BackfillChannelID != "" {
		resolved.BackfillChannelID = guildRC.BackfillChannelID
	}
	if guildRC.BackfillStartDay != "" {
		resolved.BackfillStartDay = guildRC.BackfillStartDay
	}

	// BackfillInitialDate is GuildOnly: it must be set in the guild config
	// and does not fall back to the global config.
	resolved.BackfillInitialDate = guildRC.BackfillInitialDate

	if guildRC.DisableBotRolePermMirror {
		resolved.DisableBotRolePermMirror = true
	}
	if guildRC.BotRolePermMirrorActorRoleID != "" {
		resolved.BotRolePermMirrorActorRoleID = guildRC.BotRolePermMirrorActorRoleID
	}
	if mode := strings.TrimSpace(guildRC.WebhookEmbedValidation.Mode); mode != "" {
		resolved.WebhookEmbedValidation.Mode = mode
	}
	if guildRC.WebhookEmbedValidation.TimeoutMS > 0 {
		resolved.WebhookEmbedValidation.TimeoutMS = guildRC.WebhookEmbedValidation.TimeoutMS
	}
	if guildUpdates := guildRC.NormalizedWebhookEmbedUpdates(); len(guildUpdates) > 0 {
		resolved.WebhookEmbedUpdates = append([]WebhookEmbedUpdateConfig(nil), guildUpdates...)
		resolved.WebhookEmbedUpdate = WebhookEmbedUpdateConfig{}
	}

	return resolved
}

// ModerationLoggingEnabled resolves whether moderation logs should be sent.
// New key: runtime_config.moderation_logging (true/false).
// Backward compatibility: when unset, moderation_log_mode="off" disables logs; any other value enables.
func (rc RuntimeConfig) ModerationLoggingEnabled() bool {
	if rc.ModerationLogging != nil {
		return *rc.ModerationLogging
	}
	return strings.ToLower(strings.TrimSpace(rc.ModerationLogMode)) != "off"
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
	out.Services.Monitoring = resolveFeatureBool(guild.Services.Monitoring, global.Services.Monitoring, true)
	out.Services.Automod = resolveFeatureBool(guild.Services.Automod, global.Services.Automod, true)
	out.Services.Commands = resolveFeatureBool(guild.Services.Commands, global.Services.Commands, true)
	out.Services.AdminCommands = resolveFeatureBool(guild.Services.AdminCommands, global.Services.AdminCommands, true)

	out.Logging.AvatarLogging = resolveFeatureBool(guild.Logging.AvatarLogging, global.Logging.AvatarLogging, true)
	out.Logging.RoleUpdate = resolveFeatureBool(guild.Logging.RoleUpdate, global.Logging.RoleUpdate, true)
	out.Logging.MemberJoin = resolveFeatureBool(guild.Logging.MemberJoin, global.Logging.MemberJoin, true)
	out.Logging.MemberLeave = resolveFeatureBool(guild.Logging.MemberLeave, global.Logging.MemberLeave, true)
	out.Logging.MessageProcess = resolveFeatureBool(guild.Logging.MessageProcess, global.Logging.MessageProcess, true)
	out.Logging.MessageEdit = resolveFeatureBool(guild.Logging.MessageEdit, global.Logging.MessageEdit, true)
	out.Logging.MessageDelete = resolveFeatureBool(guild.Logging.MessageDelete, global.Logging.MessageDelete, true)
	out.Logging.ReactionMetric = resolveFeatureBool(guild.Logging.ReactionMetric, global.Logging.ReactionMetric, true)
	out.Logging.AutomodAction = resolveFeatureBool(guild.Logging.AutomodAction, global.Logging.AutomodAction, true)
	out.Logging.ModerationCase = resolveFeatureBool(guild.Logging.ModerationCase, global.Logging.ModerationCase, true)
	out.Logging.CleanAction = resolveFeatureBool(guild.Logging.CleanAction, global.Logging.CleanAction, true)

	out.MessageCache.CleanupOnStartup = resolveFeatureBool(guild.MessageCache.CleanupOnStartup, global.MessageCache.CleanupOnStartup, false)
	out.MessageCache.DeleteOnLog = resolveFeatureBool(guild.MessageCache.DeleteOnLog, global.MessageCache.DeleteOnLog, false)

	out.PresenceWatch.Bot = resolveFeatureBool(guild.PresenceWatch.Bot, global.PresenceWatch.Bot, false)
	out.PresenceWatch.User = resolveFeatureBool(guild.PresenceWatch.User, global.PresenceWatch.User, false)

	out.Maintenance.DBCleanup = resolveFeatureBool(guild.Maintenance.DBCleanup, global.Maintenance.DBCleanup, true)
	out.Safety.BotRolePermMirror = resolveFeatureBool(guild.Safety.BotRolePermMirror, global.Safety.BotRolePermMirror, true)
	out.Backfill.Enabled = resolveFeatureBool(guild.Backfill.Enabled, global.Backfill.Enabled, true)

	out.StatsChannels = resolveFeatureBool(guild.StatsChannels, global.StatsChannels, true)
	out.AutoRoleAssign = resolveFeatureBool(guild.AutoRoleAssign, global.AutoRoleAssign, true)
	out.UserPrune = resolveFeatureBool(guild.UserPrune, global.UserPrune, true)

	return out
}

// ConfigManager handles bot configuration management.
type ConfigManager struct {
	configFilePath  string
	logsDirPath     string
	config          *BotConfig
	guildIndex      map[string]int
	indexRebuilds   atomic.Uint64
	indexMisses     atomic.Uint64
	indexDuplicates atomic.Uint64
	mu              sync.RWMutex
	jsonManager     *util.JSONManager
}

// GuildIndexStats exposes counters for the guild config index.
type GuildIndexStats struct {
	Rebuilds   uint64
	Misses     uint64
	Duplicates uint64
}

// AvatarChange holds information about a user's avatar change.
type AvatarChange struct {
	UserID    string
	Username  string
	OldAvatar string
	NewAvatar string
	Timestamp time.Time
}

// ## Rule and Ruleset Types

// RuleType distinguishes between native Discord rules and custom bot rules.
const (
	RuleTypeNative = "native"
	RuleTypeCustom = "custom"
)

// List represents a single list in the system.
type List struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"` // "native" or "custom"
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	NativeID        string   `json:"native_id,omitempty"`        // Native list fields
	BlockedKeywords []string `json:"blocked_keywords,omitempty"` // Custom list fields
}

// Rule represents a rule that can load a set of lists.
type Rule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Lists   []List `json:"lists"` // Set of lists associated with the rule
	Enabled bool   `json:"enabled"`
}

// Ruleset holds a collection of rules.
type Ruleset struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Rules   []Rule `json:"rules"`
	Enabled bool   `json:"enabled"`
}

// StatusString returns a human-readable status for the ruleset (Enabled/Disabled).
func (r Ruleset) StatusString() string {
	if r.Enabled {
		return "Enabled"
	}
	return "Disabled"
}

// ## ConfigManager Methods

// AddList adds a list to the LooseLists of a guild.
func (mgr *ConfigManager) AddList(guildID string, list List) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		log.ErrorLoggerRaw().Error(fmt.Sprintf("GuildConfig not found for guildID: %s", guildID))
		return fmt.Errorf("guild not found")
	}
	guildConfig.LooseLists = append(guildConfig.LooseLists, Rule{
		ID:      list.ID,
		Name:    list.Name,
		Lists:   []List{list},
		Enabled: true,
	})
	log.DatabaseLogger().Info(fmt.Sprintf("List appended successfully for guildID: %s", guildID))
	return mgr.SaveConfig()
}

// AddRule adds a rule to the LooseLists of a guild.
func (mgr *ConfigManager) AddRule(guildID string, rule Rule) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		return fmt.Errorf("guild not found")
	}

	guildConfig.LooseLists = append(guildConfig.LooseLists, rule)
	return mgr.SaveConfig()
}

// AddRuleset adds a ruleset to a guild.
func (mgr *ConfigManager) AddRuleset(guildID string, ruleset Ruleset) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		return fmt.Errorf("guild not found")
	}

	guildConfig.Rulesets = append(guildConfig.Rulesets, ruleset)
	return mgr.SaveConfig()
}

// AddListToRule adds a list to a specific rule in a guild.
func (mgr *ConfigManager) AddListToRule(guildID string, ruleID string, list List) error {
	log.DatabaseLogger().Info(fmt.Sprintf("AddListToRule called with guildID: %s, ruleID: %s, listID: %s", guildID, ruleID, list.ID))
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		log.ErrorLoggerRaw().Error(fmt.Sprintf("GuildConfig not found for guildID: %s", guildID))
		return fmt.Errorf("guild not found")
	}

	for i, rule := range guildConfig.LooseLists {
		if rule.ID == ruleID {
			log.DatabaseLogger().Info(fmt.Sprintf("Rule found for ruleID: %s, appending list", ruleID))
			guildConfig.LooseLists[i].Lists = append(guildConfig.LooseLists[i].Lists, list)
			log.DatabaseLogger().Info(fmt.Sprintf("List appended successfully to ruleID: %s", ruleID))
			return mgr.SaveConfig()
		}
	}

	log.ErrorLoggerRaw().Error(fmt.Sprintf("Rule not found for ruleID: %s", ruleID))
	return fmt.Errorf("rule not found")
}

// ## GuildConfig Methods

// RolesCacheTTLDuration returns the configured TTL for the roles cache or a default of 5m.
func (gc *GuildConfig) RolesCacheTTLDuration() time.Duration {
	const def = 5 * time.Minute
	if gc == nil || gc.RolesCacheTTL == "" {
		return def
	}
	d, err := time.ParseDuration(gc.RolesCacheTTL)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// MemberCacheTTLDuration returns the configured TTL for the members cache or a default of 5m.
func (gc *GuildConfig) MemberCacheTTLDuration() time.Duration {
	const def = 5 * time.Minute
	if gc == nil || gc.MemberCacheTTL == "" {
		return def
	}
	d, err := time.ParseDuration(gc.MemberCacheTTL)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// GuildCacheTTLDuration returns the configured TTL for the guilds cache or a default of 15m.
func (gc *GuildConfig) GuildCacheTTLDuration() time.Duration {
	const def = 15 * time.Minute
	if gc == nil || gc.GuildCacheTTL == "" {
		return def
	}
	d, err := time.ParseDuration(gc.GuildCacheTTL)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// ChannelCacheTTLDuration returns the configured TTL for the channels cache or a default of 15m.
func (gc *GuildConfig) ChannelCacheTTLDuration() time.Duration {
	const def = 15 * time.Minute
	if gc == nil || gc.ChannelCacheTTL == "" {
		return def
	}
	d, err := time.ParseDuration(gc.ChannelCacheTTL)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// SetRolesCacheTTL sets the roles cache TTL per guild (e.g., "5m", "1h") and persists the setting.
func (mgr *ConfigManager) SetRolesCacheTTL(guildID string, ttl string) error {
	if guildID == "" {
		return fmt.Errorf("guild not found")
	}
	// Validate format (allow empty to reset to default)
	if ttl != "" {
		if _, err := time.ParseDuration(ttl); err != nil {
			return fmt.Errorf("invalid ttl: %w", err)
		}
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	gcfg := mgr.GuildConfig(guildID)
	if gcfg == nil {
		return fmt.Errorf("guild not found")
	}
	gcfg.RolesCacheTTL = ttl
	return mgr.SaveConfig()
}

// GetRolesCacheTTL gets the configured roles cache TTL (original string, e.g., "5m").
func (mgr *ConfigManager) GetRolesCacheTTL(guildID string) string {
	gcfg := mgr.GuildConfig(guildID)
	if gcfg == nil {
		return ""
	}
	return gcfg.RolesCacheTTL
}

// FindListByName searches for a list by its name in LooseLists.
func (gc *GuildConfig) FindListByName(name string) *List {
	for _, rule := range gc.LooseLists {
		for _, list := range rule.Lists {
			if list.Name == name {
				return &list
			}
		}
	}
	return nil
}

// ListExists checks if a list with the given name already exists in LooseLists.
func (gc *GuildConfig) ListExists(name string) bool {
	return gc.FindListByName(name) != nil
}

// ## Error Types

// ValidationError represents a validation error with field context.
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error.
func NewValidationError(field string, value interface{}, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// ConfigError represents configuration-related errors.
type ConfigError struct {
	Operation string
	Path      string
	Cause     error
}

func (e ConfigError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("config %s failed for %s: %v", e.Operation, e.Path, e.Cause)
	}
	return fmt.Sprintf("config %s failed for %s", e.Operation, e.Path)
}

func (e ConfigError) Unwrap() error {
	return e.Cause
}

// NewConfigError creates a new configuration error.
func NewConfigError(operation, path string, cause error) ConfigError {
	return ConfigError{
		Operation: operation,
		Path:      path,
		Cause:     cause,
	}
}

// DiscordError represents Discord API related errors.
type DiscordError struct {
	Operation string
	Code      int
	Message   string
	Cause     error
}

func (e DiscordError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("Discord API error during %s (code %d): %s", e.Operation, e.Code, e.Message)
	}
	return fmt.Sprintf("Discord API error during %s: %s", e.Operation, e.Message)
}

func (e DiscordError) Unwrap() error {
	return e.Cause
}

// NewDiscordError creates a new Discord API error.
func NewDiscordError(operation string, code int, message string, cause error) DiscordError {
	return DiscordError{
		Operation: operation,
		Code:      code,
		Message:   message,
		Cause:     cause,
	}
}

// ## Utility Functions

// IsRetryableError determines if an error can be retried.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific retryable errors.
	if errors.Is(err, ErrRateLimited) {
		return true
	}

	// Check for Discord errors that might be retryable.
	var discordErr DiscordError
	if errors.As(err, &discordErr) {
		// 5xx errors are typically retryable.
		return discordErr.Code >= 500 && discordErr.Code < 600
	}

	return false
}

// ## General Errors

var ErrRateLimited = errors.New("rate limited")
