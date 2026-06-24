package runtimeapply

import (
	"context"
	"errors"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

type MockMonitoringHotApplier struct {
	Called bool
	Config files.RuntimeConfig
	Err    error
}

func (m *MockMonitoringHotApplier) ApplyRuntimeToggles(ctx context.Context, rc files.RuntimeConfig) error {
	m.Called = true
	m.Config = rc
	return m.Err
}

func TestManager(t *testing.T) {
	t.Parallel()
	// Register a dummy theme for testing SetCurrent
	testTheme := &theme.Theme{Name: "testtheme"}
	_ = theme.Register(testTheme)

	sm := service.NewServiceManager(nil)
	mockMonitoring := &MockMonitoringHotApplier{}

	// Test New
	mgr := New(sm, mockMonitoring)
	if len(mgr.serviceManagers) != 1 {
		t.Errorf("expected 1 service manager, got %d", len(mgr.serviceManagers))
	}
	if len(mgr.monitoringHotApply) != 1 {
		t.Errorf("expected 1 monitoring hot applier, got %d", len(mgr.monitoringHotApply))
	}

	// Test AddRuntime ignoring nil values
	mgr.AddRuntime(nil, nil)
	if len(mgr.serviceManagers) != 1 {
		t.Errorf("expected no service manager added when nil")
	}

	// Test SetInitial
	initialConfig := files.RuntimeConfig{
		BotTheme:             "default",
		DisableEntryExitLogs: false,
	}
	mgr.SetInitial(initialConfig)

	// Test Apply with no changes
	err := mgr.Apply(context.Background(), initialConfig)
	if err != nil {
		t.Fatalf("expected nil error on unchanged config apply, got: %v", err)
	}
	if mockMonitoring.Called {
		t.Errorf("expected monitoring hot applier not to be called when config is unchanged")
	}

	// Test Apply with theme change
	nextConfigTheme := files.RuntimeConfig{
		BotTheme:             "testtheme",
		DisableEntryExitLogs: false,
	}
	err = mgr.Apply(context.Background(), nextConfigTheme)
	if err != nil {
		t.Fatalf("expected nil error on valid theme change apply, got: %v", err)
	}

	// Test Apply with invalid theme change
	invalidConfigTheme := files.RuntimeConfig{
		BotTheme: "invalidtheme",
	}
	err = mgr.Apply(context.Background(), invalidConfigTheme)
	if err == nil {
		t.Errorf("expected error when applying invalid theme")
	}

	// Add a nil applier to cover target == nil check
	mgr.monitoringHotApply = append(mgr.monitoringHotApply, nil)

	// Test Apply with monitoring toggle change
	mockMonitoring.Called = false
	nextConfigToggle := files.RuntimeConfig{
		BotTheme:             "testtheme",
		DisableEntryExitLogs: true,
	}
	err = mgr.Apply(context.Background(), nextConfigToggle)
	if err != nil {
		t.Fatalf("expected nil error on monitoring toggle change apply, got: %v", err)
	}
	if !mockMonitoring.Called {
		t.Errorf("expected monitoring hot applier to be called when toggle changed")
	}
	if mockMonitoring.Config.DisableEntryExitLogs != true {
		t.Errorf("expected toggle change to be propagated")
	}

	// Test Apply when monitoring hot applier returns error
	mockMonitoring.Called = false
	mockMonitoring.Err = errors.New("monitoring apply error")
	nextConfigToggleErr := files.RuntimeConfig{
		BotTheme:             "testtheme",
		DisableEntryExitLogs: false, // toggle it back to trigger Apply
	}
	err = mgr.Apply(context.Background(), nextConfigToggleErr)
	if err == nil {
		t.Errorf("expected error when monitoring hot applier returns error")
	}

	// Test AddRuntime on a nil Manager does not panic
	var nilMgr *Manager
	nilMgr.AddRuntime(sm, mockMonitoring) // should return early
}

func TestMonitoringTogglesChanged(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		modify   func(*files.RuntimeConfig)
		expected bool
	}{
		{
			name:     "no changes",
			modify:   func(rc *files.RuntimeConfig) {},
			expected: false,
		},
		{
			name:     "DisableEntryExitLogs",
			modify:   func(rc *files.RuntimeConfig) { rc.DisableEntryExitLogs = true },
			expected: true,
		},
		{
			name:     "DisableMessageLogs",
			modify:   func(rc *files.RuntimeConfig) { rc.DisableMessageLogs = true },
			expected: true,
		},
		{
			name:     "DisableReactionLogs",
			modify:   func(rc *files.RuntimeConfig) { rc.DisableReactionLogs = true },
			expected: true,
		},
		{
			name:     "DisableUserLogs",
			modify:   func(rc *files.RuntimeConfig) { rc.DisableUserLogs = true },
			expected: true,
		},
		{
			name:     "DisableBotRolePermMirror",
			modify:   func(rc *files.RuntimeConfig) { rc.DisableBotRolePermMirror = true },
			expected: true,
		},
		{
			name:     "BotRolePermMirrorActorRoleID",
			modify:   func(rc *files.RuntimeConfig) { rc.BotRolePermMirrorActorRoleID = "new-role-id" },
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prev := files.RuntimeConfig{}
			next := files.RuntimeConfig{}
			tc.modify(&next)
			res := monitoringTogglesChanged(prev, next)
			if res != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, res)
			}
		})
	}
}
