package logging

import (
	"context"
	"errors"
	"fmt"
)

type subServiceEntry struct {
	name        string
	shouldStart bool
	start       func() error
	stop        func() error
	isRunning   func() bool
}

func (ms *MonitoringService) buildSubServiceEntries(ctx context.Context, workload monitoringWorkloadState) []subServiceEntry {
	return []subServiceEntry{
		{
			name:        "member_event_service",
			shouldStart: workload.memberEventService,
			start: func() error {
				if ms.memberEventService != nil && !ms.memberEventService.IsRunning() {
					return ms.memberEventService.Start(ctx)
				}
				return nil
			},
			stop: func() error {
				if ms.memberEventService != nil && ms.memberEventService.IsRunning() {
					return ms.memberEventService.Stop(ctx)
				}
				return nil
			},
			isRunning: func() bool {
				return ms.memberEventService != nil && ms.memberEventService.IsRunning()
			},
		},
		{
			name:        "message_event_service",
			shouldStart: workload.messageEventService,
			start: func() error {
				if ms.messageEventService != nil && !ms.messageEventService.IsRunning() {
					return ms.messageEventService.Start(ctx)
				}
				return nil
			},
			stop: func() error {
				if ms.messageEventService != nil && ms.messageEventService.IsRunning() {
					return ms.messageEventService.Stop(ctx)
				}
				return nil
			},
			isRunning: func() bool {
				return ms.messageEventService != nil && ms.messageEventService.IsRunning()
			},
		},
		{
			name:        "reaction_event_service",
			shouldStart: workload.reactionEventService,
			start: func() error {
				if ms.reactionEventService == nil {
					ms.reactionEventService = NewReactionEventServiceForBot(ms.session, ms.configManager, ms.store, ms.botInstanceID, ms.defaultBotInstanceID, ms.logger)
				}
				if !ms.reactionEventService.IsRunning() {
					return ms.reactionEventService.Start(ctx)
				}
				return nil
			},
			stop: func() error {
				if ms.reactionEventService != nil && ms.reactionEventService.IsRunning() {
					return ms.reactionEventService.Stop(ctx)
				}
				return nil
			},
			isRunning: func() bool {
				return ms.reactionEventService != nil && ms.reactionEventService.IsRunning()
			},
		},
	}
}

// startSubServices starts member, message, and reaction event services in
// dependency order. On failure, it rolls back all previously started services
// in reverse order, returning the combined error.
func (ms *MonitoringService) startSubServices(ctx context.Context, workload monitoringWorkloadState) error {
	entries := ms.buildSubServiceEntries(ctx, workload)

	var startErrs []error
	var startedIdx = -1

	for i, entry := range entries {
		if !entry.shouldStart {
			continue
		}
		opName := fmt.Sprintf("monitoring.start.%s", entry.name) // Use a generic name, or split to exact names
		err := startMonitoringSubService(ctx, opName, entry.name, entry.start)
		if err != nil {
			startErrs = append(startErrs, fmt.Errorf("failed to start %s: %w", entry.name, err))
			// Stop previously started services in reverse order
			for j := startedIdx; j >= 0; j-- {
				if entries[j].isRunning() {
					stopOpName := fmt.Sprintf("monitoring.start.cleanup.stop_%s_after_%s_start_failure", entries[j].name, entry.name)
					stopErr := stopMonitoringSubService(ctx, stopOpName, entries[j].name, entries[j].stop)
					if stopErr != nil {
						startErrs = append(startErrs, fmt.Errorf("cleanup stop of %s failed: %w", entries[j].name, stopErr))
					}
				}
			}
			return errors.Join(startErrs...)
		}
		startedIdx = i
	}
	return nil
}

// stopSubServices stops member, message, and reaction event services in
// reverse dependency order, collecting errors.
func (ms *MonitoringService) stopSubServices(ctx context.Context) []error {
	var stopErrs []error
	// empty workload because we're just stopping
	entries := ms.buildSubServiceEntries(ctx, monitoringWorkloadState{})
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.isRunning() {
			opName := fmt.Sprintf("monitoring.stop.%s", entry.name)
			if err := stopMonitoringSubService(ctx, opName, entry.name, entry.stop); err != nil {
				stopErrs = append(stopErrs, err)
			}
		}
	}
	return stopErrs
}

func (ms *MonitoringService) applySubServiceToggles(ctx context.Context, workload monitoringWorkloadState) []error {
	var errs []error
	entries := ms.buildSubServiceEntries(ctx, workload)

	// Stop services that should no longer run
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.shouldStart && entry.isRunning() {
			opName := fmt.Sprintf("monitoring.apply_runtime_toggles.stop_%s", entry.name)
			if err := stopMonitoringSubService(ctx, opName, entry.name, entry.stop); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Start services that should run but aren't
	for _, entry := range entries {
		if entry.shouldStart && !entry.isRunning() {
			opName := fmt.Sprintf("monitoring.apply_runtime_toggles.start_%s", entry.name)
			if err := startMonitoringSubService(ctx, opName, entry.name, entry.start); err != nil {
				errs = append(errs, fmt.Errorf("start %s: %w", entry.name, err))
			}
		}
	}

	return errs
}
