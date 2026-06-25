# Domain Architecture: members

## Layout Topology
```text
members/
├── auto_role.go
├── intents.go
├── member_events.go
├── models.go
├── observability.go
├── repository.go
└── sink.go
```

## Source Stream Aggregation

// === FILE: pkg/members/auto_role.go ===
```go
package members

type autoRoleDecision int

const (
	autoRoleNoop autoRoleDecision = iota
	AutoRoleAddTarget
	AutoRoleRemoveTarget
)

// hasRoleID checks if a role ID is present in a member role list.
func hasRoleID(roles []string, roleID string) bool {
	if roleID == "" {
		return false
	}
	for _, rid := range roles {
		if rid == roleID {
			return true
		}
	}
	return false
}

// EvaluateAutoRoleDecision centralizes the auto-assignment rule used by realtime
// member updates and periodic reconciliation.
//
// Ordering contract for requiredRoles:
// - requiredRoles[0] is roleA (stable level role, e.g. Arcane lvl 20).
// - requiredRoles[1] is roleB (volatile booster role, can be gained/lost).
//
// Business rule:
// - Add target role when member has both roleA and roleB and target is missing.
// - Remove target role when target exists but roleA is missing.
//
// Even if roleA is expected to be stable, we keep the removal rule as a safety
// self-heal for manual edits and stale states.
func EvaluateAutoRoleDecision(memberRoles []string, targetRoleID string, requiredRoles []string) autoRoleDecision {
	if targetRoleID == "" || len(requiredRoles) < 2 {
		return autoRoleNoop
	}

	roleA := requiredRoles[0]
	roleB := requiredRoles[1]

	hasTarget := hasRoleID(memberRoles, targetRoleID)
	hasA := hasRoleID(memberRoles, roleA)
	hasB := hasRoleID(memberRoles, roleB)

	if hasTarget && !hasA {
		return AutoRoleRemoveTarget
	}
	if !hasTarget && hasA && hasB {
		return AutoRoleAddTarget
	}
	return autoRoleNoop
}

```

// === FILE: pkg/members/intents.go ===
```go
package members

import "time"

// MemberJoinIntent represents a user joining a guild.
type MemberJoinIntent struct {
	GuildID    string
	UserID     string
	Username   string
	Bot        bool
	AvatarHash string
	RoleIDs    []string
	JoinedAt   time.Time
}

// MemberLeaveIntent represents a user leaving a guild.
type MemberLeaveIntent struct {
	GuildID    string
	UserID     string
	Username   string
	Bot        bool
	AvatarHash string
}

// RoleUpdateIntent represents a role update for a member.
type RoleUpdateIntent struct {
	GuildID      string
	UserID       string
	Username     string
	Bot          bool
	AddedRoles   []string
	RemovedRoles []string
}

// AvatarUpdateIntent represents a change in the user's avatar.
type AvatarUpdateIntent struct {
	GuildID       string
	UserID        string
	Username      string
	Bot           bool
	OldAvatarHash string
	NewAvatarHash string
}

// ModerationActionIntent represents an action applied to a member.
type ModerationActionIntent struct {
	GuildID        string
	ActionType     string
	TargetUserID   string
	TargetUsername string
	TargetBot      bool
	Reason         string
	ModeratorID    string
}

// MemberUpdateIntent represents a raw member update event for ingestion.
type MemberUpdateIntent struct {
	GuildID    string
	UserID     string
	Username   string
	Bot        bool
	RoleIDs    []string
	AvatarHash string
	OldRoleIDs []string
	OldAvatar  string
}

```

