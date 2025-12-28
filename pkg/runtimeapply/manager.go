package runtimeapply

import (
	"context"
	"fmt"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/util"
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
// This package is designed to be called after persisting settings.json updates.
// It assumes the caller already wrote the desired config to disk (or to the active
// ConfigManager) and just wants to apply the effect to the running process.
type Manager struct {
	mu sync.Mutex

	// serviceManager is optional; if nil, service-level hot-apply is skipped.
	serviceManager *service.ServiceManager

	// monitoringHotApply is optional; if nil, monitoring-level hot-apply is skipped.
	monitoringHotApply MonitoringHotApplier

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
	return &Manager{
		serviceManager:     sm,
		monitoringHotApply: monitoring,
		lastApplied:        files.RuntimeConfig{},
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
	if m.monitoringHotApply != nil {
		// Only call if any relevant toggle changed, to avoid churn.
		if monitoringTogglesChanged(prev, next) {
			if err := m.monitoringHotApply.ApplyRuntimeToggles(ctx, next); err != nil {
				return fmt.Errorf("apply monitoring toggles: %w", err)
			}
		}
	}

	// Apply service manager changes where feasible.
	//
	// NOTE: This code only handles services that are actually registered in ServiceManager.
	// In this repo today, `monitoring` is registered, `automod` may be registered depending
	// on startup gating.
	if m.serviceManager != nil {
		if prev.DisableAutomodLogs != next.DisableAutomodLogs {
			// If disabled => stop. If enabled => start.
			if next.DisableAutomodLogs {
				_ = m.serviceManager.StopService("automod")
			} else {
				_ = m.serviceManager.StartService("automod")
			}
		}

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
	return util.ConfigureThemeFromConfig(name)
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
