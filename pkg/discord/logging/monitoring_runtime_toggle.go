package logging

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

type monitoringWorkloadState struct {
	memberEventService    bool
	messageEventService   bool
	reactionEventService  bool
	presenceHandler       bool
	memberUpdateHandler   bool
	statsMemberHandlers   bool
	userUpdateHandler     bool
	botPermMirrorHandlers bool
	avatarScan            bool
	statsUpdates          bool
	rolesRefresh          bool
	backfill              bool
}

func resolveMonitoringWorkloadState(cfg *files.BotConfig) monitoringWorkloadState {
	state := monitoringWorkloadState{}
	if cfg == nil {
		return state
	}

	state.memberEventService = shouldRunMemberEventService(cfg, cfg.RuntimeConfig)
	for _, guildCfg := range cfg.Guilds {
		features := cfg.ResolveFeatures(guildCfg.GuildID)
		if !features.Services.Monitoring {
			continue
		}
		rc := cfg.ResolveRuntimeConfig(guildCfg.GuildID)
		statsEnabledForGuild := features.StatsChannels && statsEnabled(guildCfg.Stats)

		avatarEnabled := !rc.DisableUserLogs && features.Logging.AvatarLogging
		roleEnabled := !rc.DisableUserLogs && features.Logging.RoleUpdate
		presenceWatchEnabled := (features.PresenceWatch.User && strings.TrimSpace(rc.PresenceWatchUserID) != "") ||
			(features.PresenceWatch.Bot && rc.PresenceWatchBot)

		if avatarEnabled || presenceWatchEnabled {
			state.presenceHandler = true
		}
		if avatarEnabled || roleEnabled || statsEnabledForGuild {
			state.memberUpdateHandler = true
		}
		if avatarEnabled {
			state.userUpdateHandler = true
			state.avatarScan = true
		}
		if !rc.DisableMessageLogs && (features.Logging.MessageProcess || features.Logging.MessageEdit || features.Logging.MessageDelete) {
			state.messageEventService = true
		}
		if (!rc.DisableReactionLogs && features.Logging.ReactionMetric) || !guildCfg.ReactionBlocks.IsZero() {
			state.reactionEventService = true
		}
		if statsEnabledForGuild {
			state.statsUpdates = true
			state.statsMemberHandlers = true
		}
		if features.Backfill.Enabled && strings.TrimSpace(rc.BackfillChannelID) != "" {
			state.backfill = true
		}
		if features.Safety.BotRolePermMirror && !rc.DisableBotRolePermMirror {
			state.botPermMirrorHandlers = true
		}
		if roleEnabled || (features.AutoRoleAssign && guildCfg.Roles.AutoAssignment.Enabled) {
			state.rolesRefresh = true
		}
	}
	if state.memberEventService {
		state.rolesRefresh = true
	}
	return state
}

func (ms *MonitoringService) workloadState(globalRC files.RuntimeConfig) monitoringWorkloadState {
	cfg := ms.scopedConfig()
	if cfg == nil {
		return monitoringWorkloadState{}
	}
	scoped := *cfg
	scoped.RuntimeConfig = globalRC
	return resolveMonitoringWorkloadState(&scoped)
}

func (ms *MonitoringService) syncSchedulesLocked(runCtx context.Context, state monitoringWorkloadState) {
	if !state.avatarScan && ms.run.cronCancel != nil {
		ms.run.cronCancel()
		ms.run.cronCancel = nil
	}
	if !state.statsUpdates && ms.run.statsCronCancel != nil {
		ms.run.statsCronCancel()
		ms.run.statsCronCancel = nil
	}
	if !state.rolesRefresh && ms.run.rolesRefreshCronCancel != nil {
		ms.run.rolesRefreshCronCancel()
		ms.run.rolesRefreshCronCancel = nil
	}

	if ms.router == nil || runCtx == nil {
		return
	}

	if state.avatarScan {
		ms.router.RegisterHandler("monitor.scan_avatars", func(ctx context.Context, payload any) error {
			if _, ok := payload.(task.EmptyPayload); !ok {
				return fmt.Errorf("invalid payload type for monitor.scan_avatars")
			}
			return ms.runAvatarScanTask(runCtx)
		})
		if ms.run.cronCancel == nil {
			ms.run.cronCancel = ms.router.ScheduleEvery(2*time.Hour, task.Task{Type: "monitor.scan_avatars", Payload: task.EmptyPayload{}})
		}
	}

	if state.statsUpdates {
		ms.router.RegisterHandler("monitor.update_stats_channels", func(ctx context.Context, payload any) error {
			if _, ok := payload.(task.EmptyPayload); !ok {
				return fmt.Errorf("invalid payload type for monitor.update_stats_channels")
			}
			return ms.runStatsUpdateTask(runCtx)
		})
		if ms.run.statsCronCancel == nil {
			ms.run.statsCronCancel = ms.router.ScheduleEvery(5*time.Minute, task.Task{Type: "monitor.update_stats_channels", Payload: task.EmptyPayload{}})
			ms.dispatchMonitorTaskLocked(runCtx, "monitor.update_stats_channels")
		}
	}

	if state.rolesRefresh {
		ms.router.RegisterHandler("monitor.refresh_roles", func(ctx context.Context, payload any) error {
			if _, ok := payload.(task.EmptyPayload); !ok {
				return fmt.Errorf("invalid payload type for monitor.refresh_roles")
			}
			return ms.runRolesRefreshTask(runCtx)
		})
		if ms.run.rolesRefreshCronCancel == nil {
			ms.run.rolesRefreshCronCancel = ms.router.ScheduleDailyAtUTC(3, 0, task.Task{Type: "monitor.refresh_roles", Payload: task.EmptyPayload{}})
			ms.dispatchMonitorTaskLocked(runCtx, "monitor.refresh_roles")
		}
	}
}
