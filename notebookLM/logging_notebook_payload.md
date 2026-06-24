# Domain Architecture: logging

## Layout Topology
```text
logging/
├── formatting.go
├── policy.go
└── policy_test.go
```

## Source Stream Aggregation

// === FILE: pkg/logging/formatting.go ===
```go
package logging

import (
	"fmt"
	"strings"
	"time"
)

// FormatUserLabel returns a standardized markdown label for a user.
func FormatUserLabel(username, userID string) string {
	userID = strings.TrimSpace(userID)
	username = strings.TrimSpace(username)
	if userID == "" {
		if username != "" {
			return "**" + username + "**"
		}
		return "Unknown"
	}
	if username == "" {
		return "<@" + userID + "> (`" + userID + "`)"
	}
	return fmt.Sprintf("**%s** (<@%s>, `%s`)", username, userID, userID)
}

// FormatUserRef returns a standardized mention reference for a user.
func FormatUserRef(userID string) string {
	return FormatUserLabel("", userID)
}

// FormatChannelLabel returns a standardized markdown mention for a channel.
func FormatChannelLabel(channelID string) string {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return "Unknown"
	}
	return "<#" + channelID + ">, `" + channelID + "`"
}

// FormatRoleLabel returns a standardized markdown mention for a role.
func FormatRoleLabel(roleID, roleName string) string {
	roleID = strings.TrimSpace(roleID)
	roleName = strings.TrimSpace(roleName)
	if roleID != "" {
		return "<@&" + roleID + "> (`" + roleID + "`)"
	}
	if roleName != "" {
		return "`" + roleName + "`"
	}
	return "Unknown"
}

// FormatDurationFull shows the full duration, omitting leading zero-valued units.
func FormatDurationFull(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int64(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	type comp struct {
		label string
		value int64
	}
	parts := []comp{
		{"days", days},
		{"hours", hours},
		{"minutes", minutes},
		{"seconds", seconds},
	}

	for len(parts) > 1 && parts[0].value == 0 {
		parts = parts[1:]
	}

	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%d %s", p.value, p.label)
	}
	return out
}

// FormatDurationSmart lists all non-zero units (no abbreviations).
func FormatDurationSmart(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int64(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	parts := []string{}

	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}
	if seconds > 0 {
		if seconds == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", seconds))
		}
	}

	return strings.Join(parts, " ")
}

// FormatDuration formats a time duration in a human-readable way.
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "`            `"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 365 {
		years := days / 365
		remainingDays := days % 365
		if years == 1 {
			return fmt.Sprintf("1 year, %d days", remainingDays)
		}
		return fmt.Sprintf("%d years, %d days", years, remainingDays)
	}

	if days > 30 {
		months := days / 30
		remainingDays := days % 30
		if months == 1 {
			return fmt.Sprintf("1 month, %d days", remainingDays)
		}
		return fmt.Sprintf("%d months, %d days", months, remainingDays)
	}

	if days > 0 {
		if days == 1 {
			return fmt.Sprintf("1 day, %d hours", hours)
		}
		return fmt.Sprintf("%d days, %d hours", days, hours)
	}

	if hours > 0 {
		if hours == 1 {
			return fmt.Sprintf("1 hour, %d minutes", minutes)
		}
		return fmt.Sprintf("%d hours, %d minutes", hours, minutes)
	}

	if minutes > 0 {
		if minutes == 1 {
			return "1 minutes"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}

	return "Less than 1 minute"
}

// TruncateString truncates a string to a maximum length.
func TruncateString(s string, maxLen int) string {
	if s == "" {
		return "*empty message*"
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

```

