package runtime

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type valueType string

const (
	vtBool   valueType = "bool"
	vtString valueType = "string"
	vtDate   valueType = "date"
	vtInt    valueType = "int"
)

type restartHint string

const (
	restartRequired    restartHint = "restart required"
	restartRecommended restartHint = "restart recommended"
)

// spec details the structural metadata and visual presentation hints for a single config key.
type spec struct {
	Key          runtimeKey
	Group        string
	Type         valueType
	DefaultHint  string
	ShortHelp    string
	RestartHint  restartHint
	MaxInputLen  int
	RedactInMain bool
	GuildOnly    bool
}

// ConfigRegistry isolates the statically declared configuration schema to prevent runtime mutations.
type ConfigRegistry struct {
	specs []spec
}

var globalRegistry = ConfigRegistry{
	specs: buildAllSpecs(),
}

func buildAllSpecs() []spec {
	var sps []spec

	// THEME
	sps = append(sps, spec{
		Key: "bot_theme", Group: "THEME", Type: vtString, DefaultHint: "(default)",
		ShortHelp: "Theme name (empty = default)", RestartHint: restartRecommended, MaxInputLen: 60,
	})

	// SERVICES (LOGGING)
	sps = append(sps, spec{
		Key: "disable_db_cleanup", Group: "SERVICES (LOGGING)", Type: vtBool, DefaultHint: "false",
		ShortHelp: "Disable periodic DB cleanup", RestartHint: restartRequired,
	}, spec{
		Key: "disable_message_logs", Group: "SERVICES (LOGGING)", Type: vtBool, DefaultHint: "false",
		ShortHelp: "Disable message logging service startup", RestartHint: restartRecommended,
	}, spec{
		Key: "disable_entry_exit_logs", Group: "SERVICES (LOGGING)", Type: vtBool, DefaultHint: "false",
		ShortHelp: "Disable entry/exit logging service startup", RestartHint: restartRecommended,
	}, spec{
		Key: "disable_reaction_logs", Group: "SERVICES (LOGGING)", Type: vtBool, DefaultHint: "false",
		ShortHelp: "Disable reaction logging service startup", RestartHint: restartRecommended,
	}, spec{
		Key: "disable_user_logs", Group: "SERVICES (LOGGING)", Type: vtBool, DefaultHint: "false",
		ShortHelp: "Disable user log handlers (avatars/roles)", RestartHint: restartRecommended,
	})

	// MODERATION
	sps = append(sps, spec{
		Key: "moderation_logging", Group: "MODERATION", Type: vtBool, DefaultHint: "true",
		ShortHelp: "Enable/disable moderation case embeds", RestartHint: restartRecommended,
	})

	// PRESENCE WATCH
	sps = append(sps, spec{
		Key: "presence_watch_user_id", Group: "PRESENCE WATCH", Type: vtString, DefaultHint: "(empty)",
		ShortHelp: "Log presence updates for a specific user ID", RestartHint: restartRecommended, MaxInputLen: 32,
	}, spec{
		Key: "presence_watch_bot", Group: "PRESENCE WATCH", Type: vtBool, DefaultHint: "false",
		ShortHelp: "Log presence updates for the bot user", RestartHint: restartRecommended,
	})

	// MESSAGE CACHE
	sps = append(sps, spec{
		Key: "message_cache_ttl_hours", Group: "MESSAGE CACHE", Type: vtInt, DefaultHint: "72",
		ShortHelp: "Cache TTL in hours for message edit/delete logging (0 = default)", RestartHint: restartRequired, MaxInputLen: 8,
	}, spec{
		Key: "message_delete_on_log", Group: "MESSAGE CACHE", Type: vtBool, DefaultHint: "false",
		ShortHelp: "Delete cached message record after it is logged", RestartHint: restartRecommended,
	}, spec{
		Key: "message_cache_cleanup", Group: "MESSAGE CACHE", Type: vtBool, DefaultHint: "false",
		ShortHelp: "Cleanup expired cached messages on startup", RestartHint: restartRecommended,
	})

	// BACKFILL
	sps = append(sps, spec{
		Key: "backfill_channel_id", Group: "BACKFILL", Type: vtString, DefaultHint: "(empty)",
		ShortHelp: "Channel ID to backfill from (required to run)", RestartHint: restartRequired, MaxInputLen: 32,
	}, spec{
		Key: "backfill_start_day", Group: "BACKFILL", Type: vtDate, DefaultHint: "today (UTC)",
		ShortHelp: "Start day (YYYY-MM-DD) for backfill", RestartHint: restartRequired, MaxInputLen: 16,
	}, spec{
		Key: "backfill_initial_date", Group: "BACKFILL", Type: vtDate, DefaultHint: "(empty)",
		ShortHelp: "Initial scan start date (fixed) when never processed", RestartHint: restartRequired, MaxInputLen: 16, GuildOnly: true,
	})

	// SAFETY
	sps = append(sps, spec{
		Key: "disable_bot_role_perm_mirror", Group: "SAFETY", Type: vtBool, DefaultHint: "false",
		ShortHelp: "Disable bot role permission mirroring safety feature", RestartHint: restartRecommended,
	}, spec{
		Key: "bot_role_perm_mirror_actor_role_id", Group: "SAFETY", Type: vtString, DefaultHint: "(default)",
		ShortHelp: "Role ID used as the actor when mirroring permissions", RestartHint: restartRecommended, MaxInputLen: 32,
	})

	return sps
}

