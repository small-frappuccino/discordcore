package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RuntimeConfig centralizes operational toggles/parameters that were previously
// controlled via environment variables. These values are meant to be edited
// from Discord via an interactive embed and persisted in the active config store.
//
// Keep names in CAPS to mirror the previous env var names and make auditing easy.
type RuntimeConfig struct {
	Database DatabaseRuntimeConfig `json:"database,omitempty"`

	// THEME
	BotTheme string `json:"bot_theme,omitempty"`

	// SERVICES (LOGGING)
	DisableDBCleanup     bool `json:"disable_db_cleanup,omitempty"`
	DisableMessageLogs   bool `json:"disable_message_logs,omitempty"`
	DisableEntryExitLogs bool `json:"disable_entry_exit_logs,omitempty"`
	DisableReactionLogs  bool `json:"disable_reaction_logs,omitempty"`
	DisableUserLogs      bool `json:"disable_user_logs,omitempty"`
	DisableCleanLog      bool `json:"disable_clean_log,omitempty"`
	// MODERATION LOGS
	// true/nil: send moderation logs automatically
	// false: do not send moderation logs
	ModerationLogging  *bool  `json:"moderation_logging,omitempty"`
	LogModerationScope string `json:"log_moderation_scope,omitempty"` // discordcore, all_bots, all

	// PRESENCE WATCH
	PresenceWatchUserID string `json:"presence_watch_user_id,omitempty"`
	PresenceWatchBot    bool   `json:"presence_watch_bot,omitempty"`

	// MESSAGE CACHE
	MessageCacheTTLHours int  `json:"message_cache_ttl_hours,omitempty"`
	MessageDeleteOnLog   bool `json:"message_delete_on_log,omitempty"`
	MessageCacheCleanup  bool `json:"message_cache_cleanup,omitempty"`

	// TASK ROUTER
	// 0 means "use the runtime default budget".
	GlobalMaxWorkers int `json:"global_max_workers,omitempty"`

	// BACKFILL (ENTRY/EXIT)
	BackfillChannelID   string `json:"backfill_channel_id,omitempty"`
	BackfillStartDay    string `json:"backfill_start_day,omitempty"` // YYYY-MM-DD, default: today UTC when empty
	BackfillInitialDate string `json:"backfill_initial_date,omitempty"`
	MimuWelcomeString   string `json:"mimu_welcome_string,omitempty"`
	MimuGoodbyeString   string `json:"mimu_goodbye_string,omitempty"`

	// BOT ROLE PERMISSION MIRRORING (SAFETY)
	// Previously controllable via env vars:
	//   - ALICE_DISABLE_BOT_ROLE_PERM_MIRROR
	//   - ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID
	DisableBotRolePermMirror     bool   `json:"disable_bot_role_perm_mirror,omitempty"`
	BotRolePermMirrorActorRoleID string `json:"bot_role_perm_mirror_actor_role_id,omitempty"`

	// Webhook embed message patch (global or per-guild override).
	// Intended for editing an existing webhook message embed by ID.
	WebhookEmbedUpdates []WebhookEmbedUpdateConfig `json:"webhook_embed_updates,omitempty"`
	// Remote validation behavior for webhook embed targets used by CRUD commands.
	WebhookEmbedValidation WebhookEmbedValidationConfig `json:"webhook_embed_validation,omitempty"`

	// Toggle to disable ephemeral messages for interactive embeds per guild.
	DisableInteractiveEphemeral bool `json:"disable_interactive_ephemeral,omitempty"`

	// Global Pastebin Credentials (safely encrypted)
	PastebinDevKey       EncryptedString `json:"pastebin_dev_key,omitempty"`
	PastebinUserName     EncryptedString `json:"pastebin_user_name,omitempty"`
	PastebinUserPassword EncryptedString `json:"pastebin_user_password,omitempty"`
}