// === FILE: pkg/logging/policy.go ===
```go
package logging

import (
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// LogEventType identifies an internal logging event kind.
type LogEventType string

// LogEventRoleChange defines log event role change.
// LogEventMemberJoin defines log event member join.
// LogEventMessageProcess defines log event message process.
// LogEventMessageEdit defines log event message edit.
// LogEventMessageDelete defines log event message delete.
// LogEventReactionMetric defines log event reaction metric.
// LogEventAutomodAction defines log event automod action.
// LogEventModerationCase defines log event moderation case.
// LogEventMemberLeave defines log event member leave.
// LogEventAvatarChange defines log event avatar change.
// LogEventCleanAction defines log event clean action.
const (
	LogEventAvatarChange   LogEventType = "avatar_change"
	LogEventRoleChange     LogEventType = "role_change"
	LogEventMemberJoin     LogEventType = "member_join"
	LogEventMemberLeave    LogEventType = "member_leave"
	LogEventMessageProcess LogEventType = "message_process"
	LogEventMessageEdit    LogEventType = "message_edit"
	LogEventMessageDelete  LogEventType = "message_delete"
	LogEventReactionMetric LogEventType = "reaction_metric"
	LogEventAutomodAction  LogEventType = "automod_action"
	LogEventModerationCase LogEventType = "moderation_case"
	LogEventCleanAction    LogEventType = "clean_action"
)

// LogEventCategory groups events by subsystem.
type LogEventCategory string

// LogCategoryAutomod defines log category automod.
// LogCategoryModeration defines log category moderation.
// LogCategoryUser defines log category user.
// LogCategoryReaction defines log category reaction.
// LogCategoryMessage defines log category message.
const (
	LogCategoryUser       LogEventCategory = "user"
	LogCategoryMessage    LogEventCategory = "message"
	LogCategoryReaction   LogEventCategory = "reaction"
	LogCategoryAutomod    LogEventCategory = "automod"
	LogCategoryModeration LogEventCategory = "moderation"
)

// EmitReason is a deterministic reason for a should-emit decision.
type EmitReason string

// EmitReasonFeatureLoggingMessageDisabled defines emit reason feature logging message disabled.
// EmitReasonFeatureLoggingAutomodDisabled defines emit reason feature logging automod disabled.
// EmitReasonFeatureLoggingUserDisabled defines emit reason feature logging user disabled.
// EmitReasonFeatureLoggingModerationDisabled defines emit reason feature logging moderation disabled.
// EmitReasonConfigUnavailable defines emit reason config unavailable.
// EmitReasonConfigManagerUnavailable defines emit reason config manager unavailable.
// EmitReasonUnknownEvent defines emit reason unknown event.
// EmitReasonEnabled defines emit reason enabled.
// EmitReasonFeatureLoggingCleanDisabled defines emit reason feature logging clean disabled.
// EmitReasonRuntimeDisableUserLogs defines emit reason runtime disable user logs.
// EmitReasonFeatureLoggingReactionDisabled defines emit reason feature logging reaction disabled.
// EmitReasonGuildConfigMissing defines emit reason guild config missing.
// EmitReasonFeatureLoggingEntryExitDisabled defines emit reason feature logging entry exit disabled.
// EmitReasonRuntimeDisableReactionLogs defines emit reason runtime disable reaction logs.
// EmitReasonRuntimeModerationLoggingOff defines emit reason runtime moderation logging off.
// EmitReasonRuntimeDisableCleanLog defines emit reason runtime disable clean log.
// EmitReasonNoChannelConfigured defines emit reason no channel configured.
// EmitReasonMissingIntent defines emit reason missing intent.
// EmitReasonChannelInvalid defines emit reason channel invalid.
// EmitReasonRuntimeDisableEntryExitLogs defines emit reason runtime disable entry exit logs.
// EmitReasonRuntimeDisableMessageLogs defines emit reason runtime disable message logs.
const (
	EmitReasonEnabled                          EmitReason = "enabled"
	EmitReasonUnknownEvent                     EmitReason = "unknown_event"
	EmitReasonConfigManagerUnavailable         EmitReason = "config_manager_unavailable"
	EmitReasonConfigUnavailable                EmitReason = "config_unavailable"
	EmitReasonGuildConfigMissing               EmitReason = "guild_config_missing"
	EmitReasonFeatureLoggingUserDisabled       EmitReason = "feature_logging_user_disabled"
	EmitReasonFeatureLoggingEntryExitDisabled  EmitReason = "feature_logging_entry_exit_disabled"
	EmitReasonFeatureLoggingMessageDisabled    EmitReason = "feature_logging_message_disabled"
	EmitReasonFeatureLoggingReactionDisabled   EmitReason = "feature_logging_reaction_disabled"
	EmitReasonFeatureLoggingAutomodDisabled    EmitReason = "feature_logging_automod_disabled"
	EmitReasonFeatureLoggingModerationDisabled EmitReason = "feature_logging_moderation_disabled"
	EmitReasonFeatureLoggingCleanDisabled      EmitReason = "feature_logging_clean_disabled"
	EmitReasonRuntimeDisableUserLogs           EmitReason = "runtime_disable_user_logs"
	EmitReasonRuntimeDisableEntryExitLogs      EmitReason = "runtime_disable_entry_exit_logs"
	EmitReasonRuntimeDisableMessageLogs        EmitReason = "runtime_disable_message_logs"
	EmitReasonRuntimeDisableReactionLogs       EmitReason = "runtime_disable_reaction_logs"
	EmitReasonRuntimeModerationLoggingOff      EmitReason = "runtime_moderation_logging_off"
	EmitReasonRuntimeDisableCleanLog           EmitReason = "runtime_disable_clean_log"
	EmitReasonNoChannelConfigured              EmitReason = "no_channel_configured"
	EmitReasonMissingIntent                    EmitReason = "missing_intent"
	EmitReasonChannelInvalid                   EmitReason = "channel_invalid"
)

// LogEventCapability describes governance and requirements for a log event.
type LogEventCapability struct {
	EventType           LogEventType
	Category            LogEventCategory
	RequiredIntentsMask uint64
	RequiresChannel     bool
	// Toggles documents the two policy layers:
	// 1) `features.*` => product behavior (fine-grain enablement)
	// 2) `runtime_config.disable_*` => operational kill switch
	// Precedence is enforced in ShouldEmitLogEvent: kill switch wins.
	Toggles                    []string
	ValidateChannelPerms       bool
	RequireExclusiveModeration bool
}

// EmitDecision is the result of ShouldEmitLogEvent.
type EmitDecision struct {
	EventType   LogEventType
	Category    LogEventCategory
	Enabled     bool
	Reason      EmitReason
	ChannelID   string
	Capability  LogEventCapability
	MissingMask int
}

var logEventCapabilities = map[LogEventType]LogEventCapability{
	LogEventAvatarChange: {
		EventType:           LogEventAvatarChange,
		Category:            LogCategoryUser,
		RequiredIntentsMask: 0,
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_user_logs", "features.logging.avatar_logging"},
	},
	LogEventRoleChange: {
		EventType:           LogEventRoleChange,
		Category:            LogCategoryUser,
		RequiredIntentsMask: (1 << 1),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_user_logs", "features.logging.role_update"},
	},
	LogEventMemberJoin: {
		EventType:           LogEventMemberJoin,
		Category:            LogCategoryUser,
		RequiredIntentsMask: (1 << 1),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_entry_exit_logs", "features.logging.member_join"},
	},
	LogEventMemberLeave: {
		EventType:           LogEventMemberLeave,
		Category:            LogCategoryUser,
		RequiredIntentsMask: (1 << 1),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_entry_exit_logs", "features.logging.member_leave"},
	},
	LogEventMessageProcess: {
		EventType:           LogEventMessageProcess,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: (1 << 9),
		RequiresChannel:     false,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message_process"},
	},
	LogEventMessageEdit: {
		EventType:           LogEventMessageEdit,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: (1 << 9),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message_edit"},
	},
	LogEventMessageDelete: {
		EventType:           LogEventMessageDelete,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: (1 << 9),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message_delete"},
	},
	LogEventReactionMetric: {
		EventType:           LogEventReactionMetric,
		Category:            LogCategoryReaction,
		RequiredIntentsMask: (1 << 10),
		RequiresChannel:     false,
		Toggles:             []string{"runtime_config.disable_reaction_logs", "features.logging.reaction_metric"},
	},
	LogEventAutomodAction: {
		EventType:            LogEventAutomodAction,
		Category:             LogCategoryAutomod,
		RequiredIntentsMask:  uint64(1 << 20), // IntentAutoModerationExecution
		RequiresChannel:      true,
		Toggles:              []string{"runtime_config.disable_automod_logs", "features.logging.automod_action"},
		ValidateChannelPerms: true,
	},
	LogEventModerationCase: {
		EventType:                  LogEventModerationCase,
		Category:                   LogCategoryModeration,
		RequiredIntentsMask:        0,
		RequiresChannel:            true,
		Toggles:                    []string{"runtime_config.moderation_logging", "features.logging.moderation_case"},
		ValidateChannelPerms:       true,
		RequireExclusiveModeration: true,
	},
	LogEventCleanAction: {
		EventType:            LogEventCleanAction,
		Category:             LogCategoryModeration,
		RequiredIntentsMask:  0,
		RequiresChannel:      true,
		Toggles:              []string{"runtime_config.disable_clean_log", "features.logging.clean_action"},
		ValidateChannelPerms: true,
	},
}

// LogEventCapabilities returns a copy of the event capability map.
func LogEventCapabilities() map[LogEventType]LogEventCapability {
	out := make(map[LogEventType]LogEventCapability, len(logEventCapabilities))
	for k, v := range logEventCapabilities {
		out[k] = v
	}
	return out
}

// ResolveLogChannel returns the resolved channel ID for an event in a guild.
// Resolution is deterministic and event-specific.
func ResolveLogChannel(eventType LogEventType, guildID string, configManager *files.ConfigManager) string {
	if guildID == "" || configManager == nil {
		return ""
	}
	gcfg := configManager.GuildConfig(guildID)
	return resolveLogChannelForGuild(eventType, gcfg)
}

// CheckFeatureEnabled only verifies if the configuration allows the event to be emitted.
// It DOES NOT check intents or permissions. It should be used by the domain layer to determine if
// an event should be processed.
func CheckFeatureEnabled(configManager *files.ConfigManager, eventType LogEventType, guildID string) EmitDecision {
	capability, ok := logEventCapabilities[eventType]
	if !ok {
		return EmitDecision{EventType: eventType, Enabled: false, Reason: EmitReasonUnknownEvent}
	}

	decision := EmitDecision{
		EventType:  eventType,
		Category:   capability.Category,
		Enabled:    false,
		Reason:     EmitReasonConfigUnavailable,
		Capability: capability,
	}

	if configManager == nil {
		decision.Reason = EmitReasonConfigManagerUnavailable
		return decision
	}
	cfg := configManager.Config()
	if cfg == nil {
		decision.Reason = EmitReasonConfigUnavailable
		return decision
	}
	gcfg := configManager.GuildConfig(guildID)
	if gcfg == nil {
		decision.Reason = EmitReasonGuildConfigMissing
		return decision
	}

	rc := cfg.ResolveRuntimeConfig(guildID)
	features := cfg.ResolveFeatures(guildID)

	if reason, disabled := evaluateEventToggle(eventType, rc, features); disabled {
		decision.Reason = reason
		return decision
	}

	if capability.RequiresChannel {
		channelID := resolveLogChannelForGuild(eventType, gcfg)
		if channelID == "" {
			decision.Reason = EmitReasonNoChannelConfigured
			return decision
		}
		decision.ChannelID = channelID
	}

	decision.Enabled = true
	decision.Reason = EmitReasonEnabled
	return decision
}

// DiscordAdapter defines the state methods needed for permission evaluation.
type DiscordAdapter interface {
	CanLogToChannel(channelID string) (bool, error)
	ValidateModerationLogChannel(guildID, channelID string) error
}

// ValidateLogCapability checks gateway intents and channel permissions.
// It expects CheckFeatureEnabled to have returned Enabled = true.
func ValidateLogCapability(state DiscordAdapter, currentIntents uint64, decision EmitDecision, guildID string, configManager *files.ConfigManager) (EmitReason, uint64, bool) {
	if !decision.Enabled {
		return decision.Reason, 0, false
	}

	if decision.Capability.RequiresChannel && decision.ChannelID != "" {
		gcfg := configManager.GuildConfig(guildID)
		if reason, ok := validateResolvedLogChannel(state, decision.Capability, decision.ChannelID, guildID, gcfg); !ok {
			return reason, 0, false
		}
	}

	if reason, mask, gated := evaluateIntentRequirements(decision.Capability, currentIntents); gated {
		return reason, mask, false
	}

	return EmitReasonEnabled, 0, true
}

// evaluateEventToggle applies the per-event runtime kill switch and feature toggle in
// that precedence order. It returns the disabling reason and true when the event is
// gated off, or ("", false) when neither toggle blocks emission.
func evaluateEventToggle(eventType LogEventType, rc files.RuntimeConfig, features files.ResolvedFeatureToggles) (EmitReason, bool) {
	switch eventType {
	case LogEventAvatarChange:
		if rc.DisableUserLogs {
			return EmitReasonRuntimeDisableUserLogs, true
		}
	case LogEventRoleChange:
		if rc.DisableUserLogs {
			return EmitReasonRuntimeDisableUserLogs, true
		}
	case LogEventMemberJoin:
		if rc.DisableEntryExitLogs {
			return EmitReasonRuntimeDisableEntryExitLogs, true
		}
	case LogEventMemberLeave:
		if rc.DisableEntryExitLogs {
			return EmitReasonRuntimeDisableEntryExitLogs, true
		}
	case LogEventMessageProcess:
		if rc.DisableMessageLogs {
			return EmitReasonRuntimeDisableMessageLogs, true
		}
	case LogEventMessageEdit:
		if rc.DisableMessageLogs {
			return EmitReasonRuntimeDisableMessageLogs, true
		}
	case LogEventMessageDelete:
		if rc.DisableMessageLogs {
			return EmitReasonRuntimeDisableMessageLogs, true
		}
	case LogEventReactionMetric:
		if rc.DisableReactionLogs {
			return EmitReasonRuntimeDisableReactionLogs, true
		}
	case LogEventAutomodAction:
		// No runtime config disable override for automod logs.
	case LogEventModerationCase:
		if !rc.ModerationLoggingEnabled() {
			return EmitReasonRuntimeModerationLoggingOff, true
		}
	case LogEventCleanAction:
		if rc.DisableCleanLog {
			return EmitReasonRuntimeDisableCleanLog, true
		}
	}
	return "", false
}

// validateResolvedLogChannel validates exclusivity and bot permissions for a resolved channel.
func validateResolvedLogChannel(st DiscordAdapter, capability LogEventCapability, channelID string, guildID string, gcfg *files.GuildConfig) (EmitReason, bool) {
	if !capability.ValidateChannelPerms {
		return EmitReasonEnabled, true
	}
	if capability.RequireExclusiveModeration && IsSharedModerationChannel(channelID, gcfg) {
		return EmitReasonChannelInvalid, false
	}

	if err := ValidateModerationLogChannel(st, guildID, channelID); err != nil {
		return EmitReasonChannelInvalid, false
	}
	return EmitReasonEnabled, true
}

// evaluateIntentRequirements reports whether the capability's required gateway intents are
// missing from the active session, returning the missing-bit mask when they are.
func evaluateIntentRequirements(capability LogEventCapability, currentIntents uint64) (EmitReason, uint64, bool) {
	if capability.RequiredIntentsMask == 0 {
		return "", 0, false
	}
	missing := capability.RequiredIntentsMask &^ currentIntents
	if missing != 0 {
		return EmitReasonMissingIntent, missing, true
	}
	return "", 0, false
}

func resolveLogChannelForGuild(eventType LogEventType, gcfg *files.GuildConfig) string {
	if gcfg == nil {
		return ""
	}
	channels := gcfg.Channels
	switch eventType {
	case LogEventAvatarChange:
		return firstNonEmptyChannel(channels.AvatarLogging)
	case LogEventRoleChange:
		return firstNonEmptyChannel(channels.RoleUpdate)
	case LogEventMemberJoin:
		return firstNonEmptyChannel(channels.MemberJoin, channels.MemberLeave)
	case LogEventMemberLeave:
		return firstNonEmptyChannel(channels.MemberLeave, channels.MemberJoin)
	case LogEventMessageEdit:
		return firstNonEmptyChannel(channels.MessageEdit, channels.MessageDelete)
	case LogEventMessageDelete:
		return firstNonEmptyChannel(channels.MessageDelete, channels.MessageEdit)
	case LogEventAutomodAction:
		return firstNonEmptyChannel(channels.AutomodAction)
	case LogEventModerationCase:
		return firstNonEmptyChannel(channels.ModerationCase)
	case LogEventCleanAction:
		return firstNonEmptyChannel(channels.CleanAction, channels.ModerationCase)
	default:
		return ""
	}
}

func firstNonEmptyChannel(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// IsSharedModerationChannel is shared moderation channel.
func IsSharedModerationChannel(channelID string, gcfg *files.GuildConfig) bool {
	channelID = strings.TrimSpace(channelID)
	if gcfg == nil || channelID == "" {
		return false
	}
	sharedCandidates := []string{
		gcfg.Channels.Commands,
		gcfg.Channels.AvatarLogging,
		gcfg.Channels.RoleUpdate,
		gcfg.Channels.MemberJoin,
		gcfg.Channels.MemberLeave,
		gcfg.Channels.MessageEdit,
		gcfg.Channels.MessageDelete,
		gcfg.Channels.AutomodAction,
		gcfg.Channels.CleanAction,
	}
	for _, candidate := range sharedCandidates {
		if strings.TrimSpace(candidate) == channelID {
			return true
		}
	}
	return false
}

// ValidateModerationLogChannel validates moderation log channel.
func ValidateModerationLogChannel(st DiscordAdapter, guildID, channelIDStr string) error {
	if st == nil {
		return fmt.Errorf("state is nil")
	}
	if guildID == "" || channelIDStr == "" {
		return fmt.Errorf("missing guildID or channelID")
	}

	return st.ValidateModerationLogChannel(guildID, channelIDStr)
}

// FormatAvatarURL builds the CDN URL for an avatar hash
func FormatAvatarURL(userID, avatarHash string) string {
	if avatarHash == "" {
		return ""
	}
	ext := ".png"
	if strings.HasPrefix(avatarHash, "a_") {
		ext = ".gif"
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s%s", userID, avatarHash, ext)
}

```

