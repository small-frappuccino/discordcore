package control

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"runtime/debug"

	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
)

// generateRequestID creates a transient cryptographic identifier to correlate paginated alerts.
func generateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(bytes)
}

// emitBlockingError injects structural metadata containing the stack trace and synthetic status 500.
func emitBlockingError(msg string, err error, requestID string) {
	slog.Error(msg,
		slog.String("request_id", requestID),
		slog.String("synthetic_code", "500"),
		slog.String("stack_trace", string(debug.Stack())),
		slog.Any("error", err),
	)
}

var errUnknownFeatureID = errors.New("unknown feature id")

type featurePatchBadRequestError struct {
	message string
}

func (e featurePatchBadRequestError) Error() string {
	// Debug inspection points added directly prior to error generation
	// are omitted here as this is a strict stringification method, but
	// the construction of this struct downstream triggers Warn states.
	return e.message
}

type featurePatchPreconditionRequiredError struct {
	message string
}

func (e featurePatchPreconditionRequiredError) Error() string {
	return e.message
}

type featurePatchPreconditionFailedError struct {
	message string
}

func (e featurePatchPreconditionFailedError) Error() string {
	return e.message
}

type featureDefinition struct {
	ID                    string
	Category              string
	Label                 string
	Description           string
	Area                  featureAreaID
	Tags                  []string
	SupportsGuildOverride bool
	GlobalEditableFields  []string
	GuildEditableFields   []string
	LogEvent              logpolicy.LogEventType
}

type featureCatalogEntry struct {
	ID                    string        `json:"id"`
	Category              string        `json:"category"`
	Label                 string        `json:"label"`
	Description           string        `json:"description"`
	Area                  featureAreaID `json:"area"`
	Tags                  []string      `json:"tags,omitempty"`
	SupportsGuildOverride bool          `json:"supports_guild_override"`
	GlobalEditableFields  []string      `json:"global_editable_fields,omitempty"`
	GuildEditableFields   []string      `json:"guild_editable_fields,omitempty"`
}

type featureWorkspace struct {
	Scope    string          `json:"scope"`
	GuildID  string          `json:"guild_id,omitempty"`
	Features []featureRecord `json:"features"`
}

type featureRecord struct {
	ID                    string           `json:"id"`
	Category              string           `json:"category"`
	Label                 string           `json:"label"`
	Description           string           `json:"description"`
	Scope                 string           `json:"scope"`
	Area                  featureAreaID    `json:"area"`
	Tags                  []string         `json:"tags,omitempty"`
	SupportsGuildOverride bool             `json:"supports_guild_override"`
	OverrideState         string           `json:"override_state"`
	EffectiveEnabled      bool             `json:"effective_enabled"`
	EffectiveSource       string           `json:"effective_source"`
	ConfigVersion         int64            `json:"config_version,omitempty"`
	Readiness             string           `json:"readiness"`
	Blockers              []featureBlocker `json:"blockers,omitempty"`
	Details               *featureDetails  `json:"details,omitempty"`
	EditableFields        []string         `json:"editable_fields,omitempty"`
}

type featureBlocker struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// featureDetails is the typed payload previously carried as map[string]any. Each
// builder populates only the fields relevant to its feature; empty fields are
// dropped from the JSON via omitempty, so consumers must treat every field as
// optional.
type featureDetails struct {
	Mode                    string                      `json:"mode,omitempty"`
	RoleID                  string                      `json:"role_id,omitempty"`
	ChannelID               string                      `json:"channel_id,omitempty"`
	AllowedRoleIDs          []string                    `json:"allowed_role_ids,omitempty"`
	AllowedRoleCount        int                         `json:"allowed_role_count,omitempty"`
	RuntimeEnabled          bool                        `json:"runtime_enabled,omitempty"`
	WatchBot                bool                        `json:"watch_bot,omitempty"`
	UserID                  string                      `json:"user_id,omitempty"`
	ActorRoleID             string                      `json:"actor_role_id,omitempty"`
	RuntimeDisabled         bool                        `json:"runtime_disabled,omitempty"`
	StartDay                string                      `json:"start_day,omitempty"`
	InitialDate             string                      `json:"initial_date,omitempty"`
	ConfigEnabled           bool                        `json:"config_enabled,omitempty"`
	UpdateIntervalMins      int                         `json:"update_interval_mins,omitempty"`
	ConfiguredChannelCount  int                         `json:"configured_channel_count,omitempty"`
	Channels                []featureStatsChannelDetail `json:"channels,omitempty"`
	TargetRoleID            string                      `json:"target_role_id,omitempty"`
	RequiredRoleIDs         []string                    `json:"required_role_ids,omitempty"`
	RequiredRoleCount       int                         `json:"required_role_count,omitempty"`
	BoosterRoleID           string                      `json:"booster_role_id,omitempty"`
	LevelRoleID             string                      `json:"level_role_id,omitempty"`
	RequiresChannel         bool                        `json:"requires_channel,omitempty"`
	RequiredIntentsMask     int                         `json:"required_intents_mask,omitempty"`
	RequiredPermissionsMask int64                       `json:"required_permissions_mask,omitempty"`
	ValidateChannelPerms    bool                        `json:"validate_channel_permissions,omitempty"`
	ExclusiveModeration     bool                        `json:"exclusive_moderation_channel,omitempty"`
	RuntimeTogglePath       string                      `json:"runtime_toggle_path,omitempty"`
}

type featureStatsChannelDetail struct {
	ChannelID    string `json:"channel_id,omitempty"`
	Label        string `json:"label,omitempty"`
	NameTemplate string `json:"name_template,omitempty"`
	MemberType   string `json:"member_type,omitempty"`
	RoleID       string `json:"role_id,omitempty"`
}

var featureDefinitions = []featureDefinition{
	{ID: "services.monitoring", Category: "services", Label: "Monitoring", Description: "Core monitoring service lifecycle and shared event processing.", Area: featureAreaMaintenance, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "role_panels", Category: "roles", Label: "Role panels", Description: "Self-service role panels with toggleable buttons published to guild channels.", Area: featureAreaRoles, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled"}},
	{ID: "stats_channels", Category: "services", Label: "Stats channels", Description: "Update server stats on voice channel names.", Area: featureAreaMaintenance, SupportsGuildOverride: true, GlobalEditableFields: []string{"enabled"}, GuildEditableFields: []string{"enabled", "config_enabled", "update_interval_mins"}},
}

var featureDefinitionsByID = func() map[string]featureDefinition {
	slog.Info("Architectural state transition: Mapping feature definitions array to internal index")

	out := make(map[string]featureDefinition, len(featureDefinitions))
	for _, def := range featureDefinitions {
		out[def.ID] = def
	}

	slog.Debug("Granular inspection: Feature definition index populated",
		slog.Int("total_definitions", len(out)),
	)

	return out
}()