// UnmarshalJSON decodes a RuntimeConfig and absorbs legacy persisted keys into
// their canonical successors so older settings files continue to load:
//   - "moderation_log_mode" (off/non-off string) migrates into ModerationLogging
//     when ModerationLogging is unset
//   - "webhook_embed_update" (single-entry legacy form) is appended to
//     WebhookEmbedUpdates when no non-empty canonical entry shadows it
//
// The legacy keys never round-trip into the public type; the marshalled form
// only emits the canonical fields.
func (rc *RuntimeConfig) UnmarshalJSON(data []byte) error {
	type alias RuntimeConfig
	type rawRuntimeConfig struct {
		alias
		// Deprecated: migrated to ModerationLogging
		ModerationLogMode string `json:"moderation_log_mode,omitempty"`
		// Deprecated: migrated to WebhookEmbedUpdates
		WebhookEmbedUpdate WebhookEmbedUpdateConfig `json:"webhook_embed_update,omitempty"`
	}

	var raw rawRuntimeConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("RuntimeConfig.UnmarshalJSON: %w", err)
	}

	*rc = RuntimeConfig(raw.alias)

	if rc.ModerationLogging == nil && strings.TrimSpace(raw.ModerationLogMode) != "" {
		rc.ModerationLogging = boolPtr(strings.ToLower(strings.TrimSpace(raw.ModerationLogMode)) != "off")
	}

	if !raw.WebhookEmbedUpdate.IsZero() {
		hasCanonical := false
		for _, item := range rc.WebhookEmbedUpdates {
			if !item.IsZero() {
				hasCanonical = true
				break
			}
		}
		if !hasCanonical {
			rc.WebhookEmbedUpdates = append(rc.WebhookEmbedUpdates, raw.WebhookEmbedUpdate)
		}
	}

	return nil
}

// DatabaseRuntimeConfig defines runtime database configuration.
// The only supported driver is postgres.
type DatabaseRuntimeConfig struct {
	Driver              string `json:"driver,omitempty"`
	DatabaseURL         string `json:"database_url,omitempty"`
	MaxOpenConns        int    `json:"max_open_conns,omitempty"`
	MaxIdleConns        int    `json:"max_idle_conns,omitempty"`
	ConnMaxLifetimeSecs int    `json:"conn_max_lifetime_secs,omitempty"`
	ConnMaxIdleTimeSecs int    `json:"conn_max_idle_time_secs,omitempty"`
	PingTimeoutMS       int    `json:"ping_timeout_ms,omitempty"`
}

// WebhookEmbedUpdateConfig defines how to patch an existing webhook message embed.
type WebhookEmbedUpdateConfig struct {
	MessageID  string          `json:"message_id,omitempty"`
	WebhookURL string          `json:"webhook_url,omitempty"`
	Embed      json.RawMessage `json:"embed,omitempty"`
}

// WebhookEmbedValidationModeSoft defines webhook embed validation mode soft.
// WebhookEmbedValidationModeStrict defines webhook embed validation mode strict.
// DefaultWebhookEmbedValidationTimeoutMS defines default webhook embed validation timeout ms.
// WebhookEmbedValidationModeOff defines webhook embed validation mode off.
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

// NormalizedWebhookEmbedUpdates returns the canonical webhook_embed_updates list
// with empty placeholder entries filtered out. The legacy single-entry
// "webhook_embed_update" key is migrated into this slice at JSON decode time by
// RuntimeConfig.UnmarshalJSON, so callers no longer need a fallback branch.
func (rc RuntimeConfig) NormalizedWebhookEmbedUpdates() []WebhookEmbedUpdateConfig {
	updates := make([]WebhookEmbedUpdateConfig, 0, len(rc.WebhookEmbedUpdates))
	for _, item := range rc.WebhookEmbedUpdates {
		if item.IsZero() {
			continue
		}
		updates = append(updates, item)
	}
	if len(updates) == 0 {
		return nil
	}
	return updates
}

// EffectiveWebhookEmbedValidation resolves webhook_embed_validation defaults.
func (rc RuntimeConfig) EffectiveWebhookEmbedValidation() WebhookEmbedValidationConfig {
	return rc.WebhookEmbedValidation.Normalized()
}