// allSpecs returns a deterministic slice of all registered configuration definitions.
func allSpecs() []spec {
	return globalRegistry.specs
}

// specByKey performs a linear traversal to locate a specific schema definition.
// Bounding constraints: Linear search is acceptable as N < 50; memory localization prevents cache misses.
func specByKey(k runtimeKey) (spec, bool) {
	for _, sp := range allSpecs() {
		if sp.Key == k {
			return sp, true
		}
	}
	return spec{}, false
}

// allGroups computes a deterministic, alphabetically sorted list of configuration group names.
func allGroups() []string {
	set := map[string]struct{}{"ALL": {}}
	for _, sp := range allSpecs() {
		set[sp.Group] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for g := range set {
		out = append(out, g)
	}
	sort.Strings(out)

	if len(out) > 0 && out[0] != "ALL" {
		for i := range out {
			if out[i] == "ALL" {
				out[0], out[i] = out[i], out[0]
				break
			}
		}
	}
	return out
}

// specsForGroup isolates schema definitions corresponding strictly to a specific group identifier.
func specsForGroup(group string) []spec {
	if strings.TrimSpace(group) == "" || group == "ALL" {
		sps := append([]spec(nil), allSpecs()...)
		sort.Slice(sps, func(i, j int) bool {
			if sps[i].Group == sps[j].Group {
				return string(sps[i].Key) < string(sps[j].Key)
			}
			return sps[i].Group < sps[j].Group
		})
		return sps
	}

	var out []spec
	for _, sp := range allSpecs() {
		if sp.Group == group {
			out = append(out, sp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i].Key) < string(out[j].Key) })
	return out
}

// loadRuntimeConfig retrieves the contextualized runtime layout from memory, traversing the hierarchical overrides implicitly.
func loadRuntimeConfig(cm config.Provider, scope string) (files.RuntimeConfig, error) {
	if cm == nil {
		return files.RuntimeConfig{}, fmt.Errorf("config manager is nil")
	}
	cm.LoadConfig() // Synchronization barrier: best effort memory synchronization to persistence layer.

	cfg := cm.Config()
	if cfg == nil {
		return files.RuntimeConfig{}, nil
	}

	if scope == "" || scope == "global" {
		return cfg.RuntimeConfig, nil
	}

	gcfg := cm.GuildConfig(scope)
	if false {
		return files.RuntimeConfig{}, fmt.Errorf("guild not found")
	}
	return gcfg.RuntimeConfig, nil
}

