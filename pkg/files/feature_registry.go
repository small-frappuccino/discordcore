package files

import (
	"reflect"
	"strings"
)

// toggleSpec describes one boolean feature toggle. It is the single
// source of truth that drives resolve, clone, defaults, dashboard
// binding, override detection and the per-command enabled check.
//
// Path is a dotted accessor that resolves to a *bool field on
// FeatureToggles and to a bool field on ResolvedFeatureToggles. The
// matching field names on both structs must be identical, e.g.
// "Moderation.Clean" or single-segment "MuteRole".
type toggleSpec struct {
	ID      string
	Path    string
	Default bool
}

var featureRegistry = []toggleSpec{
	{ID: "services.monitoring", Path: "Services.Monitoring", Default: true},
	{ID: "services.automod", Path: "Services.Automod", Default: true},
	{ID: "services.commands", Path: "Services.Commands", Default: true},
	{ID: "services.admin_commands", Path: "Services.AdminCommands", Default: true},
	{ID: "logging.avatar_logging", Path: "Logging.AvatarLogging", Default: true},
	{ID: "logging.role_update", Path: "Logging.RoleUpdate", Default: true},
	{ID: "logging.member_join", Path: "Logging.MemberJoin", Default: true},
	{ID: "logging.member_leave", Path: "Logging.MemberLeave", Default: true},
	{ID: "logging.message_process", Path: "Logging.MessageProcess", Default: true},
	{ID: "logging.message_edit", Path: "Logging.MessageEdit", Default: true},
	{ID: "logging.message_delete", Path: "Logging.MessageDelete", Default: true},
	{ID: "logging.reaction_metric", Path: "Logging.ReactionMetric", Default: true},
	{ID: "logging.automod_action", Path: "Logging.AutomodAction", Default: true},
	{ID: "logging.moderation_case", Path: "Logging.ModerationCase", Default: true},
	{ID: "logging.clean_action", Path: "Logging.CleanAction", Default: true},
	{ID: "moderation.ban", Path: "Moderation.Ban", Default: true},
	{ID: "moderation.massban", Path: "Moderation.MassBan", Default: true},
	{ID: "moderation.kick", Path: "Moderation.Kick", Default: true},
	{ID: "moderation.timeout", Path: "Moderation.Timeout", Default: true},
	{ID: "moderation.warn", Path: "Moderation.Warn", Default: true},
	{ID: "moderation.warnings", Path: "Moderation.Warnings", Default: true},
	{ID: "moderation.clean", Path: "Moderation.Clean", Default: true},
	{ID: "message_cache.cleanup_on_startup", Path: "MessageCache.CleanupOnStartup", Default: false},
	{ID: "message_cache.delete_on_log", Path: "MessageCache.DeleteOnLog", Default: false},
	{ID: "presence_watch.bot", Path: "PresenceWatch.Bot", Default: false},
	{ID: "presence_watch.user", Path: "PresenceWatch.User", Default: false},
	{ID: "maintenance.db_cleanup", Path: "Maintenance.DBCleanup", Default: true},
	{ID: "safety.bot_role_perm_mirror", Path: "Safety.BotRolePermMirror", Default: true},
	{ID: "backfill.enabled", Path: "Backfill.Enabled", Default: true},
	{ID: "moderation.mute_role", Path: "MuteRole", Default: true},
	{ID: "stats_channels", Path: "StatsChannels", Default: true},
	{ID: "auto_role_assignment", Path: "AutoRoleAssign", Default: true},
	{ID: "user_prune", Path: "UserPrune", Default: true},
}

var featureSpecByID = func() map[string]toggleSpec {
	out := make(map[string]toggleSpec, len(featureRegistry))
	for _, spec := range featureRegistry {
		out[spec.ID] = spec
	}
	return out
}()

func walkFieldByPath(root reflect.Value, path string) reflect.Value {
	current := root
	for _, segment := range strings.Split(path, ".") {
		current = current.FieldByName(segment)
		if !current.IsValid() {
			return reflect.Value{}
		}
	}
	return current
}

// togglePtrValue returns the addressable *bool field on FeatureToggles
// for the given dotted path. The returned reflect.Value has
// Kind() == reflect.Pointer with elem kind bool.
func togglePtrValue(ft *FeatureToggles, path string) reflect.Value {
	return walkFieldByPath(reflect.ValueOf(ft).Elem(), path)
}

// resolvedToggleValue returns the addressable bool field on
// ResolvedFeatureToggles for the given dotted path.
func resolvedToggleValue(rft *ResolvedFeatureToggles, path string) reflect.Value {
	return walkFieldByPath(reflect.ValueOf(rft).Elem(), path)
}

// FeatureToggleIDs returns the list of registered toggle IDs in
// declaration order.
func FeatureToggleIDs() []string {
	out := make([]string, len(featureRegistry))
	for i, spec := range featureRegistry {
		out[i] = spec.ID
	}
	return out
}

// FeatureToggleSpec looks up a registered toggle by ID.
func FeatureToggleSpec(id string) (toggleSpec, bool) {
	spec, ok := featureSpecByID[id]
	return spec, ok
}

// LookupToggle returns the *bool stored under the given toggle ID,
// or nil when the toggle is unset or the ID is not registered.
func (ft FeatureToggles) LookupToggle(id string) *bool {
	spec, ok := featureSpecByID[id]
	if !ok {
		return nil
	}
	field := togglePtrValue(&ft, spec.Path)
	if !field.IsValid() || field.IsNil() {
		return nil
	}
	v := field.Elem().Bool()
	return &v
}

// SetToggle writes value into the registered toggle. Unknown IDs are
// ignored. The value pointer is copied; callers may reuse it.
func (ft *FeatureToggles) SetToggle(id string, value *bool) {
	spec, ok := featureSpecByID[id]
	if !ok {
		return
	}
	field := togglePtrValue(ft, spec.Path)
	if !field.IsValid() {
		return
	}
	if value == nil {
		field.Set(reflect.Zero(field.Type()))
		return
	}
	copied := *value
	field.Set(reflect.ValueOf(&copied))
}

// HasAnyOverride reports whether any registered toggle field is set.
// Non-toggle fields on FeatureToggles are not considered.
func (ft FeatureToggles) HasAnyOverride() bool {
	for _, spec := range featureRegistry {
		field := togglePtrValue(&ft, spec.Path)
		if field.IsValid() && !field.IsNil() {
			return true
		}
	}
	return false
}

// Lookup returns the resolved bool for the given toggle ID and a
// flag indicating whether the ID is registered.
func (rft ResolvedFeatureToggles) Lookup(id string) (bool, bool) {
	spec, ok := featureSpecByID[id]
	if !ok {
		return false, false
	}
	field := resolvedToggleValue(&rft, spec.Path)
	if !field.IsValid() {
		return false, false
	}
	return field.Bool(), true
}