// ## Config Types

// ChannelsConfig groups channel IDs per guild.
type ChannelsConfig struct {
	Commands string `json:"commands,omitempty"`

	// Event/feature-scoped channels (canonical settings schema).
	AvatarLogging  string `json:"avatar_logging,omitempty"`
	RoleUpdate     string `json:"role_update,omitempty"`
	MemberJoin     string `json:"member_join,omitempty"`
	MemberLeave    string `json:"member_leave,omitempty"`
	MessageEdit    string `json:"message_edit,omitempty"`
	MessageDelete  string `json:"message_delete,omitempty"`
	AutomodAction  string `json:"automod_action,omitempty"`
	ModerationCase string `json:"moderation_case,omitempty"`
	CleanAction    string `json:"clean_action,omitempty"`
	EntryBackfill  string `json:"entry_backfill,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (cc *ChannelsConfig) UnmarshalJSON(data []byte) error {
	type alias ChannelsConfig
	type rawChannelsConfig struct {
		alias
		// Deprecated: removed in favor of native features
		VerificationCleanup string `json:"verification_cleanup,omitempty"`
	}

	var raw rawChannelsConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("ChannelsConfig.UnmarshalJSON: %w", err)
	}

	*cc = ChannelsConfig(raw.alias)
	return nil
}

// BackfillChannelID backfills channel id.
func (cc ChannelsConfig) BackfillChannelID() string {
	return strings.TrimSpace(cc.EntryBackfill)
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
	Channels []StatsChannelConfig `json:"channels,omitempty"`
}

// AutoAssignmentConfig defines automatic role assignment rules.
type AutoAssignmentConfig struct {
	Enabled       bool     `json:"enabled,omitempty"`
	TargetRoleID  string   `json:"target_role,omitempty"`
	RequiredRoles []string `json:"required_roles,omitempty"`
}

// RolesConfig groups role-related settings per guild.
type RolesConfig struct {
	Allowed        []string             `json:"allowed,omitempty"`
	DashboardRead  []string             `json:"dashboard_read,omitempty"`
	DashboardWrite []string             `json:"dashboard_write,omitempty"`
	AutoAssignment AutoAssignmentConfig `json:"auto_assignment,omitempty"`
	BoosterRole    string               `json:"booster_role,omitempty"`
	MuteRole       string               `json:"mute_role,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (rc *RolesConfig) UnmarshalJSON(data []byte) error {
	type alias RolesConfig
	type rawRolesConfig struct {
		alias
		// Deprecated: removed in favor of native features
		VerificationRole string `json:"verification_role,omitempty"`
	}

	var raw rawRolesConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("RolesConfig.UnmarshalJSON: %w", err)
	}

	*rc = RolesConfig(raw.alias)
	return nil
}

// EmbedUpdateTargetTypeWebhookMessage defines embed update target type webhook message.
// EmbedUpdateTargetTypeChannelMessage defines embed update target type channel message.
const (
	EmbedUpdateTargetTypeWebhookMessage = "webhook_message"
	EmbedUpdateTargetTypeChannelMessage = "channel_message"
)

