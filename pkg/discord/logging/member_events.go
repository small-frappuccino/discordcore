package logging

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// Hardcoded IDs for automatic role assignment
const unknownServerTimeSentinel time.Duration = -1

// MemberEventService manages member join/leave events
type MemberEventService struct {
	session        *discordgo.Session
	configManager  *files.ConfigManager
	botInstanceID  string
	defaultBotID   string
	notifier       *NotificationSender
	adapters       *task.NotificationAdapters
	activity       *runtimeActivity
	lifecycle      serviceLifecycle
	handlerCancels []func()

	// Cache for join times (member and bot)

	joinTimes map[string]time.Time // key: guildID:userID
	joinMu    sync.RWMutex

	// Complementary persistence (Postgres)
	store *storage.Store
}

// NewMemberEventService creates a new instance of the member events service
func NewMemberEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender, store *storage.Store) *MemberEventService {
	return NewMemberEventServiceForBot(session, configManager, notifier, store, "", "")
}

// NewMemberEventServiceForBot creates a member event service scoped to one bot instance.
func NewMemberEventServiceForBot(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	notifier *NotificationSender,
	store *storage.Store,
	botInstanceID string,
	defaultBotInstanceID string,
) *MemberEventService {
	return &MemberEventService{
		session:       session,
		configManager: configManager,
		botInstanceID: files.NormalizeBotInstanceID(botInstanceID),
		defaultBotID:  files.NormalizeBotInstanceID(defaultBotInstanceID),
		notifier:      notifier,
		store:         store,
		activity: newRuntimeActivity(store, runtimeActivityOptions{
			RunErr:        runErrWithTimeoutContext,
			EventTimeout:  loggingDependencyTimeout,
			BotInstanceID: files.NormalizeBotInstanceID(botInstanceID),
			Warn:          slog.Warn,
		}),
		lifecycle:      newServiceLifecycle("member event service"),
		handlerCancels: make([]func(), 0, 3),
	}
}

func (mes *MemberEventService) SetAdapters(adapters *task.NotificationAdapters) {
	mes.adapters = adapters
}

// Start registers member event handlers
func (mes *MemberEventService) Start(ctx context.Context) error {
	if mes.session == nil {
		return fmt.Errorf("member event service discord session is nil")
	}
	runCtx, err := mes.lifecycle.Start(ctx)
	if err != nil {
		return err
	}

	// Ensure join map is initialized
	if mes.joinTimes == nil {
		mes.joinTimes = make(map[string]time.Time)
	}

	// Store should be injected and already initialized
	if mes.store != nil {
		if err := runErrWithTimeoutContext(runCtx, loggingDependencyTimeout, func(context.Context) error { return mes.store.Init() }); err != nil {
			slog.Warn(fmt.Sprintf("Member event service: failed to initialize store (continuing): %v", err))
		}
	}

	mes.handlerCancels = mes.handlerCancels[:0]
	mes.handlerCancels = append(mes.handlerCancels,
		mes.session.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
			runCtx, done, ok := mes.lifecycle.Begin()
			if !ok {
				return
			}
			defer done()

			mes.handleGuildMemberAdd(runCtx, s, m)
		}),
		mes.session.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
			runCtx, done, ok := mes.lifecycle.Begin()
			if !ok {
				return
			}
			defer done()

			mes.handleGuildMemberUpdate(runCtx, s, m)
		}),
		mes.session.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
			runCtx, done, ok := mes.lifecycle.Begin()
			if !ok {
				return
			}
			defer done()

			mes.handleGuildMemberRemove(runCtx, s, m)
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

	slog.Info("Member event service started")
	return nil
}

// Stop the service
func (mes *MemberEventService) Stop(ctx context.Context) error {
	if err := mes.lifecycle.Cancel(); err != nil {
		return err
	}

	for _, cancel := range mes.handlerCancels {
		if cancel != nil {
			cancel()
		}
	}
	mes.handlerCancels = nil

	if err := mes.lifecycle.Wait(ctx); err != nil {
		return err
	}

	slog.Info("Member event service stopped")
	return nil
}

// IsRunning returns whether the service is running
func (mes *MemberEventService) IsRunning() bool {
	return mes.lifecycle.IsRunning()
}