// === FILE: pkg/members/member_events.go ===
```go
package members

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logging"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/system"
)

// Hardcoded IDs for automatic role assignment
const unknownServerTimeSentinel time.Duration = -1

// DiscordAdapter provides a pure domain interface for Discord API operations
// without leaking the underlying gateway or state SDK types.
type DiscordAdapter interface {
	Me() (string, error)
	MemberJoinedAt(ctx context.Context, guildID, userID string) (time.Time, error)
	AddRole(ctx context.Context, guildID, userID, roleID string) error
	RemoveRole(ctx context.Context, guildID, userID, roleID string) error
}

// MemberEventService manages member join/leave events
type MemberEventService struct {
	configManager *files.ConfigManager
	botInstanceID string
	sink          MemberSink
	activity      *service.RuntimeActivity
	lifecycle     service.BaseLifecycle
	logger        *slog.Logger

	// Cache for join times (member and bot)
	joinTimes map[string]time.Time // key: guildID:userID
	joinMu    sync.Mutex

	membersRepo Repository
	systemRepo  system.Repository

	discordAdapter DiscordAdapter
}

// EventServiceDeps bundles the shared dependencies for the bot-scoped logging
// event services. BotInstanceID is normalized by the
// constructors via files.NormalizeBotInstanceID.
type EventServiceDeps struct {
	ConfigManager  *files.ConfigManager
	Sink           MemberSink
	MembersRepo    Repository
	SystemRepo     system.Repository
	BotInstanceID  string
	Logger         *slog.Logger
	DiscordAdapter DiscordAdapter
}

// NewMemberEventService creates a new instance of the member events service
func NewMemberEventService(configManager *files.ConfigManager, sink MemberSink, membersRepo Repository, systemRepo system.Repository, logger *slog.Logger) *MemberEventService {
	return NewMemberEventServiceForBot(EventServiceDeps{
		ConfigManager:  configManager,
		Sink:           sink,
		MembersRepo:    membersRepo,
		SystemRepo:     systemRepo,
		Logger:         logger,
		DiscordAdapter: nil, // Fallback if no discord adapter
	})
}

// NewMemberEventServiceForBot creates a member event service scoped to one bot instance.
func NewMemberEventServiceForBot(deps EventServiceDeps) *MemberEventService {
	return &MemberEventService{
		configManager: deps.ConfigManager,
		botInstanceID: files.NormalizeBotInstanceID(deps.BotInstanceID),
		sink:          deps.Sink,
		membersRepo:   deps.MembersRepo,
		systemRepo:    deps.SystemRepo,
		logger:        deps.Logger,
		activity: service.NewRuntimeActivity(deps.SystemRepo, service.RuntimeActivityOptions{
			RunErr:        service.RunErrWithTimeoutContext,
			EventTimeout:  service.DependencyTimeout,
			BotInstanceID: files.NormalizeBotInstanceID(deps.BotInstanceID),
			Logger:        deps.Logger,
		}),
		lifecycle:      service.NewBaseLifecycle("member event service"),
		discordAdapter: deps.DiscordAdapter,
	}
}

// Start registers member event handlers
func (mes *MemberEventService) Start(ctx context.Context) error {
	_, err := mes.lifecycle.Start(ctx)
	if err != nil {
		return fmt.Errorf("MemberEventService.Start: %w", err)
	}

	// Ensure join map is initialized
	if mes.joinTimes == nil {
		mes.joinTimes = make(map[string]time.Time)
	}

	// Handlers are managed externally

	cleanupCtx, done, ok := mes.lifecycle.Begin()
	if !ok {
		mes.lifecycle.Cancel()
		return fmt.Errorf("member event service cleanup worker failed to start")
	}
	go func() {
		defer done()
		mes.cleanupLoop(cleanupCtx)
	}()

	mes.logger.Info("Member event service started")
	return nil
}

// Stop the service
func (mes *MemberEventService) Stop(ctx context.Context) error {
	if err := mes.lifecycle.Cancel(); err != nil {
		return fmt.Errorf("MemberEventService.Stop: %w", err)
	}

	if err := mes.lifecycle.Wait(ctx); err != nil {
		return fmt.Errorf("MemberEventService.Stop: %w", err)
	}

	mes.logger.Info("Member event service stopped")
	return nil
}

// IsRunning returns whether the service is running
func (mes *MemberEventService) IsRunning() bool {
	return mes.lifecycle.IsRunning()
}

// Dependencies returns the service dependencies.
func (mes *MemberEventService) Dependencies() []string {
	return nil
}

// Name returns the service name.
func (mes *MemberEventService) Name() string {
	return "member_events_" + mes.botInstanceID
}

// Type returns the service type.
func (mes *MemberEventService) Type() service.ServiceType {
	return service.TypeMonitoring
}

// Priority returns the startup priority.
func (mes *MemberEventService) Priority() service.ServicePriority {
	return service.PriorityNormal
}

// HealthCheck returns the health status of the service.
func (mes *MemberEventService) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{Healthy: true, Message: "OK", LastCheck: time.Now()}
}

// Stats returns runtime statistics.
func (mes *MemberEventService) Stats() service.ServiceStats {
	return service.ServiceStats{}
}

// IngestGuildMemberAdd processes when a user joins the server
func (mes *MemberEventService) IngestGuildMemberAdd(ctx context.Context, m MemberJoinIntent) {
	if m.UserID == "" || m.Bot {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_add",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.UserID),
	)
	defer done()

	mes.markEvent(ctx)
	if mes.configManager == nil {
		return
	}
	if !mes.handlesGuild(m.GuildID) {
		return
	}
	cfg := mes.configManager.Config()
	if cfg == nil {
		return
	}
	guildConfig := mes.configManager.GuildConfig(m.GuildID)
	if guildConfig == nil {
		return
	}

	// Composite automatic role assignment (per-guild config).
	if mes.discordAdapter != nil && guildConfig.Roles.AutoAssignment.Enabled {
		targetRoleID := guildConfig.Roles.AutoAssignment.TargetRoleID
		required := guildConfig.Roles.AutoAssignment.RequiredRoles
		if targetRoleID != "" && len(required) >= 2 {
			if EvaluateAutoRoleDecision(m.RoleIDs, targetRoleID, required) == AutoRoleAddTarget {
				if err := mes.guildMemberRoleAdd(ctx, m.GuildID, m.UserID, targetRoleID); err != nil {
					mes.logger.Error("Failed to grant target role on join", "guildID", m.GuildID, "userID", m.UserID, "roleID", targetRoleID, "error", err)
				} else {
					mes.logger.Info("Granted target role on join", "guildID", m.GuildID, "userID", m.UserID, "roleID", targetRoleID)
				}
			}
		}
	}

	// Logging is now delegated to Sink
	emit := logging.CheckFeatureEnabled(mes.configManager, logging.LogEventMemberJoin, m.GuildID)
	if !emit.Enabled {
		if emit.Reason == logging.EmitReasonNoChannelConfigured {
			mes.logger.Info("User entry/leave channel not configured for guild, member join notification not sent", "guildID", m.GuildID, "userID", m.UserID)
		} else {
			mes.logger.Debug("Member join notification suppressed by policy", "guildID", m.GuildID, "userID", m.UserID, "reason", emit.Reason)
		}
		return
	}

	// Calculate how long the account has existed
	accountAge := mes.calculateAccountAge(m.UserID)

	joinedAt := m.JoinedAt

	// Persist absolute join time to Postgres store (best effort)
	if mes.membersRepo != nil && mes.systemRepo != nil && !joinedAt.IsZero() {
		if err := service.RunErrWithTimeoutContext(ctx, service.DependencyTimeout, func(runCtx context.Context) error {
			return mes.membersRepo.UpsertMemberJoinContext(runCtx, m.GuildID, m.UserID, joinedAt)
		}); err != nil {
			mes.logger.Warn("Failed to persist member join timestamp", "guildID", m.GuildID, "userID", m.UserID, "joinedAt", joinedAt, "error", err)
		}
		if err := service.RunErrWithTimeoutContext(ctx, service.DependencyTimeout, func(runCtx context.Context) error {
			return mes.systemRepo.IncrementDailyMemberJoinContext(runCtx, m.GuildID, m.UserID, joinedAt)
		}); err != nil {
			mes.logger.Warn("Failed to increment daily member join metric", "guildID", m.GuildID, "userID", m.UserID, "joinedAt", joinedAt, "error", err)
		}
	}

	// Register precise member join timestamp in memory
	if !joinedAt.IsZero() {
		mes.joinMu.Lock()
		if mes.joinTimes == nil {
			mes.joinTimes = make(map[string]time.Time)
		}
		mes.joinTimes[m.GuildID+":"+m.UserID] = joinedAt
		mes.joinMu.Unlock()
	}

	mes.logger.Info("Member joined guild", "guildID", m.GuildID, "userID", m.UserID, "username", m.Username, "accountAge", accountAge.String())

	if mes.sink != nil {
		mes.sink.OnMemberJoin(ctx, m, accountAge)
	}

}

// handleGuildMemberRemove processes when a user leaves the server
func (mes *MemberEventService) IngestGuildMemberRemove(ctx context.Context, m MemberLeaveIntent) {
	if m.UserID == "" || m.Bot {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_remove",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.UserID),
	)
	defer done()

	mes.markEvent(ctx)
	if mes.configManager == nil {
		return
	}
	if !mes.handlesGuild(m.GuildID) {
		return
	}
	cfg := mes.configManager.Config()
	if cfg == nil {
		return
	}

	// Calculate how long they were in the server
	serverTime, hasServerTime, serverTimeErr := mes.calculateServerTime(ctx, m.GuildID, m.UserID)
	serverTimeForNotification := serverTime
	serverTimeForLog := "N/A"
	if serverTimeErr != nil {
		serverTimeForNotification = unknownServerTimeSentinel
	} else if hasServerTime {
		serverTimeForLog = serverTime.String()
	} else {
		serverTimeForLog = "unknown"
	}

	botTime := mes.getBotTimeOnServer(ctx, m.GuildID)

	// Increment daily member leave metric
	if mes.systemRepo != nil {
		if err := service.RunErrWithTimeoutContext(ctx, service.DependencyTimeout, func(runCtx context.Context) error {
			return mes.systemRepo.IncrementDailyMemberLeaveContext(runCtx, m.GuildID, m.UserID, time.Now().UTC())
		}); err != nil {
			mes.logger.Warn("Failed to increment daily member leave metric", "guildID", m.GuildID, "userID", m.UserID, "error", err)
		}
	}

	mes.logger.Info("Member left guild", "guildID", m.GuildID, "userID", m.UserID, "username", m.Username, "serverTime", serverTimeForLog, "botTime", botTime.String())

	if mes.sink != nil {
		mes.sink.OnMemberLeave(ctx, m, serverTimeForNotification, botTime)
	}
}

// handleGuildMemberUpdate maintains the role relationship:
// - If the user loses role A, remove the target role.
// - If the user has both A and B, grant the target role (if not already present).
// It also tracks role changes and avatar updates to dispatch to MemberSink.
func (mes *MemberEventService) IngestGuildMemberUpdate(ctx context.Context, m MemberUpdateIntent) {
	if m.UserID == "" || m.Bot {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_update",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.UserID),
	)
	defer done()

	if mes.configManager == nil {
		return
	}
	if !mes.handlesGuild(m.GuildID) {
		return
	}
	cfg := mes.configManager.Config()
	if cfg == nil {
		return
	}
	guildConfig := mes.configManager.GuildConfig(m.GuildID)
	if guildConfig == nil || !guildConfig.Roles.AutoAssignment.Enabled {
		return
	}

	targetRoleID := guildConfig.Roles.AutoAssignment.TargetRoleID
	required := guildConfig.Roles.AutoAssignment.RequiredRoles
	if targetRoleID == "" || len(required) < 2 {
		return
	}

	switch EvaluateAutoRoleDecision(m.RoleIDs, targetRoleID, required) {
	case AutoRoleRemoveTarget:
		if err := mes.guildMemberRoleRemove(ctx, m.GuildID, m.UserID, targetRoleID); err != nil {
			mes.logger.Error("Failed to remove target role on update", "guildID", m.GuildID, "userID", m.UserID, "roleID", targetRoleID, "error", err)
		} else {
			mes.logger.Info("Removed target role on update", "guildID", m.GuildID, "userID", m.UserID, "roleID", targetRoleID)
		}
	case AutoRoleAddTarget:
		if err := mes.guildMemberRoleAdd(ctx, m.GuildID, m.UserID, targetRoleID); err != nil {
			mes.logger.Error("Failed to grant target role on update", "guildID", m.GuildID, "userID", m.UserID, "roleID", targetRoleID, "error", err)
		} else {
			mes.logger.Info("Granted target role on update", "guildID", m.GuildID, "userID", m.UserID, "roleID", targetRoleID)
		}
	}

	if mes.sink != nil {
		// Compare roles
		var addedRoles, removedRoles []string
		oldRolesMap := make(map[string]bool, len(m.OldRoleIDs))
		for _, r := range m.OldRoleIDs {
			oldRolesMap[r] = true
		}
		newRolesMap := make(map[string]bool, len(m.RoleIDs))
		for _, r := range m.RoleIDs {
			newRolesMap[r] = true
			if !oldRolesMap[r] {
				addedRoles = append(addedRoles, r)
			}
		}
		for r := range oldRolesMap {
			if !newRolesMap[r] {
				removedRoles = append(removedRoles, r)
			}
		}

		if len(addedRoles) > 0 || len(removedRoles) > 0 {
			mes.sink.OnRoleUpdate(ctx, RoleUpdateIntent{
				GuildID:      m.GuildID,
				UserID:       m.UserID,
				Username:     m.Username,
				Bot:          m.Bot,
				AddedRoles:   addedRoles,
				RemovedRoles: removedRoles,
			})
		}

		// Compare avatar
		if m.OldAvatar != m.AvatarHash {
			mes.sink.OnAvatarUpdate(ctx, AvatarUpdateIntent{
				GuildID:       m.GuildID,
				UserID:        m.UserID,
				Username:      m.Username,
				Bot:           m.Bot,
				OldAvatarHash: m.OldAvatar,
				NewAvatarHash: m.AvatarHash,
			})
		}
	}
}

// calculateAccountAge calculates how long the Discord account has existed based on the Snowflake ID
func (mes *MemberEventService) calculateAccountAge(userID string) time.Duration {
	// Discord Snowflake: (timestamp_ms - DISCORD_EPOCH) << 22
	const DISCORD_EPOCH = 1420070400000 // 01/01/2015 00:00:00 UTC in milliseconds

	// Convert string ID to uint64
	snowflake, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		mes.logger.Warn(fmt.Sprintf("Failed to parse user ID for account age calculation: userID=%s, error=%v", userID, err))
		return 0
	}

	// Extract timestamp from the snowflake
	timestamp := (snowflake >> 22) + DISCORD_EPOCH
	accountCreated := time.Unix(int64(timestamp/1000), int64((timestamp%1000)*1000000))

	return time.Since(accountCreated)
}

// calculateServerTime tries to estimate how long the user was on the server.
func (mes *MemberEventService) calculateServerTime(ctx context.Context, guildID, userID string) (time.Duration, bool, error) {
	// 1) memory (most precise during runtime)
	mes.joinMu.Lock()
	t, ok := mes.joinTimes[guildID+":"+userID]
	mes.joinMu.Unlock()
	if ok && !t.IsZero() {
		return time.Since(t), true, nil
	}

	// 3) Persistent store (Postgres)
	if mes.systemRepo != nil {
		type joinLookup struct {
			at time.Time
			ok bool
		}
		res, err := service.RunWithTimeoutContext(ctx, service.DependencyTimeout, func(runCtx context.Context) (joinLookup, error) {
			at, ok, err := mes.membersRepo.MemberJoin(runCtx, guildID, userID)
			return joinLookup{at: at, ok: ok}, err
		})
		if err != nil {
			mes.logger.Warn("Failed to read member join timestamp from store; time on server unavailable", "guildID", guildID, "userID", userID, "error", err)
			return 0, false, fmt.Errorf("get member join from store: %w", err)
		}
		if res.ok && !res.at.IsZero() {
			return time.Since(res.at), true, nil
		}
	}
	return 0, false, nil
}

// cleanupLoop periodically removes old entries from joinTimes map
func (mes *MemberEventService) cleanupLoop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			mes.logger.Error("MemberEventService cleanup loop panic caught", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mes.cleanupJoinTimes()
		case <-ctx.Done():
			return
		}
	}
}

// cleanupJoinTimes removes entries older than 7 days from joinTimes map
func (mes *MemberEventService) cleanupJoinTimes() {
	if mes.joinTimes == nil {
		return
	}

	now := time.Now()
	threshold := 7 * 24 * time.Hour
	var toDelete []string

	// Collect keys to delete (can't delete while iterating)
	mes.joinMu.Lock()
	for key, joinTime := range mes.joinTimes {
		if now.Sub(joinTime) > threshold {
			toDelete = append(toDelete, key)
		}
	}
	mes.joinMu.Unlock()

	// Delete old entries
	if len(toDelete) > 0 {
		mes.joinMu.Lock()
		for _, key := range toDelete {
			delete(mes.joinTimes, key)
		}
		mes.joinMu.Unlock()
	}

	if len(toDelete) > 0 {
		mes.logger.Info("Cleaned up old join time entries from memory", slog.Int("count", len(toDelete)))
	}
}

func (mes *MemberEventService) markEvent(ctx context.Context) {
	if mes.activity == nil {
		return
	}
	mes.activity.MarkEvent(ctx, "member_event_service")
}

// NEW: calculates how long the bot has been in the guild (real-time Discord query)
func (mes *MemberEventService) getBotTimeOnServer(ctx context.Context, guildID string) time.Duration {
	if mes.discordAdapter == nil {
		return 0
	}
	botID, err := mes.discordAdapter.Me()
	if err != nil || botID == "" {
		return 0
	}
	joinedAt, err := mes.getGuildMemberJoinedAt(ctx, guildID, botID)
	if err != nil || joinedAt.IsZero() {
		return 0
	}
	return time.Since(joinedAt)
}

func (mes *MemberEventService) getGuildMemberJoinedAt(ctx context.Context, guildID, userID string) (time.Time, error) {
	if mes.discordAdapter == nil {
		return time.Time{}, fmt.Errorf("discord adapter is nil")
	}
	return service.RunWithTimeoutContext(ctx, service.DependencyTimeout, func(runCtx context.Context) (time.Time, error) {
		return mes.discordAdapter.MemberJoinedAt(runCtx, guildID, userID)
	})
}

func (mes *MemberEventService) guildMemberRoleAdd(ctx context.Context, guildID, userID, roleID string) error {
	if mes.discordAdapter == nil {
		return fmt.Errorf("discord adapter is nil")
	}
	return service.RunErrWithTimeoutContext(ctx, service.DependencyTimeout, func(runCtx context.Context) error {
		return mes.discordAdapter.AddRole(runCtx, guildID, userID, roleID)
	})
}

func (mes *MemberEventService) guildMemberRoleRemove(ctx context.Context, guildID, userID, roleID string) error {
	if mes.discordAdapter == nil {
		return fmt.Errorf("discord adapter is nil")
	}
	return service.RunErrWithTimeoutContext(ctx, service.DependencyTimeout, func(runCtx context.Context) error {
		return mes.discordAdapter.RemoveRole(runCtx, guildID, userID, roleID)
	})
}

func (mes *MemberEventService) handlesGuild(guildID string) bool {
	if mes == nil || mes.configManager == nil {
		return false
	}
	if files.NormalizeBotInstanceID(mes.botInstanceID) == "" {
		return true
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false
	}
	guild := mes.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}
	if !files.BelongsToBotInstance(*guild, mes.botInstanceID) {
		return false
	}
	rolesResolvedID, _ := files.ResolveFeatureBotInstanceID(*guild, "roles")
	loggingResolvedID, _ := files.ResolveFeatureBotInstanceID(*guild, "logging")
	return rolesResolvedID == mes.botInstanceID || loggingResolvedID == mes.botInstanceID
}

```

