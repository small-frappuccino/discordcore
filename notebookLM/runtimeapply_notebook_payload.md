# Domain Architecture: runtimeapply

## Layout Topology
```text
runtimeapply/
├── manager.go
└── manager_test.go
```

## Source Stream Aggregation

// === FILE: pkg/runtimeapply/manager.go ===
```go
package runtimeapply

import (
	"context"
	"fmt"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
)

// Manager applies a subset of runtime configuration changes immediately (hot-apply)
// without requiring a full process restart.
//
// Scope (requested):
// - ALICE_BOT_THEME: apply theme in-process
// - ALICE_DISABLE_*: start/stop services and handlers when feasible
//
// Non-goals:
// - DB path / cache persist interval / backfill: intentionally not handled here
// - Message cache/versioning: intentionally not handled here
//
// This package is designed to be called after persisting runtime config updates.
// It assumes the caller already wrote the desired config to the active store (or to the active
// ConfigManager) and just wants to apply the effect to the running process.
type Manager struct {
	mu sync.Mutex

	// serviceManagers are optional; when empty, service-level hot-apply is skipped.
	serviceManagers []*service.ServiceManager

	// monitoringHotApply targets are optional; when empty, monitoring hot-apply is skipped.
	monitoringHotApply []MonitoringHotApplier

	// lastApplied is used to compute diffs. It should be initialized from config
	// during startup via SetInitial().
	lastApplied files.RuntimeConfig
}

// MonitoringHotApplier abstracts the subset of MonitoringService behavior we need
// for hot-applying runtime toggles (so this package does not import logging directly).
//
// This is intentionally small and focused on the ALICE_DISABLE_* runtime toggles that
// map to monitoring sub-services / event handlers.
type MonitoringHotApplier interface {
	ApplyRuntimeToggles(ctx context.Context, rc files.RuntimeConfig) error
}

// New creates a Manager. Any dependency can be nil; unsupported apply steps will be skipped.
func New(sm *service.ServiceManager, monitoring MonitoringHotApplier) *Manager {
	m := &Manager{lastApplied: files.RuntimeConfig{}}
	m.AddRuntime(sm, monitoring)
	return m
}

// AddRuntime registers another runtime target for hot-apply. Nil dependencies
// are ignored so callers can add runtimes progressively during startup.
func (m *Manager) AddRuntime(sm *service.ServiceManager, monitoring MonitoringHotApplier) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if sm != nil {
		m.serviceManagers = append(m.serviceManagers, sm)
	}
	if monitoring != nil {
		m.monitoringHotApply = append(m.monitoringHotApply, monitoring)
	}
}

// SetInitial sets the baseline config used for diffing. Call once at startup, after config load.
// If you don't call it, Apply() will still apply idempotently, but diffs may be less accurate.
func (m *Manager) SetInitial(rc files.RuntimeConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastApplied = rc
}

// Apply applies relevant changes between the last applied config and next.
// It updates the baseline if all apply steps succeed.
//
// If partial apply failure happens, it returns an error and does NOT update the baseline.
func (m *Manager) Apply(ctx context.Context, next files.RuntimeConfig) error {
	m.mu.Lock()
	prev := m.lastApplied
	m.mu.Unlock()

	// Apply theme if it changed
	if prev.BotTheme != next.BotTheme {
		if err := applyTheme(next.BotTheme); err != nil {
			return fmt.Errorf("apply theme: %w", err)
		}
	}

	// Apply monitoring toggles (entry/exit, message logs, reaction logs, user logs, perm mirror)
	// This is done through an interface so the logging package can implement it without cycles.
	if len(m.monitoringHotApply) > 0 {
		// Only call if any relevant toggle changed, to avoid churn.
		if monitoringTogglesChanged(prev, next) {
			for _, target := range m.monitoringHotApply {
				if target == nil {
					continue
				}
				if err := target.ApplyRuntimeToggles(ctx, next); err != nil {
					return fmt.Errorf("apply monitoring toggles: %w", err)
				}
			}
		}
	}

	// Apply service manager changes where feasible.
	//
	// NOTE: This code only handles services that are actually registered in ServiceManager.
	// In this repo today, `monitoring` is registered, `automod` may be registered depending
	// on startup gating.
	if len(m.serviceManagers) > 0 {
		// Removed DisableAutomodLogs check.
		// DB cleanup is not a service wrapper in ServiceManager; it is a goroutine in app runner,
		// so we intentionally do nothing here.
	}

	m.mu.Lock()
	m.lastApplied = next
	m.mu.Unlock()

	return nil
}

func applyTheme(name string) error {
	// Theme is purely in-process state (theme.SetCurrent under the hood).
	// Empty resets to default.
	return files.ConfigureThemeFromConfig(name)
}

func monitoringTogglesChanged(prev, next files.RuntimeConfig) bool {
	// These are all the toggles that affect MonitoringService sub-systems.
	if prev.DisableEntryExitLogs != next.DisableEntryExitLogs {
		return true
	}
	if prev.DisableMessageLogs != next.DisableMessageLogs {
		return true
	}
	if prev.DisableReactionLogs != next.DisableReactionLogs {
		return true
	}
	if prev.DisableUserLogs != next.DisableUserLogs {
		return true
	}
	if prev.DisableBotRolePermMirror != next.DisableBotRolePermMirror {
		return true
	}
	// Actor role ID changes are meaningful for perm mirroring behavior.
	if prev.BotRolePermMirrorActorRoleID != next.BotRolePermMirrorActorRoleID {
		return true
	}
	return false
}

```

// === FILE: pkg/runtimeapply/manager_test.go ===
```go
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

```