// EmbedUpdateTargetConfig defines the target used to update one existing message embed.
// Supported target types:
// - webhook_message: requires message_id + webhook_url
// - channel_message: requires message_id + channel_id
type EmbedUpdateTargetConfig struct {
	Type       string `json:"type,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	ChannelID  string `json:"channel_id,omitempty"`
	WebhookURL string `json:"webhook_url,omitempty"`
}

// IsZero reports whether all fields are empty.
func (c EmbedUpdateTargetConfig) IsZero() bool {
	return strings.TrimSpace(c.Type) == "" &&
		strings.TrimSpace(c.MessageID) == "" &&
		strings.TrimSpace(c.ChannelID) == "" &&
		strings.TrimSpace(c.WebhookURL) == ""
}

// PartnerEntryConfig defines one partner record for a board.
type PartnerEntryConfig struct {
	Fandom string `json:"fandom,omitempty"`
	Name   string `json:"name,omitempty"`
	Link   string `json:"link,omitempty"`
}

// PartnerBoardTemplateConfig controls board rendering behavior.
type PartnerBoardTemplateConfig struct {
	Title                      string `json:"title,omitempty"`
	ContinuationTitle          string `json:"continuation_title,omitempty"`
	Intro                      string `json:"intro,omitempty"`
	SectionHeaderTemplate      string `json:"section_header_template,omitempty"`
	SectionContinuationSuffix  string `json:"section_continuation_suffix,omitempty"`
	SectionContinuationPattern string `json:"section_continuation_template,omitempty"`
	LineTemplate               string `json:"line_template,omitempty"`
	EmptyStateText             string `json:"empty_state_text,omitempty"`
	FooterTemplate             string `json:"footer_template,omitempty"`
	OtherFandomLabel           string `json:"other_fandom_label,omitempty"`
	Color                      int    `json:"color,omitempty"`
	DisableFandomSorting       bool   `json:"disable_fandom_sorting,omitempty"`
	DisablePartnerSorting      bool   `json:"disable_partner_sorting,omitempty"`
}

// PartnerBoardConfig stores target, template, and partner records.
type PartnerBoardConfig struct {
	Template PartnerBoardTemplateConfig `json:"template,omitempty"`
	Partners []PartnerEntryConfig       `json:"partners,omitempty"`
	Postings []CustomEmbedPostingConfig `json:"postings,omitempty"`
}

// QOTDDeckConfig stores one named QOTD deck plus its target delivery channel.
type QOTDDeckConfig struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Enabled   bool   `json:"enabled,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	// SelectionStrategy controls how the next ready question is picked at
	// automatic publish time: "queue" (default — head of the queue, the
	// historical behavior) or "random" (uniformly random eligible question).
	// The visible thread numbering ("Question #001"...) is independent of
	// this strategy because each post carries its own publish ordinal.
	SelectionStrategy string `json:"selection_strategy,omitempty"`
}

// QOTDPublishScheduleConfig stores the UTC publish boundary for one guild.
type QOTDPublishScheduleConfig struct {
	HourUTC   *int `json:"hour_utc,omitempty"`
	MinuteUTC *int `json:"minute_utc,omitempty"`
}

// QOTDConfig stores per-guild question-of-the-day deck settings.
type QOTDConfig struct {
	VerifiedRoleID string                    `json:"verified_role_id,omitempty"`
	ActiveDeckID   string                    `json:"active_deck_id,omitempty"`
	Decks          []QOTDDeckConfig          `json:"decks,omitempty"`
	Schedule       QOTDPublishScheduleConfig `json:"schedule,omitempty"`
	// SuppressScheduledPublishDatesUTC is the canonical set of UTC publish
	// dates (YYYY-MM-DD) for which the scheduler must skip its automatic
	// publish. Manual publishes that consume a slot, or maintenance flows
	// that pause one specific day, add entries here; the runtime trims
	// expired dates on each cycle. Replaces the legacy single-string field
	// "suppress_scheduled_publish_date_utc" — JSON unmarshal still accepts
	// the legacy form and migrates it into this slice so old persisted
	// configs continue to load.
	SuppressScheduledPublishDatesUTC []string `json:"suppress_scheduled_publish_dates_utc,omitempty"`
}

