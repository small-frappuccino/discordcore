package logging

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

func newMonitoringTestConfigManager(t *testing.T) *files.ConfigManager {
	t.Helper()

	mgr := files.NewMemoryConfigManager()
	if err := mgr.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	return mgr
}

func TestMonitoringService_SetupAndRemoveEventHandlersFromRuntimeConfig(t *testing.T) {
	session := newLoggingLifecycleSession(t)
	cfgMgr := newMonitoringTestConfigManager(t)

	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		eventHandlers: make([]interface{}, 0),
	}

	ms.setupEventHandlersFromRuntimeConfig(files.RuntimeConfig{DisableUserLogs: true})
	if got := len(ms.eventHandlers); got != 4 {
		t.Fatalf("expected 4 handlers when user logs are disabled, got %d", got)
	}

	ms.removeEventHandlers()
	if got := len(ms.eventHandlers); got != 0 {
		t.Fatalf("expected 0 handlers after remove, got %d", got)
	}

	ms.setupEventHandlersFromRuntimeConfig(files.RuntimeConfig{})
	if got := len(ms.eventHandlers); got != 7 {
		t.Fatalf("expected 7 handlers when user logs are enabled, got %d", got)
	}

	ms.removeEventHandlers()
}

func TestMonitoringService_ApplyRuntimeTogglesStartsAndStopsServices(t *testing.T) {
	session := newLoggingLifecycleSession(t)
	cfgMgr := newMonitoringTestConfigManager(t)
	router := task.NewRouter(task.Defaults())
	t.Cleanup(router.Close)
	runCtx, cancelRun := context.WithCancel(context.Background())
	t.Cleanup(cancelRun)

	ms := &MonitoringService{
		session:              session,
		configManager:        cfgMgr,
		memberEventService:   NewMemberEventService(session, cfgMgr, nil, nil),
		messageEventService:  NewMessageEventService(session, cfgMgr, nil, nil),
		reactionEventService: NewReactionEventService(session, cfgMgr, nil),
		router:               router,
		isRunning:            true,
		runCtx:               runCtx,
		eventHandlers:        make([]interface{}, 0),
	}

	if err := ms.memberEventService.Start(context.Background()); err != nil {
		t.Fatalf("start member service: %v", err)
	}
	if err := ms.messageEventService.Start(context.Background()); err != nil {
		t.Fatalf("start message service: %v", err)
	}
	if err := ms.reactionEventService.Start(context.Background()); err != nil {
		t.Fatalf("start reaction service: %v", err)
	}

	disabledRC := files.RuntimeConfig{
		DisableEntryExitLogs: true,
		DisableMessageLogs:   true,
		DisableReactionLogs:  true,
		DisableUserLogs:      true,
	}
	if err := ms.ApplyRuntimeToggles(context.Background(), disabledRC); err != nil {
		t.Fatalf("apply disabled runtime toggles: %v", err)
	}

	if ms.memberEventService.IsRunning() {
		t.Fatalf("member service should be stopped after disable toggles")
	}
	if ms.messageEventService.IsRunning() {
		t.Fatalf("message service should be stopped after disable toggles")
	}
	if ms.reactionEventService.IsRunning() {
		t.Fatalf("reaction service should be stopped after disable toggles")
	}
	if ms.cronCancel != nil {
		t.Fatalf("avatar scan schedule should be stopped after disable toggles")
	}
	if ms.rolesRefreshCronCancel != nil {
		t.Fatalf("roles refresh schedule should be stopped after disable toggles")
	}
	if got := len(ms.eventHandlers); got != 4 {
		t.Fatalf("expected 4 handlers after disable toggles, got %d", got)
	}

	if err := ms.ApplyRuntimeToggles(context.Background(), files.RuntimeConfig{}); err != nil {
		t.Fatalf("apply enabled runtime toggles: %v", err)
	}

	if !ms.memberEventService.IsRunning() {
		t.Fatalf("member service should be running after enable toggles")
	}
	if !ms.messageEventService.IsRunning() {
		t.Fatalf("message service should be running after enable toggles")
	}
	if !ms.reactionEventService.IsRunning() {
		t.Fatalf("reaction service should be running after enable toggles")
	}
	if ms.cronCancel == nil {
		t.Fatalf("avatar scan schedule should be active after enable toggles")
	}
	if ms.rolesRefreshCronCancel == nil {
		t.Fatalf("roles refresh schedule should be active after enable toggles")
	}
	if ms.statsCronCancel != nil {
		t.Fatalf("stats schedule should remain inactive without stats config")
	}
	if got := len(ms.eventHandlers); got != 7 {
		t.Fatalf("expected 7 handlers after enable toggles, got %d", got)
	}

	ms.removeEventHandlers()
	if ms.memberEventService.IsRunning() {
		_ = ms.memberEventService.Stop(context.Background())
	}
	if ms.messageEventService.IsRunning() {
		_ = ms.messageEventService.Stop(context.Background())
	}
	if ms.reactionEventService.IsRunning() {
		_ = ms.reactionEventService.Stop(context.Background())
	}
}