// === FILE: pkg/members/models.go ===
```go
package members

import "time"

// Snapshot represents the persisted snapshot for one guild member.
type Snapshot struct {
	UserID     string
	AvatarHash string
	HasAvatar  bool
	Roles      []string
	HasRoles   bool
	JoinedAt   time.Time
	IsBot      bool
	HasBot     bool
}

// CurrentState is the persisted current membership state for a user.
type CurrentState struct {
	UserID     string
	JoinedAt   time.Time
	LastSeenAt time.Time
	LeftAt     time.Time
	Active     bool
	IsBot      bool
	HasBot     bool
	Roles      []string
}

// UserPreferences represents user-specific settings.
type UserPreferences struct {
	UserID   string `json:"user_id"`
	Theme    string `json:"theme"`
	Timezone string `json:"timezone"`
}

// PresenceInput describes a member presence upsert payload.
type PresenceInput struct {
	GuildID  string
	UserID   string
	JoinedAt time.Time
	SeenAt   time.Time
	IsBot    bool
}

```

// === FILE: pkg/members/observability.go ===
```go
package members

import (
	"sync/atomic"
)

// Metrics is the observability seam the members service writes through.
type Metrics interface {
	RecordGuildMemberCall()
	RecordStateMemberCacheHit()
	RecordRolesCacheMemoryHit()
	RecordRolesCacheStoreHit()
	RecordRolesAuditCacheHit()
	RecordAuditLogCall()
}

// SnapshotProvider is the optional capability the /v1/health/members handler looks for.
type SnapshotProvider interface {
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot is the JSON payload /v1/health/members returns.
type MetricsSnapshot struct {
	GuildMemberCallsTotal int64 `json:"guild_member_calls_total"`
	StateMemberHitsTotal  int64 `json:"state_member_hits_total"`
	RolesMemoryHitsTotal  int64 `json:"roles_memory_hits_total"`
	RolesStoreHitsTotal   int64 `json:"roles_store_hits_total"`
	RolesAuditHitsTotal   int64 `json:"roles_audit_hits_total"`
	AuditLogCallsTotal    int64 `json:"audit_log_calls_total"`
}

// NopMetrics is the default implementation when the service is constructed without explicit metrics wiring.
type NopMetrics struct{}

func (NopMetrics) RecordGuildMemberCall()     {}
func (NopMetrics) RecordStateMemberCacheHit() {}
func (NopMetrics) RecordRolesCacheMemoryHit() {}
func (NopMetrics) RecordRolesCacheStoreHit()  {}
func (NopMetrics) RecordRolesAuditCacheHit()  {}
func (NopMetrics) RecordAuditLogCall()        {}

// InMemoryMetrics is the lightweight implementation backing /v1/health/members.
type InMemoryMetrics struct {
	guildMemberCalls atomic.Int64
	stateMemberHits  atomic.Int64
	rolesMemoryHits  atomic.Int64
	rolesStoreHits   atomic.Int64
	rolesAuditHits   atomic.Int64
	auditLogCalls    atomic.Int64
}

// NewInMemoryMetrics constructs the production metrics implementation.
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{}
}

func (m *InMemoryMetrics) RecordGuildMemberCall()     { m.guildMemberCalls.Add(1) }
func (m *InMemoryMetrics) RecordStateMemberCacheHit() { m.stateMemberHits.Add(1) }
func (m *InMemoryMetrics) RecordRolesCacheMemoryHit() { m.rolesMemoryHits.Add(1) }
func (m *InMemoryMetrics) RecordRolesCacheStoreHit()  { m.rolesStoreHits.Add(1) }
func (m *InMemoryMetrics) RecordRolesAuditCacheHit()  { m.rolesAuditHits.Add(1) }
func (m *InMemoryMetrics) RecordAuditLogCall()        { m.auditLogCalls.Add(1) }

// Snapshot returns a JSON-friendly view of the current counter state.
func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		GuildMemberCallsTotal: m.guildMemberCalls.Load(),
		StateMemberHitsTotal:  m.stateMemberHits.Load(),
		RolesMemoryHitsTotal:  m.rolesMemoryHits.Load(),
		RolesStoreHitsTotal:   m.rolesStoreHits.Load(),
		RolesAuditHitsTotal:   m.rolesAuditHits.Load(),
		AuditLogCallsTotal:    m.auditLogCalls.Load(),
	}
}

```