// UserPruneConfig controls periodic user pruning per guild.
type UserPruneConfig struct {
	// Enabled toggles the automatic monthly prune.
	// true: execute native Discord prune automatically on day 28 (30-day inactivity window)
	// false: do not execute automatically
	Enabled bool `json:"enabled,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (upc *UserPruneConfig) UnmarshalJSON(data []byte) error {
	type alias UserPruneConfig
	type rawUserPruneConfig struct {
		alias
		// Deprecated: removed in favor of native Discord prune (Enabled toggle)
		GraceDays int `json:"grace_days,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		ScanIntervalMins int `json:"scan_interval_mins,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		InitialDelaySecs int `json:"initial_delay_secs,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		KicksPerSecond int `json:"kps,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		MaxKicksPerRun int `json:"max_kicks_per_run,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		ExemptRoleIDs []string `json:"exempt_role_ids,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		DryRun bool `json:"dry_run,omitempty"`
	}

	var raw rawUserPruneConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("UserPruneConfig.UnmarshalJSON: %w", err)
	}

	*upc = UserPruneConfig(raw.alias)
	return nil
}

// ReactionBlockEmojiConfig stores one blocked emoji selector.
//
// Kind is one of:
// - "custom": Value is the custom emoji ID, Name is the display name
// - "unicode": Value is the Unicode emoji, Alias is an optional :shortcode:
type ReactionBlockEmojiConfig struct {
	Kind     string `json:"kind,omitempty"`
	Value    string `json:"value,omitempty"`
	Name     string `json:"name,omitempty"`
	Alias    string `json:"alias,omitempty"`
	Animated bool   `json:"animated,omitempty"`
}

// ReactionBlockRuleConfig stores the blocked emoji list for one reactor/target pair.
type ReactionBlockRuleConfig struct {
	ReactorUserID string                     `json:"reactor_user_id,omitempty"`
	TargetUserID  string                     `json:"target_user_id,omitempty"`
	Emojis        []ReactionBlockEmojiConfig `json:"emojis,omitempty"`
}

// ReactionBlockConfig stores per-guild emoji reaction restrictions.
type ReactionBlockConfig struct {
	Rules []ReactionBlockRuleConfig `json:"rules,omitempty"`
}

// TicketsCategoryConfig maps a ticket category name to its assigned Role ID.
type TicketsCategoryConfig struct {
	Name   string `json:"name,omitempty"`
	RoleID string `json:"role_id,omitempty"`
}

// TicketsConfig stores ticket system configuration per guild.
type TicketsConfig struct {
	Enabled             bool                    `json:"enabled,omitempty"`
	TranscriptChannelID string                  `json:"transcript_channel_id,omitempty"`
	Categories          []TicketsCategoryConfig `json:"categories,omitempty"`
}

// GuildConfig holds the configuration for a specific guild.
type GuildConfig struct {
	GuildID             string                     `json:"guild_id"`
	ConfigVersion       int64                      `json:"config_version"`
	BotInstanceTokens   map[string]EncryptedString `json:"bot_instance_tokens,omitempty"`
	BotInstanceStatuses map[string]string          `json:"bot_instance_statuses,omitempty"`
	FeatureRouting      map[string]string          `json:"feature_routing,omitempty"`
	Features            FeatureToggles             `json:"features,omitempty"`
	Channels            ChannelsConfig             `json:"channels,omitempty"`
	Roles               RolesConfig                `json:"roles,omitempty"`
	Stats               StatsConfig                `json:"stats,omitempty"`

	// Cache TTL configuration (per-guild tuning)
	RolesCacheTTL   string `json:"roles_cache_ttl,omitempty"`   // e.g.: "5m", "1h" (default: "5m")
	MemberCacheTTL  string `json:"member_cache_ttl,omitempty"`  // e.g.: "5m", "10m" (default: "5m")
	GuildCacheTTL   string `json:"guild_cache_ttl,omitempty"`   // e.g.: "15m", "30m" (default: "15m")
	ChannelCacheTTL string `json:"channel_cache_ttl,omitempty"` // e.g.: "15m", "30m" (default: "15m")

	UserPrune UserPruneConfig `json:"user_prune,omitempty"`

	PartnerBoard   PartnerBoardConfig  `json:"partner_board,omitempty"`
	ReactionBlocks ReactionBlockConfig `json:"reaction_blocks,omitempty"`
	QOTD           QOTDConfig          `json:"qotd,omitempty"`
	Tickets        TicketsConfig       `json:"tickets,omitempty"`
	RolePanels     []RolePanelConfig   `json:"role_panels,omitempty"`
	CustomEmbeds   []CustomEmbedConfig `json:"custom_embeds,omitempty"`

	// RuntimeConfig allows per-guild overrides for certain settings.
	RuntimeConfig RuntimeConfig `json:"runtime_config,omitempty"`

	// LogModerationScope determines what moderation events are logged.
	LogModerationScope string `json:"log_moderation_scope,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (gc *GuildConfig) UnmarshalJSON(data []byte) error {
	type alias GuildConfig
	type rawGuildConfig struct {
		alias
		BotInstanceID        string            `json:"bot_instance_id,omitempty"`
		DomainBotInstanceIDs map[string]string `json:"domain_bot_instance_ids,omitempty"`
	}

	var raw rawGuildConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("GuildConfig.UnmarshalJSON: %w", err)
	}

	if raw.BotInstanceID != "" || len(raw.DomainBotInstanceIDs) > 0 {
		if raw.BotInstanceTokens == nil {
			raw.BotInstanceTokens = make(map[string]EncryptedString)
		}

		if raw.BotInstanceID != "" {
			normalized := NormalizeBotInstanceID(raw.BotInstanceID)
			if normalized != "" {
				if _, exists := raw.BotInstanceTokens[normalized]; !exists {
					raw.BotInstanceTokens[normalized] = ""
				}
			}
		}

		for _, instanceID := range raw.DomainBotInstanceIDs {
			normalized := NormalizeBotInstanceID(instanceID)
			if normalized != "" {
				if _, exists := raw.BotInstanceTokens[normalized]; !exists {
					raw.BotInstanceTokens[normalized] = ""
				}
			}
		}
	}

	*gc = GuildConfig(raw.alias)
	return nil
}