// handleGuildMemberAdd processes when a user joins the server
func (mes *MemberEventService) handleGuildMemberAdd(ctx context.Context, s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m == nil || m.User == nil || m.User.Bot {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_add",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.User.ID),
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
	features := cfg.ResolveFeatures(m.GuildID)

	// Keep member from the gateway payload when available.
	var member *discordgo.Member
	if m.Member != nil {
		member = m.Member
	}

	// Composite automatic role assignment (per-guild config).
	// This must not depend on entry/exit logging toggles.
	if mes.session != nil && features.AutoRoleAssign && guildConfig.Roles.AutoAssignment.Enabled {
		targetRoleID := guildConfig.Roles.AutoAssignment.TargetRoleID
		required := guildConfig.Roles.AutoAssignment.RequiredRoles
		if targetRoleID != "" && len(required) >= 2 {
			if member != nil {
				roles := member.Roles
				if evaluateAutoRoleDecision(roles, targetRoleID, required) == autoRoleAddTarget {
					if err := mes.guildMemberRoleAdd(ctx, m.GuildID, m.User.ID, targetRoleID); err != nil {
						slog.Error(fmt.Sprintf("Failed to grant target role on join: guildID=%s, userID=%s, roleID=%s, error=%v", m.GuildID, m.User.ID, targetRoleID, err))
					} else {
						slog.Info(fmt.Sprintf("Granted target role on join: guildID=%s, userID=%s, roleID=%s", m.GuildID, m.User.ID, targetRoleID))
					}
				}
			} else {
				// As a last resort (only when role assignment is enabled), fetch member once.
				if mm, err := mes.getGuildMember(ctx, m.GuildID, m.User.ID); err == nil && mm != nil {
					member = mm
					roles := mm.Roles
					if evaluateAutoRoleDecision(roles, targetRoleID, required) == autoRoleAddTarget {
						if err := mes.guildMemberRoleAdd(ctx, m.GuildID, m.User.ID, targetRoleID); err != nil {
							slog.Error(fmt.Sprintf("Failed to grant target role on join: guildID=%s, userID=%s, roleID=%s, error=%v", m.GuildID, m.User.ID, targetRoleID, err))
						} else {
							slog.Info(fmt.Sprintf("Granted target role on join: guildID=%s, userID=%s, roleID=%s", m.GuildID, m.User.ID, targetRoleID))
						}
					}
				}
			}
		}
	}

	emit := ShouldEmitLogEvent(mes.session, mes.configManager, LogEventMemberJoin, m.GuildID)
	if !emit.Enabled {
		if emit.Reason == EmitReasonNoChannelConfigured {
			slog.Info(fmt.Sprintf("User entry/leave channel not configured for guild, member join notification not sent: guildID=%s, userID=%s", m.GuildID, m.User.ID))
		} else {
			slog.Debug(fmt.Sprintf("Member join notification suppressed by policy: guildID=%s, userID=%s, reason=%s", m.GuildID, m.User.ID, emit.Reason))
		}
		return
	}
	logChannelID := emit.ChannelID

	// Calculate how long the account has existed
	accountAge := mes.calculateAccountAge(m.User.ID)

	// Resolve JoinedAt with minimal API usage:
	// - Prefer the timestamp already present in the event payload.
	// - Fallback to a single REST query only when missing.
	joinedAt := time.Time{}
	if member != nil {
		joinedAt = member.JoinedAt
	}
	if joinedAt.IsZero() && mes.session != nil {
		if mm, err := mes.getGuildMember(ctx, m.GuildID, m.User.ID); err == nil && mm != nil {
			member = mm
			joinedAt = mm.JoinedAt
		}
	}

	// Persist absolute join time to Postgres store (best effort)
	if mes.store != nil && !joinedAt.IsZero() {
		if err := runErrWithTimeoutContext(ctx, loggingDependencyTimeout, func(runCtx context.Context) error {
			return mes.store.UpsertMemberJoinContext(runCtx, m.GuildID, m.User.ID, joinedAt)
		}); err != nil {
			slog.Warn("Failed to persist member join timestamp", "guildID", m.GuildID, "userID", m.User.ID, "joinedAt", joinedAt, "error", err)
		}
		if err := runErrWithTimeoutContext(ctx, loggingDependencyTimeout, func(runCtx context.Context) error {
			return mes.store.IncrementDailyMemberJoinContext(runCtx, m.GuildID, m.User.ID, joinedAt)
		}); err != nil {
			slog.Warn("Failed to increment daily member join metric", "guildID", m.GuildID, "userID", m.User.ID, "joinedAt", joinedAt, "error", err)
		}
	}

	// Register precise member join timestamp in memory
	if !joinedAt.IsZero() {
		mes.joinMu.Lock()
		if mes.joinTimes == nil {
			mes.joinTimes = make(map[string]time.Time)
		}
		mes.joinTimes[m.GuildID+":"+m.User.ID] = joinedAt
		mes.joinMu.Unlock()
	}

	slog.Info(fmt.Sprintf("Member joined guild: guildID=%s, userID=%s, username=%s, accountAge=%s", m.GuildID, m.User.ID, m.User.Username, accountAge.String()))

	if mes.adapters != nil {
		if err := mes.adapters.EnqueueMemberJoin(logChannelID, m, accountAge); err != nil {
			slog.Error(fmt.Sprintf("Failed to send member join notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err))
		} else {
			slog.Info(fmt.Sprintf("Member join notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID))
		}
	} else if mes.notifier != nil {
		if err := runErrWithTimeout(ctx, loggingDependencyTimeout, func() error {
			return mes.notifier.SendMemberJoinNotification(logChannelID, m, accountAge)
		}); err != nil {
			slog.Error(fmt.Sprintf("Failed to send member join notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err))
		} else {
			slog.Info(fmt.Sprintf("Member join notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID))
		}
	}

}

// handleGuildMemberRemove processes when a user leaves the server
func (mes *MemberEventService) handleGuildMemberRemove(ctx context.Context, s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m == nil || m.User == nil || m.User.Bot {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_remove",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.User.ID),
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
	emit := ShouldEmitLogEvent(mes.session, mes.configManager, LogEventMemberLeave, m.GuildID)
	if !emit.Enabled {
		if emit.Reason == EmitReasonNoChannelConfigured {
			slog.Info(fmt.Sprintf("User entry/leave channel not configured for guild, member leave notification not sent: guildID=%s, userID=%s", m.GuildID, m.User.ID))
		} else {
			slog.Debug(fmt.Sprintf("Member leave notification suppressed by policy: guildID=%s, userID=%s, reason=%s", m.GuildID, m.User.ID, emit.Reason))
		}
		return
	}
	logChannelID := emit.ChannelID

	// Calculate how long they were in the server
	serverTime, hasServerTime, serverTimeErr := mes.calculateServerTime(ctx, m.GuildID, m.User.ID)
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
	if mes.store != nil {
		if err := runErrWithTimeoutContext(ctx, loggingDependencyTimeout, func(runCtx context.Context) error {
			return mes.store.IncrementDailyMemberLeaveContext(runCtx, m.GuildID, m.User.ID, time.Now().UTC())
		}); err != nil {
			slog.Warn("Failed to increment daily member leave metric", "guildID", m.GuildID, "userID", m.User.ID, "error", err)
		}
	}

	slog.Info(fmt.Sprintf("Member left guild: guildID=%s, userID=%s, username=%s, serverTime=%s, botTime=%s", m.GuildID, m.User.ID, m.User.Username, serverTimeForLog, botTime.String()))

	if mes.adapters != nil {
		if err := mes.adapters.EnqueueMemberLeave(logChannelID, m, serverTimeForNotification, botTime); err != nil {
			slog.Error(fmt.Sprintf("Failed to send member leave notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err))
		} else {
			slog.Info(fmt.Sprintf("Member leave notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID))
		}
	} else if mes.notifier != nil {
		if err := runErrWithTimeout(ctx, loggingDependencyTimeout, func() error {
			return mes.notifier.SendMemberLeaveNotification(logChannelID, m, serverTimeForNotification, botTime)
		}); err != nil {
			slog.Error(fmt.Sprintf("Failed to send member leave notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err))
		} else {
			slog.Info(fmt.Sprintf("Member leave notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID))
		}
	}
}

// handleGuildMemberUpdate maintains the role relationship:
// - If the user loses role A, remove the target role.
// - If the user has both A and B, grant the target role (if not already present).
func (mes *MemberEventService) handleGuildMemberUpdate(ctx context.Context, s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m == nil || m.User == nil || m.User.Bot {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_update",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.User.ID),
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
	if !cfg.ResolveFeatures(m.GuildID).AutoRoleAssign {
		return
	}

	targetRoleID := guildConfig.Roles.AutoAssignment.TargetRoleID
	required := guildConfig.Roles.AutoAssignment.RequiredRoles
	if targetRoleID == "" || len(required) < 2 {
		return
	}
	switch evaluateAutoRoleDecision(m.Roles, targetRoleID, required) {
	case autoRoleRemoveTarget:
		if err := mes.guildMemberRoleRemove(ctx, m.GuildID, m.User.ID, targetRoleID); err != nil {
			slog.Error(fmt.Sprintf("Failed to remove target role on update: guildID=%s, userID=%s, roleID=%s, error=%v", m.GuildID, m.User.ID, targetRoleID, err))
		} else {
			slog.Info(fmt.Sprintf("Removed target role on update: guildID=%s, userID=%s, roleID=%s", m.GuildID, m.User.ID, targetRoleID))
		}
		return
	case autoRoleAddTarget:
		if err := mes.guildMemberRoleAdd(ctx, m.GuildID, m.User.ID, targetRoleID); err != nil {
			slog.Error(fmt.Sprintf("Failed to grant target role on update: guildID=%s, userID=%s, roleID=%s, error=%v", m.GuildID, m.User.ID, targetRoleID, err))
		} else {
			slog.Info(fmt.Sprintf("Granted target role on update: guildID=%s, userID=%s, roleID=%s", m.GuildID, m.User.ID, targetRoleID))
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
		slog.Warn(fmt.Sprintf("Failed to parse user ID for account age calculation: userID=%s, error=%v", userID, err))
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
		res, err := runWithTimeoutContext(ctx, loggingDependencyTimeout, func(runCtx context.Context) (joinLookup, error) {
			at, ok, err := mes.store.GetMemberJoinContext(runCtx, guildID, userID)
			return joinLookup{at: at, ok: ok}, err
		})
		if err != nil {
			slog.Warn("Failed to read member join timestamp from store; time on server unavailable", "guildID", guildID, "userID", userID, "error", err)
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
		log.ApplicationLogger().Info("Cleaned up old join time entries from memory", "count", len(toDelete))
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
	if mes.session == nil || mes.session.State == nil || mes.session.State.User == nil {
		return 0
	}
	botID := mes.session.State.User.ID
	member, err := mes.getGuildMember(ctx, guildID, botID)
	if err != nil || member == nil || member.JoinedAt.IsZero() {
		return 0
	}
	return time.Since(member.JoinedAt)
}

func (mes *MemberEventService) getGuildMember(ctx context.Context, guildID, userID string) (*discordgo.Member, error) {
	if mes.session == nil {
		return nil, fmt.Errorf("discord session is nil")
	}
	return runWithTimeout(ctx, loggingDependencyTimeout, func() (*discordgo.Member, error) {
		return mes.session.GuildMember(guildID, userID)
	})
}

func (mes *MemberEventService) guildMemberRoleAdd(ctx context.Context, guildID, userID, roleID string) error {
	if mes.session == nil {
		return fmt.Errorf("discord session is nil")
	}
	return runErrWithTimeout(ctx, loggingDependencyTimeout, func() error {
		return mes.session.GuildMemberRoleAdd(guildID, userID, roleID)
	})
}

func (mes *MemberEventService) guildMemberRoleRemove(ctx context.Context, guildID, userID, roleID string) error {
	if mes.session == nil {
		return fmt.Errorf("discord session is nil")
	}
	return runErrWithTimeout(ctx, loggingDependencyTimeout, func() error {
		return mes.session.GuildMemberRoleRemove(guildID, userID, roleID)
	})
}

func (mes *MemberEventService) handlesGuild(guildID string) bool {
	if mes == nil || mes.configManager == nil {
		return false
	}
	if files.NormalizeBotInstanceID(mes.botInstanceID) == "" && files.NormalizeBotInstanceID(mes.defaultBotID) == "" {
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
	return guild.EffectiveBotInstanceID(mes.defaultBotID) == files.NormalizeBotInstanceID(mes.botInstanceID)
}