// === FILE: pkg/members/repository.go ===
```go
package members

import (
	"context"
	"iter"
	"time"
)

// Repository encapsulates the persistent storage logic for the members domain.
type Repository interface {
	GetUserPreferences(ctx context.Context, userID string) (*UserPreferences, error)
	UpdateUserPreferences(ctx context.Context, prefs *UserPreferences) error
	UpsertGuildMemberSnapshotsContext(ctx context.Context, guildID string, snapshots []Snapshot, updatedAt time.Time) error
	UpsertMemberJoinContext(ctx context.Context, guildID, userID string, joinedAt time.Time) error
	UpsertMemberPresenceContext(ctx context.Context, input PresenceInput) error
	MemberJoin(ctx context.Context, guildID, userID string) (time.Time, bool, error)
	GetAvatar(ctx context.Context, guildID, userID string) (hash string, updatedAt time.Time, ok bool, err error)
	GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[CurrentState, error]
	StreamAllGuildMemberRoles(ctx context.Context, guildID string) (iter.Seq2[string, []string], error)
	MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error
	UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error
}

```

// === FILE: pkg/members/sink.go ===
```go
package members

import (
	"context"
	"time"
)

// MemberSink is the abstraction for emitting pure member events.
type MemberSink interface {
	// OnMemberJoin is emitted when a member joins the guild.
	OnMemberJoin(ctx context.Context, intent MemberJoinIntent, accountAge time.Duration)

	// OnMemberLeave is emitted when a member leaves the guild.
	OnMemberLeave(ctx context.Context, intent MemberLeaveIntent, serverTime time.Duration, botTime time.Duration)

	// OnRoleUpdate is emitted when a member's roles change.
	OnRoleUpdate(ctx context.Context, intent RoleUpdateIntent)

	// OnAvatarUpdate is emitted when a user's avatar changes.
	OnAvatarUpdate(ctx context.Context, intent AvatarUpdateIntent)

	// OnModerationAction is emitted when a moderation action occurs.
	OnModerationAction(ctx context.Context, intent ModerationActionIntent)
}

// NopMemberSink is a no-operation implementation of MemberSink.
type NopMemberSink struct{}

func (NopMemberSink) OnMemberJoin(ctx context.Context, intent MemberJoinIntent, accountAge time.Duration) {
}
func (NopMemberSink) OnMemberLeave(ctx context.Context, intent MemberLeaveIntent, serverTime time.Duration, botTime time.Duration) {
}
func (NopMemberSink) OnRoleUpdate(ctx context.Context, intent RoleUpdateIntent)             {}
func (NopMemberSink) OnAvatarUpdate(ctx context.Context, intent AvatarUpdateIntent)         {}
func (NopMemberSink) OnModerationAction(ctx context.Context, intent ModerationActionIntent) {}

```

