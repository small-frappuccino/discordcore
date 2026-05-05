package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestScheduleRuntimeWarmupWithoutWorkerRunsPhasesSequentially(t *testing.T) {
	origWarmup := intelligentWarmupFn
	origCache := monitoringUnifiedCacheFn
	origSchedule := scheduleStartupMemberWarmupFn
	t.Cleanup(func() {
		intelligentWarmupFn = origWarmup
		monitoringUnifiedCacheFn = origCache
		scheduleStartupMemberWarmupFn = origSchedule
	})

	var calls []cache.WarmupConfig
	intelligentWarmupFn = func(ctx context.Context, session *discordgo.Session, unifiedCache *cache.UnifiedCache, store *storage.Store, config cache.WarmupConfig) error {
		calls = append(calls, config)
		return nil
	}
	monitoringUnifiedCacheFn = func(ms *logging.MonitoringService) *cache.UnifiedCache {
		return &cache.UnifiedCache{}
	}
	scheduleStartupMemberWarmupFn = func(ms *logging.MonitoringService, config cache.WarmupConfig) bool {
		t.Fatalf("unexpected async member warmup scheduling")
		return false
	}

	runtime := &botRuntime{
		instanceID: "main",
		capabilities: botRuntimeCapabilities{
			warmup: true,
		},
		session:           &discordgo.Session{},
		monitoringService: &logging.MonitoringService{},
	}

	scheduleRuntimeWarmup(runtime, nil, nil)

	if len(calls) != 2 {
		t.Fatalf("expected 2 warmup phases, got %d", len(calls))
	}
	if calls[0].FetchMissingMembers {
		t.Fatalf("expected base phase to skip members")
	}
	if !calls[1].FetchMissingMembers || calls[1].FetchMissingGuilds || calls[1].FetchMissingRoles || calls[1].FetchMissingChannels {
		t.Fatalf("unexpected member phase config: %+v", calls[1])
	}
}

func TestScheduleRuntimeWarmupQueuesMemberPhaseAfterBasePhase(t *testing.T) {
	origWarmup := intelligentWarmupFn
	origCache := monitoringUnifiedCacheFn
	origSchedule := scheduleStartupMemberWarmupFn
	t.Cleanup(func() {
		intelligentWarmupFn = origWarmup
		monitoringUnifiedCacheFn = origCache
		scheduleStartupMemberWarmupFn = origSchedule
	})

	var mu sync.Mutex
	var baseCalls []cache.WarmupConfig
	var queued []cache.WarmupConfig
	baseDone := make(chan struct{}, 1)
	queueDone := make(chan struct{}, 1)
	intelligentWarmupFn = func(ctx context.Context, session *discordgo.Session, unifiedCache *cache.UnifiedCache, store *storage.Store, config cache.WarmupConfig) error {
		mu.Lock()
		baseCalls = append(baseCalls, config)
		mu.Unlock()
		baseDone <- struct{}{}
		return nil
	}
	monitoringUnifiedCacheFn = func(ms *logging.MonitoringService) *cache.UnifiedCache {
		return &cache.UnifiedCache{}
	}
	scheduleStartupMemberWarmupFn = func(ms *logging.MonitoringService, config cache.WarmupConfig) bool {
		mu.Lock()
		queued = append(queued, config)
		mu.Unlock()
		queueDone <- struct{}{}
		return true
	}

	runtime := &botRuntime{
		instanceID: "main",
		capabilities: botRuntimeCapabilities{
			warmup: true,
		},
		session:           &discordgo.Session{},
		monitoringService: &logging.MonitoringService{},
	}

	worker := newRuntimeStartupBackgroundWorker(1)
	startupTasks := &startupTaskOrchestrator{heavy: worker}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = worker.Shutdown(ctx)
	})

	scheduleRuntimeWarmup(runtime, nil, startupTasks)

	select {
	case <-baseDone:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for base warmup phase")
	}
	select {
	case <-queueDone:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for queued member phase")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown worker: %v", err)
	}

	if len(baseCalls) != 1 {
		t.Fatalf("expected 1 base warmup call, got %d", len(baseCalls))
	}
	if baseCalls[0].FetchMissingMembers {
		t.Fatalf("expected base phase to skip members")
	}
	if len(queued) != 1 {
		t.Fatalf("expected 1 queued member warmup, got %d", len(queued))
	}
	if !queued[0].FetchMissingMembers || queued[0].FetchMissingGuilds || queued[0].FetchMissingRoles || queued[0].FetchMissingChannels {
		t.Fatalf("unexpected queued member phase config: %+v", queued[0])
	}
}