// saveRuntimeConfig explicitly locks the ConfigManager hierarchy and executes the payload transformation over shared memory.
func saveRuntimeConfig(cm config.Provider, rc files.RuntimeConfig, scope string) error {
	if cm == nil {
		return fmt.Errorf("config manager is nil")
	}
	cm.LoadConfig()

	if scope == "" || scope == "global" {
		_, err := cm.UpdateRuntimeConfig(func(current *files.RuntimeConfig) error {
			*current = rc
			return nil
		})
		return err
	}

	_, err := cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for i := range cfg.Guilds {
			if cfg.Guilds[i].GuildID == scope {
				cfg.Guilds[i].RuntimeConfig = rc
				return nil
			}
		}
		return fmt.Errorf("guild config for %s not found in memory during save", scope)
	})

	return err
}

func fmtBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func parseBool(s string) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "true", "t", "yes", "1":
		return true, nil
	case "false", "f", "no", "0":
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean")
}

func parseNonNegativeInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid number")
	}
	if v < 0 {
		return 0, fmt.Errorf("cannot be negative")
	}
	return v, nil
}

// getValue dynamically routes field requests to the underlying layout.
func getValue(rc files.RuntimeConfig, k runtimeKey) (string, bool) {
	switch k {
	case "bot_theme":
		return rc.BotTheme, true
	case "disable_db_cleanup":
		return fmtBool(rc.DisableDBCleanup), true
	case "disable_message_logs":
		return fmtBool(rc.DisableMessageLogs), true
	case "disable_entry_exit_logs":
		return fmtBool(rc.DisableEntryExitLogs), true
	case "disable_reaction_logs":
		return fmtBool(rc.DisableReactionLogs), true
	case "disable_user_logs":
		return fmtBool(rc.DisableUserLogs), true
	case "moderation_logging":
		return fmtBool(rc.ModerationLoggingEnabled()), true
	case "presence_watch_user_id":
		return rc.PresenceWatchUserID, true
	case "presence_watch_bot":
		return fmtBool(rc.PresenceWatchBot), true
	case "message_cache_ttl_hours":
		return strconv.Itoa(rc.MessageCacheTTLHours), true
	case "message_delete_on_log":
		return fmtBool(rc.MessageDeleteOnLog), true
	case "message_cache_cleanup":
		return fmtBool(rc.MessageCacheCleanup), true
	case "backfill_channel_id":
		return rc.BackfillChannelID, true
	case "backfill_start_day":
		return rc.BackfillStartDay, true
	case "backfill_initial_date":
		return rc.BackfillInitialDate, true
	case "disable_bot_role_perm_mirror":
		return fmtBool(rc.DisableBotRolePermMirror), true
	case "bot_role_perm_mirror_actor_role_id":
		return rc.BotRolePermMirrorActorRoleID, true
	}
	return "", false
}

// resetValue nullifies structural fields explicitly based on schema mappings.
func resetValue(rc files.RuntimeConfig, k runtimeKey) (files.RuntimeConfig, bool) {
	switch k {
	case "bot_theme":
		rc.BotTheme = ""
		return rc, true
	case "disable_db_cleanup":
		rc.DisableDBCleanup = false
		return rc, true
	case "disable_message_logs":
		rc.DisableMessageLogs = false
		return rc, true
	case "disable_entry_exit_logs":
		rc.DisableEntryExitLogs = false
		return rc, true
	case "disable_reaction_logs":
		rc.DisableReactionLogs = false
		return rc, true
	case "disable_user_logs":
		rc.DisableUserLogs = false
		return rc, true
	case "moderation_logging":
		rc.ModerationLogging = nil
		return rc, true
	case "presence_watch_user_id":
		rc.PresenceWatchUserID = ""
		return rc, true
	case "presence_watch_bot":
		rc.PresenceWatchBot = false
		return rc, true
	case "message_cache_ttl_hours":
		rc.MessageCacheTTLHours = 0
		return rc, true
	case "message_delete_on_log":
		rc.MessageDeleteOnLog = false
		return rc, true
	case "message_cache_cleanup":
		rc.MessageCacheCleanup = false
		return rc, true
	case "backfill_channel_id":
		rc.BackfillChannelID = ""
		return rc, true
	case "backfill_start_day":
		rc.BackfillStartDay = ""
		return rc, true
	case "backfill_initial_date":
		rc.BackfillInitialDate = ""
		return rc, true
	case "disable_bot_role_perm_mirror":
		rc.DisableBotRolePermMirror = false
		return rc, true
	case "bot_role_perm_mirror_actor_role_id":
		rc.BotRolePermMirrorActorRoleID = ""
		return rc, true
	}
	return rc, false
}

