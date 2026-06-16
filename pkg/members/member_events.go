package members

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// Hardcoded IDs for automatic role assignment
const unknownServerTimeSentinel time.Duration = -1

// MemberEventService manages member join/leave events
type MemberEventService struct {
	arikawaState   *state.State
	configManager  *files.ConfigManager
	botInstanceID  string
	sink           MemberSink
	activity       *monitoring.RuntimeActivity
	lifecycle      monitoring.ServiceLifecycle
	handlerCancels []func()
	logger         *slog.Logger

	// Cache for join times (member and bot)
	joinTimes map[string]time.Time // key: guildID:userID
	joinMu    sync.RWMutex

	// Complementary persistence (Postgres)
	store *storage.Store
}

// eventServiceDeps bundles the shared dependencies for the bot-scoped logging
// event services. BotInstanceID is normalized by the
// constructors via files.NormalizeBotInstanceID.
type eventServiceDeps struct {
	ArikawaState  *state.State
	ConfigManager *files.ConfigManager
	Sink          MemberSink
	Store         *storage.Store
	BotInstanceID string
	Logger        *slog.Logger
}

// NewMemberEventService creates a new instance of the member events service
func NewMemberEventService(arikawaState *state.State, configManager *files.ConfigManager, sink MemberSink, store *storage.Store, logger *slog.Logger) *MemberEventService {
	return NewMemberEventServiceForBot(eventServiceDeps{
		ArikawaState:  arikawaState,
		ConfigManager: configManager,
		Sink:          sink,
		Store:         store,
		Logger:        logger,
	})
}

// NewMemberEventServiceForBot creates a member event service scoped to one bot instance.
func NewMemberEventServiceForBot(deps eventServiceDeps) *MemberEventService {
	return &MemberEventService{
		arikawaState:  deps.ArikawaState,
		configManager: deps.ConfigManager,
		botInstanceID: files.NormalizeBotInstanceID(deps.BotInstanceID),
		sink:          deps.Sink,
		store:         deps.Store,
		logger:        deps.Logger,
		activity: monitoring.NewRuntimeActivity(deps.Store, monitoring.RuntimeActivityOptions{
			RunErr:        monitoring.RunErrWithTimeoutContext,
			EventTimeout:  monitoring.DependencyTimeout,
			BotInstanceID: files.NormalizeBotInstanceID(deps.BotInstanceID),
			Logger:        deps.Logger,
		}),
		lifecycle:      monitoring.NewServiceLifecycle("member event service"),
		handlerCancels: make([]func(), 0, 3),
	}
}