func TestMonitoringService_SyncSchedulesLockedReactivatesSchedules(t *testing.T) {
	session := newLoggingLifecycleSession(t)
	cfgMgr := newMonitoringTestConfigManager(t)
	router := task.NewRouter(task.Defaults())
	t.Cleanup(router.Close)
	runCtx, cancelRun := context.WithCancel(context.Background())
	t.Cleanup(cancelRun)

	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		router:        router,
		runCtx:        runCtx,
		statsLastRun:  make(map[string]time.Time),
	}

	state := monitoringWorkloadState{
		avatarScan:   true,
		statsUpdates: true,
		rolesRefresh: true,
	}

	ms.syncSchedulesLocked(runCtx, state)
	if ms.cronCancel == nil {
		t.Fatalf("avatar scan schedule should be created")
	}
	if ms.statsCronCancel == nil {
		t.Fatalf("stats schedule should be created")
	}
	if ms.rolesRefreshCronCancel == nil {
		t.Fatalf("roles refresh schedule should be created")
	}
	if got := router.Stats().RegisteredTypes; got != 3 {
		t.Fatalf("expected 3 registered task handlers, got %d", got)
	}

	ms.syncSchedulesLocked(runCtx, monitoringWorkloadState{})
	if ms.cronCancel != nil {
		t.Fatalf("avatar scan schedule should be removed")
	}
	if ms.statsCronCancel != nil {
		t.Fatalf("stats schedule should be removed")
	}
	if ms.rolesRefreshCronCancel != nil {
		t.Fatalf("roles refresh schedule should be removed")
	}

	ms.syncSchedulesLocked(runCtx, state)
	if ms.cronCancel == nil {
		t.Fatalf("avatar scan schedule should be recreated")
	}
	if ms.statsCronCancel == nil {
		t.Fatalf("stats schedule should be recreated")
	}
	if ms.rolesRefreshCronCancel == nil {
		t.Fatalf("roles refresh schedule should be recreated")
	}

	ms.syncSchedulesLocked(runCtx, monitoringWorkloadState{})
}

func TestMonitoringService_SetupEventHandlersKeepsPresenceWatchWhenUserLogsDisabled(t *testing.T) {
	session := newLoggingLifecycleSession(t)
	cfgMgr := newMonitoringTestConfigManager(t)
	enabled := true
	disabled := false
	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Features.Logging.AvatarLogging = &disabled
		cfg.Features.Logging.RoleUpdate = &disabled
		cfg.Features.PresenceWatch.Bot = &enabled
		cfg.Features.Safety.BotRolePermMirror = &disabled
		return nil
	}); err != nil {
		t.Fatalf("update config: %v", err)
	}

	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		eventHandlers: make([]interface{}, 0),
	}

	ms.setupEventHandlersFromRuntimeConfig(files.RuntimeConfig{
		DisableUserLogs:  true,
		PresenceWatchBot: true,
	})
	if got := len(ms.eventHandlers); got != 3 {
		t.Fatalf("expected 3 handlers with presence watch only, got %d", got)
	}
}