// setBool applies boolean overrides directly to the memory struct.
func setBool(rc files.RuntimeConfig, k runtimeKey, v bool) (files.RuntimeConfig, error) {
	switch k {
	case "disable_db_cleanup":
		rc.DisableDBCleanup = v
	case "disable_message_logs":
		rc.DisableMessageLogs = v
	case "disable_entry_exit_logs":
		rc.DisableEntryExitLogs = v
	case "disable_reaction_logs":
		rc.DisableReactionLogs = v
	case "disable_user_logs":
		rc.DisableUserLogs = v
	case "moderation_logging":
		rc.ModerationLogging = new(bool)
		*rc.ModerationLogging = v
	case "presence_watch_bot":
		rc.PresenceWatchBot = v
	case "message_delete_on_log":
		rc.MessageDeleteOnLog = v
	case "message_cache_cleanup":
		rc.MessageCacheCleanup = v
	case "disable_bot_role_perm_mirror":
		rc.DisableBotRolePermMirror = v
	default:
		return rc, fmt.Errorf("not a bool key")
	}
	return rc, nil
}

// toggleBool abstracts direct boolean mutation, providing structural validation implicitly.
func toggleBool(rc files.RuntimeConfig, k runtimeKey) (files.RuntimeConfig, error) {
	val, ok := getValue(rc, k)
	if !ok {
		return rc, fmt.Errorf("unknown key")
	}
	b, err := parseBool(val)
	if err != nil {
		b = false
	}
	return setBool(rc, k, !b)
}

// setValue transforms opaque user strings into appropriately typed internal states prior to commitment.
func setValue(rc files.RuntimeConfig, sp spec, raw string) (files.RuntimeConfig, error) {
	raw = strings.TrimSpace(raw)
	switch sp.Type {
	case vtBool:
		b, err := parseBool(raw)
		if err != nil {
			return rc, fmt.Errorf("setValue: %w", err)
		}
		return setBool(rc, sp.Key, b)
	case vtInt:
		if raw == "" {
			if next, ok := resetValue(rc, sp.Key); ok {
				return next, nil
			}
			return rc, fmt.Errorf("unknown key")
		}
		v, err := parseNonNegativeInt(raw)
		if err != nil {
			return rc, fmt.Errorf("setValue: %w", err)
		}
		if sp.Key == "message_cache_ttl_hours" {
			rc.MessageCacheTTLHours = v
			return rc, nil
		}
		return rc, fmt.Errorf("not an int key")
	case vtDate:
		if raw == "" {
			if sp.Key == "backfill_start_day" {
				rc.BackfillStartDay = ""
				return rc, nil
			}
			if sp.Key == "backfill_initial_date" {
				rc.BackfillInitialDate = ""
				return rc, nil
			}
			return rc, nil
		}
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return rc, fmt.Errorf("invalid date (expected YYYY-MM-DD)")
		}
		if sp.Key == "backfill_start_day" {
			rc.BackfillStartDay = raw
			return rc, nil
		}
		if sp.Key == "backfill_initial_date" {
			rc.BackfillInitialDate = raw
			return rc, nil
		}
		return rc, fmt.Errorf("unsupported date key")
	case vtString:
		switch sp.Key {
		case "bot_theme":
			rc.BotTheme = raw
			return rc, nil
		case "presence_watch_user_id":
			rc.PresenceWatchUserID = raw
			return rc, nil
		case "backfill_channel_id":
			rc.BackfillChannelID = raw
			return rc, nil
		case "bot_role_perm_mirror_actor_role_id":
			rc.BotRolePermMirrorActorRoleID = raw
			return rc, nil
		}
		return rc, fmt.Errorf("unsupported string key")
	}
	return rc, fmt.Errorf("unknown type")
}

// runtimeConfigApplier interfaces the immediate application side-effects to the dependency container organically.
type runtimeConfigApplier interface {
	Apply(ctx context.Context, rc files.RuntimeConfig) error
}