// Start registers member event handlers
func (mes *MemberEventService) Start(ctx context.Context) error {
	if mes.arikawaState == nil {
		return fmt.Errorf("member event service arikawa state is nil")
	}
	runCtx, err := mes.lifecycle.Start(ctx)
	if err != nil {
		return fmt.Errorf("MemberEventService.Start: %w", err)
	}

	// Ensure join map is initialized
	if mes.joinTimes == nil {
		mes.joinTimes = make(map[string]time.Time)
	}

	// Store should be injected and already initialized
	if mes.store != nil {
		if err := monitoring.RunErrWithTimeoutContext(runCtx, monitoring.DependencyTimeout, func(context.Context) error { return mes.store.Init() }); err != nil {
			mes.logger.Warn(fmt.Sprintf("Member event service: failed to initialize store (continuing): %v", err))
		}
	}

	mes.handlerCancels = mes.handlerCancels[:0]
	mes.handlerCancels = append(mes.handlerCancels,
		mes.arikawaState.AddHandler(func(e *gateway.GuildMemberAddEvent) {
			mes.handleGuildMemberAdd(context.Background(), e)
		}),
		mes.arikawaState.AddHandler(func(e *gateway.GuildMemberUpdateEvent) {
			mes.handleGuildMemberUpdate(context.Background(), e)
		}),
		mes.arikawaState.AddHandler(func(e *gateway.GuildMemberRemoveEvent) {
			mes.handleGuildMemberRemove(context.Background(), e)
		}),
	)

	cleanupCtx, done, ok := mes.lifecycle.Begin()
	if !ok {
		for _, cancel := range mes.handlerCancels {
			if cancel != nil {
				cancel()
			}
		}
		mes.handlerCancels = nil
		_ = mes.lifecycle.Cancel()
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

	for _, cancel := range mes.handlerCancels {
		if cancel != nil {
			cancel()
		}
	}
	mes.handlerCancels = nil

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

// handleGuildMemberAdd processes when a user joins the server
func (mes *MemberEventService) handleGuildMemberAdd(ctx context.Context, m *gateway.GuildMemberAddEvent) {
	if m == nil || m.User.Bot {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_add",
		slog.String("guildID", m.GuildID.String()),
		slog.String("userID", m.User.ID.String()),
	)
	defer done()

	mes.markEvent(ctx)
	if mes.configManager == nil {
		return
	}
	if !mes.handlesGuild(m.GuildID.String()) {
		return
	}
	cfg := mes.configManager.Config()
	if cfg == nil {
		return
	}
	guildConfig := mes.configManager.GuildConfig(m.GuildID.String())
	if guildConfig == nil {
		return
	}
	features := cfg.ResolveFeatures(m.GuildID.String())

	// Composite automatic role assignment (per-guild config).
	if mes.arikawaState != nil && features.AutoRoleAssign && guildConfig.Roles.AutoAssignment.Enabled {
		targetRoleID := guildConfig.Roles.AutoAssignment.TargetRoleID
		required := guildConfig.Roles.AutoAssignment.RequiredRoles
		if targetRoleID != "" && len(required) >= 2 {
			roles := make([]string, len(m.RoleIDs))
			for i, r := range m.RoleIDs {
				roles[i] = r.String()
			}
			if EvaluateAutoRoleDecision(roles, targetRoleID, required) == AutoRoleAddTarget {
				if err := mes.guildMemberRoleAdd(ctx, m.GuildID.String(), m.User.ID.String(), targetRoleID); err != nil {
					mes.logger.Error("Failed to grant target role on join", "guildID", m.GuildID, "userID", m.User.ID, "roleID", targetRoleID, "error", err)
				} else {
					mes.logger.Info("Granted target role on join", "guildID", m.GuildID, "userID", m.User.ID, "roleID", targetRoleID)
				}
			}
		}
	}

	// Logging is now delegated to Sink
	emit := logpolicy.ShouldEmitLogEvent(nil, mes.configManager, logpolicy.LogEventMemberJoin, m.GuildID.String())
	if !emit.Enabled {
		if emit.Reason == logpolicy.EmitReasonNoChannelConfigured {
			mes.logger.Info("User entry/leave channel not configured for guild, member join notification not sent", "guildID", m.GuildID, "userID", m.User.ID)
		} else {
			mes.logger.Debug("Member join notification suppressed by policy", "guildID", m.GuildID, "userID", m.User.ID, "reason", emit.Reason)
		}
		return
	}

	// Calculate how long the account has existed
	accountAge := mes.calculateAccountAge(m.User.ID.String())

	joinedAt := m.Joined.Time()

	// Persist absolute join time to Postgres store (best effort)
	if mes.store != nil && !joinedAt.IsZero() {
		if err := monitoring.RunErrWithTimeoutContext(ctx, monitoring.DependencyTimeout, func(runCtx context.Context) error {
			return mes.store.UpsertMemberJoinContext(runCtx, m.GuildID.String(), m.User.ID.String(), joinedAt)
		}); err != nil {
			mes.logger.Warn("Failed to persist member join timestamp", "guildID", m.GuildID, "userID", m.User.ID, "joinedAt", joinedAt, "error", err)
		}
		if err := monitoring.RunErrWithTimeoutContext(ctx, monitoring.DependencyTimeout, func(runCtx context.Context) error {
			return mes.store.IncrementDailyMemberJoinContext(runCtx, m.GuildID.String(), m.User.ID.String(), joinedAt)
		}); err != nil {
			mes.logger.Warn("Failed to increment daily member join metric", "guildID", m.GuildID, "userID", m.User.ID, "joinedAt", joinedAt, "error", err)
		}
	}

	// Register precise member join timestamp in memory
	if !joinedAt.IsZero() {
		mes.joinMu.Lock()
		if mes.joinTimes == nil {
			mes.joinTimes = make(map[string]time.Time)
		}
		mes.joinTimes[m.GuildID.String()+":"+m.User.ID.String()] = joinedAt
		mes.joinMu.Unlock()
	}

	mes.logger.Info("Member joined guild", "guildID", m.GuildID, "userID", m.User.ID, "username", m.User.Username, "accountAge", accountAge.String())

	if mes.sink != nil {
		mes.sink.OnMemberJoin(ctx, m, accountAge)
	}

}

// handleGuildMemberRemove processes when a user leaves the server
func (mes *MemberEventService) handleGuildMemberRemove(ctx context.Context, m *gateway.GuildMemberRemoveEvent) {
	if m == nil || m.User.Bot {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_remove",
		slog.String("guildID", m.GuildID.String()),
		slog.String("userID", m.User.ID.String()),
	)
	defer done()

	mes.markEvent(ctx)
	if mes.configManager == nil {
		return
	}
	if !mes.handlesGuild(m.GuildID.String()) {
		return
	}
	cfg := mes.configManager.Config()
	if cfg == nil {
		return
	}

	// Calculate how long they were in the server
	serverTime, hasServerTime, serverTimeErr := mes.calculateServerTime(ctx, m.GuildID.String(), m.User.ID.String())
	serverTimeForNotification := serverTime
	serverTimeForLog := "N/A"
	if serverTimeErr != nil {
		serverTimeForNotification = unknownServerTimeSentinel
	} else if hasServerTime {
		serverTimeForLog = serverTime.String()
	} else {
		serverTimeForLog = "unknown"
	}

	botTime := mes.getBotTimeOnServer(ctx, m.GuildID.String())

	// Increment daily member leave metric
	if mes.store != nil {
		if err := monitoring.RunErrWithTimeoutContext(ctx, monitoring.DependencyTimeout, func(runCtx context.Context) error {
			return mes.store.IncrementDailyMemberLeaveContext(runCtx, m.GuildID.String(), m.User.ID.String(), time.Now().UTC())
		}); err != nil {
			mes.logger.Warn("Failed to increment daily member leave metric", "guildID", m.GuildID, "userID", m.User.ID, "error", err)
		}
	}

	mes.logger.Info("Member left guild", "guildID", m.GuildID, "userID", m.User.ID, "username", m.User.Username, "serverTime", serverTimeForLog, "botTime", botTime.String())

	if mes.sink != nil {
		mes.sink.OnMemberLeave(ctx, m, serverTimeForNotification, botTime)
	}
}

// handleGuildMemberUpdate maintains the role relationship:
// - If the user loses role A, remove the target role.
// - If the user has both A and B, grant the target role (if not already present).
func (mes *MemberEventService) handleGuildMemberUpdate(ctx context.Context, m *gateway.GuildMemberUpdateEvent) {
	if m == nil || m.User.Bot {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_update",
		slog.String("guildID", m.GuildID.String()),
		slog.String("userID", m.User.ID.String()),
	)
	defer done()

	if mes.configManager == nil {
		return
	}
	if !mes.handlesGuild(m.GuildID.String()) {
		return
	}
	cfg := mes.configManager.Config()
	if cfg == nil {
		return
	}
	guildConfig := mes.configManager.GuildConfig(m.GuildID.String())
	if guildConfig == nil || !guildConfig.Roles.AutoAssignment.Enabled {
		return
	}
	if !cfg.ResolveFeatures(m.GuildID.String()).AutoRoleAssign {
		return
	}

	targetRoleID := guildConfig.Roles.AutoAssignment.TargetRoleID
	required := guildConfig.Roles.AutoAssignment.RequiredRoles
	if targetRoleID == "" || len(required) < 2 {
		return
	}

	roles := make([]string, len(m.RoleIDs))
	for i, r := range m.RoleIDs {
		roles[i] = r.String()
	}

	switch EvaluateAutoRoleDecision(roles, targetRoleID, required) {
	case AutoRoleRemoveTarget:
		if err := mes.guildMemberRoleRemove(ctx, m.GuildID.String(), m.User.ID.String(), targetRoleID); err != nil {
			mes.logger.Error("Failed to remove target role on update", "guildID", m.GuildID, "userID", m.User.ID, "roleID", targetRoleID, "error", err)
		} else {
			mes.logger.Info("Removed target role on update", "guildID", m.GuildID, "userID", m.User.ID, "roleID", targetRoleID)
		}
		return
	case AutoRoleAddTarget:
		if err := mes.guildMemberRoleAdd(ctx, m.GuildID.String(), m.User.ID.String(), targetRoleID); err != nil {
			mes.logger.Error("Failed to grant target role on update", "guildID", m.GuildID, "userID", m.User.ID, "roleID", targetRoleID, "error", err)
		} else {
			mes.logger.Info("Granted target role on update", "guildID", m.GuildID, "userID", m.User.ID, "roleID", targetRoleID)
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
	mes.joinMu.RLock()
	t, ok := mes.joinTimes[guildID+":"+userID]
	mes.joinMu.RUnlock()
	if ok && !t.IsZero() {
		return time.Since(t), true, nil
	}

	// 3) Persistent store (Postgres)
	if mes.store != nil {
		type joinLookup struct {
			at time.Time
			ok bool
		}
		res, err := monitoring.RunWithTimeoutContext(ctx, monitoring.DependencyTimeout, func(runCtx context.Context) (joinLookup, error) {
			at, ok, err := mes.store.MemberJoin(runCtx, guildID, userID)
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
	mes.joinMu.RLock()
	for key, joinTime := range mes.joinTimes {
		if now.Sub(joinTime) > threshold {
			toDelete = append(toDelete, key)
		}
	}
	mes.joinMu.RUnlock()

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
	if mes.arikawaState == nil {
		return 0
	}
	u, err := mes.arikawaState.Me()
	if err != nil || u == nil {
		return 0
	}
	botID := u.ID.String()
	member, err := mes.getGuildMember(ctx, guildID, botID)
	if err != nil || member == nil || member.Joined.Time().IsZero() {
		return 0
	}
	return time.Since(member.Joined.Time())
}

func (mes *MemberEventService) getGuildMember(ctx context.Context, guildID, userID string) (*discord.Member, error) {
	if mes.arikawaState == nil {
		return nil, fmt.Errorf("arikawa state is nil")
	}
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	uID, err := discord.ParseSnowflake(userID)
	if err != nil {
		return nil, err
	}
	return monitoring.RunWithTimeout(ctx, monitoring.DependencyTimeout, func() (*discord.Member, error) {
		return mes.arikawaState.Member(discord.GuildID(gID), discord.UserID(uID))
	})
}

func (mes *MemberEventService) guildMemberRoleAdd(ctx context.Context, guildID, userID, roleID string) error {
	if mes.arikawaState == nil {
		return fmt.Errorf("arikawa state is nil")
	}
	gID, _ := discord.ParseSnowflake(guildID)
	uID, _ := discord.ParseSnowflake(userID)
	rID, _ := discord.ParseSnowflake(roleID)
	return monitoring.RunErrWithTimeout(ctx, monitoring.DependencyTimeout, func() error {
		return mes.arikawaState.Client.AddRole(discord.GuildID(gID), discord.UserID(uID), discord.RoleID(rID), api.AddRoleData{})
	})
}

func (mes *MemberEventService) guildMemberRoleRemove(ctx context.Context, guildID, userID, roleID string) error {
	if mes.arikawaState == nil {
		return fmt.Errorf("arikawa state is nil")
	}
	gID, _ := discord.ParseSnowflake(guildID)
	uID, _ := discord.ParseSnowflake(userID)
	rID, _ := discord.ParseSnowflake(roleID)
	return monitoring.RunErrWithTimeout(ctx, monitoring.DependencyTimeout, func() error {
		return mes.arikawaState.Client.RemoveRole(discord.GuildID(gID), discord.UserID(uID), discord.RoleID(rID), "")
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
	if !guild.BelongsToBotInstance(mes.botInstanceID) {
		return false
	}
	resolvedID, _ := guild.ResolveFeatureBotInstanceID("roles")
	return resolvedID == mes.botInstanceID
}
