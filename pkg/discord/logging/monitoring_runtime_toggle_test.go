package logging

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func newMonitoringTestConfigManager(t *testing.T) *files.ConfigManager {
	t.Helper()

	mgr := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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

	ms := &MonitoringService{
		session:              session,
		configManager:        cfgMgr,
		memberEventService:   NewMemberEventService(session, cfgMgr, nil, nil),
		messageEventService:  NewMessageEventService(session, cfgMgr, nil, nil),
		reactionEventService: NewReactionEventService(session, cfgMgr, nil),
		isRunning:            true,
		eventHandlers:        make([]interface{}, 0),
	}

	if err := ms.memberEventService.Start(); err != nil {
		t.Fatalf("start member service: %v", err)
	}
	if err := ms.messageEventService.Start(); err != nil {
		t.Fatalf("start message service: %v", err)
	}
	if err := ms.reactionEventService.Start(); err != nil {
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
	if got := len(ms.eventHandlers); got != 7 {
		t.Fatalf("expected 7 handlers after enable toggles, got %d", got)
	}

	ms.removeEventHandlers()
	if ms.memberEventService.IsRunning() {
		_ = ms.memberEventService.Stop()
	}
	if ms.messageEventService.IsRunning() {
		_ = ms.messageEventService.Stop()
	}
	if ms.reactionEventService.IsRunning() {
		_ = ms.reactionEventService.Stop()
	}
}
