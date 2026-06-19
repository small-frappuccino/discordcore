// Package files owns the canonical feature-toggle registry.
//
// This file intentionally carries only the schema-level data each
// toggle needs (ID, struct Path, Default). Product-facing metadata —
// human label, description, area, tags, editable fields, and the
// associated discord/logging LogEvent — lives in
// pkg/control/features_catalog.go (`featureDefinitions`). The split
// is deliberate: pkg/files is the lowest layer in the dependency
// graph and must not import pkg/control or pkg/discord/logging.
// Pulling UI metadata down would invert layering; introducing a
// third joining layer would just rebuild featureDefinitions under a
// different name. featureDefinitions consumes registry IDs and the
// bijection between the two is locked by a contract test in
// pkg/control/feature_contract_test.go.
package files

// toggleSpec describes one boolean feature toggle. It is the single
// source of truth that drives resolve, clone, defaults, dashboard
// binding, override detection and the per-command enabled check.
//
// Accessor functions replace reflection to ensure compile-time safety
// when interacting with FeatureToggles and ResolvedFeatureToggles.
type toggleSpec struct {
	ID          string
	Default     bool
	Get         func(ft *FeatureToggles) *bool
	Set         func(ft *FeatureToggles, val *bool)
	GetResolved func(rft *ResolvedFeatureToggles) bool
	SetResolved func(rft *ResolvedFeatureToggles, val bool)
}

var featureRegistry = []toggleSpec{
	{
		ID: "services.monitoring", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Services.Monitoring },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Services.Monitoring = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Services.Monitoring },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Services.Monitoring = val },
	},
	{
		ID: "services.commands", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Services.Commands },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Services.Commands = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Services.Commands },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Services.Commands = val },
	},

	// --------------------------------------------------------------------
	// DEPRECATED LOGGING TOGGLES
	// Logging features are implicitly enabled by the presence of a target
	// channel configuration. These registry entries are retained solely to
	// preserve JSON schema deserialization backwards-compatibility.
	// --------------------------------------------------------------------
	{
		ID: "logging.avatar_logging", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.AvatarLogging },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.AvatarLogging = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.AvatarLogging },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.AvatarLogging = val },
	},
	{
		ID: "logging.role_update", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.RoleUpdate },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.RoleUpdate = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.RoleUpdate },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.RoleUpdate = val },
	},
	{
		ID: "logging.member_join", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MemberJoin },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MemberJoin = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MemberJoin },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MemberJoin = val },
	},
	{
		ID: "logging.member_leave", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MemberLeave },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MemberLeave = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MemberLeave },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MemberLeave = val },
	},
	{
		ID: "logging.message_process", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MessageProcess },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MessageProcess = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MessageProcess },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MessageProcess = val },
	},
	{
		ID: "logging.message_edit", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MessageEdit },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MessageEdit = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MessageEdit },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MessageEdit = val },
	},
	{
		ID: "logging.message_delete", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MessageDelete },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MessageDelete = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MessageDelete },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MessageDelete = val },
	},
	{
		ID: "logging.reaction_metric", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.ReactionMetric },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.ReactionMetric = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.ReactionMetric },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.ReactionMetric = val },
	},
	{
		ID: "logging.automod_action", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.AutomodAction },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.AutomodAction = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.AutomodAction },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.AutomodAction = val },
	},
	{
		ID: "logging.moderation_case", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.ModerationCase },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.ModerationCase = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.ModerationCase },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.ModerationCase = val },
	},
	{
		ID: "logging.clean_action", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.CleanAction },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.CleanAction = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.CleanAction },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.CleanAction = val },
	},
	// --------------------------------------------------------------------
	{
		ID: "moderation.ban", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Ban },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Ban = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Ban },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Ban = val },
	},
	{
		ID: "moderation.massban", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.MassBan },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.MassBan = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.MassBan },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.MassBan = val },
	},
	{
		ID: "moderation.kick", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Kick },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Kick = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Kick },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Kick = val },
	},
	{
		ID: "moderation.timeout", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Timeout },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Timeout = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Timeout },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Timeout = val },
	},
	{
		ID: "moderation.warn", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Warn },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Warn = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Warn },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Warn = val },
	},
	{
		ID: "moderation.warnings", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Warnings },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Warnings = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Warnings },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Warnings = val },
	},
	{
		ID: "moderation.clean", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Clean },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Clean = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Clean },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Clean = val },
	},
	{
		ID: "message_cache.cleanup_on_startup", Default: false,
		Get:         func(ft *FeatureToggles) *bool { return ft.MessageCache.CleanupOnStartup },
		Set:         func(ft *FeatureToggles, val *bool) { ft.MessageCache.CleanupOnStartup = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.MessageCache.CleanupOnStartup },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.MessageCache.CleanupOnStartup = val },
	},
	{
		ID: "message_cache.delete_on_log", Default: false,
		Get:         func(ft *FeatureToggles) *bool { return ft.MessageCache.DeleteOnLog },
		Set:         func(ft *FeatureToggles, val *bool) { ft.MessageCache.DeleteOnLog = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.MessageCache.DeleteOnLog },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.MessageCache.DeleteOnLog = val },
	},
	{
		ID: "presence_watch.bot", Default: false,
		Get:         func(ft *FeatureToggles) *bool { return ft.PresenceWatch.Bot },
		Set:         func(ft *FeatureToggles, val *bool) { ft.PresenceWatch.Bot = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.PresenceWatch.Bot },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.PresenceWatch.Bot = val },
	},
	{
		ID: "presence_watch.user", Default: false,
		Get:         func(ft *FeatureToggles) *bool { return ft.PresenceWatch.User },
		Set:         func(ft *FeatureToggles, val *bool) { ft.PresenceWatch.User = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.PresenceWatch.User },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.PresenceWatch.User = val },
	},
	{
		ID: "maintenance.db_cleanup", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Maintenance.DBCleanup },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Maintenance.DBCleanup = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Maintenance.DBCleanup },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Maintenance.DBCleanup = val },
	},
	{
		ID: "safety.bot_role_perm_mirror", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Safety.BotRolePermMirror },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Safety.BotRolePermMirror = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Safety.BotRolePermMirror },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Safety.BotRolePermMirror = val },
	},

	{
		ID: "moderation.mute_role", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.MuteRole },
		Set:         func(ft *FeatureToggles, val *bool) { ft.MuteRole = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.MuteRole },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.MuteRole = val },
	},

	{
		ID: "role_panels", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.RolePanels },
		Set:         func(ft *FeatureToggles, val *bool) { ft.RolePanels = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.RolePanels },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.RolePanels = val },
	},
}

var featureSpecByID = func() map[string]toggleSpec {
	out := make(map[string]toggleSpec, len(featureRegistry))
	for _, spec := range featureRegistry {
		out[spec.ID] = spec
	}
	return out
}()

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
	return cloneBoolPtr(spec.Get(&ft))
}

// SetToggle writes value into the registered toggle. Unknown IDs are
// ignored. The value pointer is copied; callers may reuse it.
func (ft *FeatureToggles) SetToggle(id string, value *bool) {
	spec, ok := featureSpecByID[id]
	if !ok {
		return
	}
	spec.Set(ft, value)
}

// HasAnyOverride reports whether any registered toggle field is set.
// Non-toggle fields on FeatureToggles are not considered.
func (ft FeatureToggles) HasAnyOverride() bool {
	for _, spec := range featureRegistry {
		ptr := spec.Get(&ft)
		if ptr != nil {
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
	return spec.GetResolved(&rft), true
}