// BotConfig holds the configuration for the bot.
type BotConfig struct {
	ConfigVersion int64         `json:"config_version"`
	Guilds        []GuildConfig `json:"guilds"`

	// Features holds optional toggles for runtime behavior overrides.
	Features FeatureToggles `json:"features,omitempty"`

	// RuntimeConfig holds bot-level runtime overrides editable from Discord.
	// This intentionally replaces the previous "env var toggles" for operational
	// behavior (except for token), so settings can be managed in-app.
	//
	// NOTE: These are NOT environment variables. They are persisted in the active config store.
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

// ResolveRuntimeConfig returns the runtime configuration for a guild,
// falling back to the global one if the field is not defined (zero-value).
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

	if guildRC.Database.Driver != "" {
		resolved.Database.Driver = guildRC.Database.Driver
	}
	if guildRC.Database.DatabaseURL != "" {
		resolved.Database.DatabaseURL = guildRC.Database.DatabaseURL
	}
	if guildRC.Database.MaxOpenConns != 0 {
		resolved.Database.MaxOpenConns = guildRC.Database.MaxOpenConns
	}
	if guildRC.Database.MaxIdleConns != 0 {
		resolved.Database.MaxIdleConns = guildRC.Database.MaxIdleConns
	}
	if guildRC.Database.ConnMaxLifetimeSecs != 0 {
		resolved.Database.ConnMaxLifetimeSecs = guildRC.Database.ConnMaxLifetimeSecs
	}
	if guildRC.Database.ConnMaxIdleTimeSecs != 0 {
		resolved.Database.ConnMaxIdleTimeSecs = guildRC.Database.ConnMaxIdleTimeSecs
	}
	if guildRC.Database.PingTimeoutMS != 0 {
		resolved.Database.PingTimeoutMS = guildRC.Database.PingTimeoutMS
	}

	if guildRC.BotTheme != "" {
		resolved.BotTheme = guildRC.BotTheme
	}

	if guildRC.DisableDBCleanup {
		resolved.DisableDBCleanup = true
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
	}
	if guildRC.LogModerationScope != "" {
		resolved.LogModerationScope = guildRC.LogModerationScope
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
	if guildRC.GlobalMaxWorkers != 0 {
		resolved.GlobalMaxWorkers = guildRC.GlobalMaxWorkers
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

	if guildRC.MimuWelcomeString != "" {
		resolved.MimuWelcomeString = guildRC.MimuWelcomeString
	}
	if guildRC.MimuGoodbyeString != "" {
		resolved.MimuGoodbyeString = guildRC.MimuGoodbyeString
	}

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
	}
	if guildRC.DisableInteractiveEphemeral {
		resolved.DisableInteractiveEphemeral = true
	}
	return resolved
}

// ModerationLoggingEnabled resolves whether moderation logs should be sent.
// Defaults to true when runtime_config.moderation_logging is unset; the legacy
// "moderation_log_mode" key is migrated into ModerationLogging at JSON decode
// time by RuntimeConfig.UnmarshalJSON.
func (rc RuntimeConfig) ModerationLoggingEnabled() bool {
	if rc.ModerationLogging != nil {
		return *rc.ModerationLogging
	}
	return true
}

// ConfigSubscriber receives notifications when the bot configuration changes.
type ConfigSubscriber func(oldCfg, newCfg *BotConfig)

// ConfigManager handles bot configuration management.
//
// Concurrency: ConfigManager is safe for concurrent use by multiple goroutines.
// Readers should treat Config() and GuildConfig() results as read-only snapshots;
// persist changes through the existing update helpers.
type ConfigManager struct {
	configFilePath  string
	logsDirPath     string
	store           ConfigStore
	logger          *slog.Logger
	config          *BotConfig
	guildIndex      map[string]int
	published       atomic.Pointer[publishedConfigSnapshot]
	indexRebuilds   atomic.Uint64
	indexMisses     atomic.Uint64
	indexDuplicates atomic.Uint64
	subscribers     []ConfigSubscriber
	mu              sync.RWMutex
}

type publishedConfigSnapshot struct {
	config     *BotConfig
	guildIndex map[string]int
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

func guildConfigByID(cfg *BotConfig, guildID string) (*GuildConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: guild_id=%s", ErrGuildConfigNotFound, strings.TrimSpace(guildID))
	}

	target := strings.TrimSpace(guildID)
	if target == "" {
		return nil, fmt.Errorf("%w: guild_id=%s", ErrGuildConfigNotFound, target)
	}

	for idx := range cfg.Guilds {
		if cfg.Guilds[idx].GuildID == target {
			return &cfg.Guilds[idx], nil
		}
	}
	return nil, fmt.Errorf("%w: guild_id=%s", ErrGuildConfigNotFound, target)
}

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
	_, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		gcfg, err := guildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("guild not found")
		}
		gcfg.RolesCacheTTL = ttl
		return nil
	})
	return err
}