// === FILE: pkg/logging/policy_test.go ===
```go
package logging

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// Mock transport to avoid live API calls
type mockTransport struct{}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

func TestFormatUserLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		username string
		userID   string
		expected string
	}{
		{"", "", "Unknown"},
		{"alice", "", "**alice**"},
		{"", "12345", "<@12345> (`12345`)"},
		{"alice", "12345", "**alice** (<@12345>, `12345`)"},
		{" alice ", " 12345 ", "**alice** (<@12345>, `12345`)"},
	}

	for _, tt := range tests {
		got := FormatUserLabel(tt.username, tt.userID)
		if got != tt.expected {
			t.Errorf("FormatUserLabel(%q, %q) = %q; expected %q", tt.username, tt.userID, got, tt.expected)
		}
	}
}

func TestFormatUserRef(t *testing.T) {
	t.Parallel()
	got := FormatUserRef("123")
	expected := "<@123> (`123`)"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatChannelLabel(t *testing.T) {
	t.Parallel()
	if FormatChannelLabel("") != "Unknown" {
		t.Errorf("expected Unknown for empty channel ID")
	}
	got := FormatChannelLabel("123")
	expected := "<#123>, `123`"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatRoleLabel(t *testing.T) {
	t.Parallel()
	if FormatRoleLabel("123", "") != "<@&123> (`123`)" {
		t.Errorf("unexpected role reference output")
	}
	if FormatRoleLabel("", "admin") != "`admin`" {
		t.Errorf("unexpected role name output")
	}
	if FormatRoleLabel("", "") != "Unknown" {
		t.Errorf("expected Unknown for empty role")
	}
}

func TestFormatDurationFull(t *testing.T) {
	t.Parallel()
	if FormatDurationFull(-1) != "0 seconds" {
		t.Errorf("expected 0 seconds for negative duration")
	}
	d := 24*time.Hour + 3*time.Hour + 4*time.Minute + 5*time.Second
	got := FormatDurationFull(d)
	expected := "1 days 3 hours 4 minutes 5 seconds"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatDurationSmart(t *testing.T) {
	t.Parallel()
	if FormatDurationSmart(-1) != "" {
		t.Errorf("expected empty string for negative duration")
	}
	d1 := 24*time.Hour + 1*time.Hour + 1*time.Minute + 1*time.Second
	if FormatDurationSmart(d1) != "1 day 1 hour 1 minute 1 second" {
		t.Errorf("unexpected output: %q", FormatDurationSmart(d1))
	}
	d2 := 48*time.Hour + 2*time.Hour + 2*time.Minute + 2*time.Second
	if FormatDurationSmart(d2) != "2 days 2 hours 2 minutes 2 seconds" {
		t.Errorf("unexpected output: %q", FormatDurationSmart(d2))
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	if FormatDuration(0) != "`            `" {
		t.Errorf("unexpected zero duration output")
	}
	// Years
	if FormatDuration(400*24*time.Hour) != "1 year, 35 days" {
		t.Errorf("unexpected year output: %q", FormatDuration(400*24*time.Hour))
	}
	if FormatDuration(800*24*time.Hour) != "2 years, 70 days" {
		t.Errorf("unexpected years output")
	}
	// Months
	if FormatDuration(45*24*time.Hour) != "1 month, 15 days" {
		t.Errorf("unexpected month output")
	}
	if FormatDuration(75*24*time.Hour) != "2 months, 15 days" {
		t.Errorf("unexpected months output")
	}
	// Days
	if FormatDuration(1*24*time.Hour+3*time.Hour) != "1 day, 3 hours" {
		t.Errorf("unexpected day output")
	}
	if FormatDuration(2*24*time.Hour+4*time.Hour) != "2 days, 4 hours" {
		t.Errorf("unexpected days output")
	}
	// Hours
	if FormatDuration(1*time.Hour+3*time.Minute) != "1 hour, 3 minutes" {
		t.Errorf("unexpected hour output")
	}
	if FormatDuration(2*time.Hour+4*time.Minute) != "2 hours, 4 minutes" {
		t.Errorf("unexpected hours output")
	}
	// Minutes
	if FormatDuration(1*time.Minute) != "1 minutes" {
		t.Errorf("unexpected minute output")
	}
	if FormatDuration(5*time.Minute) != "5 minutes" {
		t.Errorf("unexpected minutes output")
	}
	// Less than 1 minute
	if FormatDuration(30*time.Second) != "Less than 1 minute" {
		t.Errorf("unexpected seconds output")
	}
}

func TestTruncateString(t *testing.T) {
	t.Parallel()
	if TruncateString("", 10) != "*empty message*" {
		t.Errorf("unexpected empty message output")
	}
	if TruncateString("hello", 10) != "hello" {
		t.Errorf("unexpected no truncate output")
	}
	if TruncateString("hello world", 8) != "hello..." {
		t.Errorf("unexpected truncate output: %q", TruncateString("hello world", 8))
	}
}

func TestLogEventCapabilities(t *testing.T) {
	t.Parallel()
	caps := LogEventCapabilities()
	if len(caps) != len(logEventCapabilities) {
		t.Errorf("expected same length")
	}
}

func TestResolveLogChannel(t *testing.T) {
	t.Parallel()
	// nil inputs
	if ResolveLogChannel(LogEventAvatarChange, "111", nil) != "" {
		t.Errorf("expected empty string for nil config manager")
	}
	if ResolveLogChannel(LogEventAvatarChange, "", &files.ConfigManager{}) != "" {
		t.Errorf("expected empty string for empty guild ID")
	}

	store := &files.MemoryConfigStore{}
	_ = store.Save(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "111",
				Channels: files.ChannelsConfig{
					AvatarLogging:  "avatar_ch",
					RoleUpdate:     "role_ch",
					MemberJoin:     "join_ch",
					MemberLeave:    "leave_ch",
					MessageEdit:    "edit_ch",
					MessageDelete:  "delete_ch",
					AutomodAction:  "automod_ch",
					ModerationCase: "mod_ch",
					CleanAction:    "clean_ch",
				},
			},
		},
	})
	mgr := files.NewConfigManagerWithStore(store, nil)
	_ = mgr.LoadConfig()

	// Test resolutions for all events
	eventsToChannels := map[LogEventType]string{
		LogEventAvatarChange:    "avatar_ch",
		LogEventRoleChange:      "role_ch",
		LogEventMemberJoin:      "join_ch",
		LogEventMemberLeave:     "leave_ch",
		LogEventMessageEdit:     "edit_ch",
		LogEventMessageDelete:   "delete_ch",
		LogEventAutomodAction:   "automod_ch",
		LogEventModerationCase:  "mod_ch",
		LogEventCleanAction:     "clean_ch",
		LogEventType("unknown"): "",
	}

	for evt, expected := range eventsToChannels {
		got := ResolveLogChannel(evt, "111", mgr)
		if got != expected {
			t.Errorf("ResolveLogChannel(%s) = %q; expected %q", evt, got, expected)
		}
	}
}

func TestCheckFeatureEnabled_Errors(t *testing.T) {
	t.Parallel()
	// Unknown event
	dec := CheckFeatureEnabled(nil, LogEventType("unknown"), "111")
	if dec.Enabled || dec.Reason != EmitReasonUnknownEvent {
		t.Errorf("expected EmitReasonUnknownEvent")
	}

	// nil config manager
	dec = CheckFeatureEnabled(nil, LogEventAvatarChange, "111")
	if dec.Enabled || dec.Reason != EmitReasonConfigManagerUnavailable {
		t.Errorf("expected EmitReasonConfigManagerUnavailable")
	}

	// config unavailable (nil config store load)
	mgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	dec = CheckFeatureEnabled(mgr, LogEventAvatarChange, "111")
	if dec.Enabled || dec.Reason != EmitReasonConfigUnavailable {
		t.Errorf("expected EmitReasonConfigUnavailable")
	}

	// guild config missing
	_ = mgr.LoadConfig()
	dec = CheckFeatureEnabled(mgr, LogEventAvatarChange, "111")
	if dec.Enabled || dec.Reason != EmitReasonGuildConfigMissing {
		t.Errorf("expected EmitReasonGuildConfigMissing")
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestCheckFeatureEnabled_Toggles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		eventType LogEventType
		rc        files.RuntimeConfig
		expected  EmitReason
	}{
		{LogEventAvatarChange, files.RuntimeConfig{DisableUserLogs: true}, EmitReasonRuntimeDisableUserLogs},
		{LogEventRoleChange, files.RuntimeConfig{DisableUserLogs: true}, EmitReasonRuntimeDisableUserLogs},
		{LogEventMemberJoin, files.RuntimeConfig{DisableEntryExitLogs: true}, EmitReasonRuntimeDisableEntryExitLogs},
		{LogEventMemberLeave, files.RuntimeConfig{DisableEntryExitLogs: true}, EmitReasonRuntimeDisableEntryExitLogs},
		{LogEventMessageProcess, files.RuntimeConfig{DisableMessageLogs: true}, EmitReasonRuntimeDisableMessageLogs},
		{LogEventMessageEdit, files.RuntimeConfig{DisableMessageLogs: true}, EmitReasonRuntimeDisableMessageLogs},
		{LogEventMessageDelete, files.RuntimeConfig{DisableMessageLogs: true}, EmitReasonRuntimeDisableMessageLogs},
		{LogEventReactionMetric, files.RuntimeConfig{DisableReactionLogs: true}, EmitReasonRuntimeDisableReactionLogs},
		{LogEventCleanAction, files.RuntimeConfig{DisableCleanLog: true}, EmitReasonRuntimeDisableCleanLog},
		{LogEventModerationCase, files.RuntimeConfig{ModerationLogging: boolPtr(false)}, EmitReasonRuntimeModerationLoggingOff},
	}

	for _, tt := range tests {
		store := &files.MemoryConfigStore{}
		_ = store.Save(&files.BotConfig{
			Guilds: []files.GuildConfig{
				{
					GuildID: "111",
					Channels: files.ChannelsConfig{
						AvatarLogging:  "ch",
						RoleUpdate:     "ch",
						MemberJoin:     "ch",
						MemberLeave:    "ch",
						MessageEdit:    "ch",
						MessageDelete:  "ch",
						AutomodAction:  "ch",
						ModerationCase: "ch",
						CleanAction:    "ch",
					},
					RuntimeConfig: tt.rc,
				},
			},
		})
		mgr := files.NewConfigManagerWithStore(store, nil)
		_ = mgr.LoadConfig()

		dec := CheckFeatureEnabled(mgr, tt.eventType, "111")
		if dec.Enabled || dec.Reason != tt.expected {
			t.Errorf("event %s: expected decision gated off by %s, got enabled=%t reason=%s", tt.eventType, tt.expected, dec.Enabled, dec.Reason)
		}
	}
}

func TestCheckFeatureEnabled_NoChannelConfigured(t *testing.T) {
	t.Parallel()
	store := &files.MemoryConfigStore{}
	_ = store.Save(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "111", // all channels empty
			},
		},
	})
	mgr := files.NewConfigManagerWithStore(store, nil)
	_ = mgr.LoadConfig()

	dec := CheckFeatureEnabled(mgr, LogEventAvatarChange, "111")
	if dec.Enabled || dec.Reason != EmitReasonNoChannelConfigured {
		t.Errorf("expected EmitReasonNoChannelConfigured, got %v", dec.Reason)
	}
}

func TestValidateLogCapability(t *testing.T) {
	t.Parallel()
	// Test not enabled decision
	dec := EmitDecision{Enabled: false, Reason: EmitReasonUnknownEvent}
	reason, _, ok := ValidateLogCapability(nil, 0, dec, "111", nil)
	if ok || reason != EmitReasonUnknownEvent {
		t.Errorf("expected false and reason unknown_event")
	}

	// Test validate resolved log channel: when ValidateChannelPerms is false
	decEnabled := EmitDecision{
		Enabled: true,
		Capability: LogEventCapability{
			RequiresChannel:      true,
			ValidateChannelPerms: false,
		},
		ChannelID: "123",
	}
	reason, _, ok = ValidateLogCapability(nil, 0, decEnabled, "111", nil)
	if !ok || reason != EmitReasonEnabled {
		t.Errorf("expected validation success when ValidateChannelPerms is false")
	}

	// Test validate resolved log channel: exclusive moderation channel conflict
	store := &files.MemoryConfigStore{}
	_ = store.Save(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "111",
				Channels: files.ChannelsConfig{
					Commands:       "123", // conflicts with channelID 123
					ModerationCase: "123",
				},
			},
		},
	})
	mgr := files.NewConfigManagerWithStore(store, nil)
	_ = mgr.LoadConfig()

	decExcl := EmitDecision{
		Enabled: true,
		Capability: LogEventCapability{
			RequiresChannel:            true,
			ValidateChannelPerms:       true,
			RequireExclusiveModeration: true,
		},
		ChannelID: "123",
	}
	reason, _, ok = ValidateLogCapability(nil, 0, decExcl, "111", mgr)
	if ok || reason != EmitReasonChannelInvalid {
		t.Errorf("expected EmitReasonChannelInvalid for shared moderation channel, got reason=%s ok=%t", reason, ok)
	}

	// Test validate resolved log channel: check intent requirement validation
	decIntent := EmitDecision{
		Enabled: true,
		Capability: LogEventCapability{
			RequiresChannel:      true,
			ValidateChannelPerms: false,
			RequiredIntentsMask:  uint64((1 << 1)),
		},
		ChannelID: "123",
	}
	reason, mask, ok := ValidateLogCapability(nil, 0, decIntent, "111", mgr)
	if ok || reason != EmitReasonMissingIntent || mask != uint64((1<<1)) {
		t.Errorf("expected missing intent, got reason=%s mask=%v ok=%t", reason, mask, ok)
	}
}

type mockDiscordAdapter2 struct {
	canLog bool
	valid  bool
	err    error
}

func (m *mockDiscordAdapter2) CanLogToChannel(channelID string) (bool, error) {
	return m.canLog, m.err
}

func (m *mockDiscordAdapter2) ValidateModerationLogChannel(guildID, channelID string) error {
	if m.err != nil {
		return m.err
	}
	if !m.valid {
		return fmt.Errorf("invalid channel")
	}
	return nil
}

func TestValidateModerationLogChannel(t *testing.T) {
	err := ValidateModerationLogChannel(nil, "111", "222")
	if err == nil {
		t.Error("expected err on nil state")
	}

	err = ValidateModerationLogChannel(&mockDiscordAdapter2{valid: true}, "", "222")
	if err == nil {
		t.Error("expected err on empty guild")
	}

	m := &mockDiscordAdapter2{valid: false}
	err = ValidateModerationLogChannel(m, "111", "222")
	if err == nil {
		t.Error("expected err on invalid channel")
	}

	m.valid = true
	err = ValidateModerationLogChannel(m, "111", "222")
	if err != nil {
		t.Error("expected no error")
	}
}

func TestIsSharedModerationChannel(t *testing.T) {
	t.Parallel()
	if IsSharedModerationChannel("123", nil) {
		t.Errorf("expected false for nil guild config")
	}
	if IsSharedModerationChannel("", &files.GuildConfig{}) {
		t.Errorf("expected false for empty channel ID")
	}

	gcfg := &files.GuildConfig{
		Channels: files.ChannelsConfig{
			Commands: "123",
		},
	}
	if !IsSharedModerationChannel("123", gcfg) {
		t.Errorf("expected true for commands channel match")
	}
	if IsSharedModerationChannel("456", gcfg) {
		t.Errorf("expected false for no match")
	}
}

```