// GetRolesCacheTTL gets the configured roles cache TTL (original string, e.g., "5m").
func (mgr *ConfigManager) GetRolesCacheTTL(guildID string) string {
	gcfg := mgr.GuildConfig(guildID)
	if gcfg == nil {
		return ""
	}
	return gcfg.RolesCacheTTL
}

// ## Error Types

// ValidationError represents a validation error with field context.
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

// ValidationField validations field.
func (e ValidationError) ValidationField() string {
	return e.Field
}

// Error errors.
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error.
func NewValidationError(field string, value any, message string) ValidationError {
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

// ConfigErrorPath configs error path.
func (e ConfigError) ConfigErrorPath() string {
	return e.Path
}

// Error errors.
func (e ConfigError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("config %s failed for %s: %v", e.Operation, e.Path, e.Cause)
	}
	return fmt.Sprintf("config %s failed for %s", e.Operation, e.Path)
}

// Unwrap unwraps.
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

// DiscordErrorCode discords error code.
func (e DiscordError) DiscordErrorCode() int {
	return e.Code
}

// Error errors.
func (e DiscordError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("Discord API error during %s (code %d): %s", e.Operation, e.Code, e.Message)
	}
	return fmt.Sprintf("Discord API error during %s: %s", e.Operation, e.Message)
}

// Unwrap unwraps.
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

// ErrRateLimited defines err rate limited.
var ErrRateLimited = errors.New("rate limited")
