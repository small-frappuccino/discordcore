package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestResolveDatabaseBootstrapFromEnv(t *testing.T) {
	t.Setenv(databaseDriverEnv, "postgres")
	t.Setenv(databaseURLEnv, "postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable")
	t.Setenv(databaseMaxOpenConnsEnv, "9")
	t.Setenv(databaseMaxIdleConnsEnv, "4")
	t.Setenv(databaseConnMaxLifetimeSecsEnv, "180")
	t.Setenv(databaseConnMaxIdleTimeSecsEnv, "45")
	t.Setenv(databasePingTimeoutMSEnv, "2500")

	bootstrap, err := resolveDatabaseBootstrap()
	if err != nil {
		t.Fatalf("resolve bootstrap from env: %v", err)
	}

	if bootstrap.Source != "env" {
		t.Fatalf("expected env bootstrap source, got %q", bootstrap.Source)
	}
	if got := bootstrap.Config.DatabaseURL; got != "postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable" {
		t.Fatalf("unexpected database url %q", got)
	}
	if got := bootstrap.Config.MaxOpenConns; got != 9 {
		t.Fatalf("expected max open conns 9, got %d", got)
	}
	if got := bootstrap.Config.PingTimeoutMS; got != 2500 {
		t.Fatalf("expected ping timeout 2500, got %d", got)
	}
}

func TestResolveDatabaseBootstrapRequiresEnv(t *testing.T) {
	t.Setenv(databaseDriverEnv, "")
	t.Setenv(databaseURLEnv, "")
	t.Setenv(databaseMaxOpenConnsEnv, "")
	t.Setenv(databaseMaxIdleConnsEnv, "")
	t.Setenv(databaseConnMaxLifetimeSecsEnv, "")
	t.Setenv(databaseConnMaxIdleTimeSecsEnv, "")
	t.Setenv(databasePingTimeoutMSEnv, "")

	_, err := resolveDatabaseBootstrap()
	if err == nil {
		t.Fatalf("expected missing bootstrap environment to fail")
	}
	if !strings.Contains(err.Error(), databaseURLEnv) {
		t.Fatalf("expected error to mention %s, got %v", databaseURLEnv, err)
	}
}

func TestStartupTaskOrchestrator_GoLight(t *testing.T) {
	orchestrator := NewStartupTaskOrchestrator(1)

	var executed int32
	var wg sync.WaitGroup
	wg.Add(1)

	orchestrator.GoLight("test_light", func(ctx context.Context) error {
		atomic.AddInt32(&executed, 1)
		wg.Done()
		return nil
	})

	wg.Wait()

	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("Expected GoLight task to execute exactly once")
	}

	if err := orchestrator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Unexpected error during shutdown: %v", err)
	}
}

func TestStartupTaskOrchestrator_GoHeavy(t *testing.T) {
	orchestrator := NewStartupTaskOrchestrator(2)

	var executed int32
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		orchestrator.GoHeavy("test_heavy", func(ctx context.Context) error {
			atomic.AddInt32(&executed, 1)
			wg.Done()
			return nil
		})
	}

	wg.Wait()

	if atomic.LoadInt32(&executed) != 2 {
		t.Errorf("Expected GoHeavy task to execute exactly twice")
	}

	if err := orchestrator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Unexpected error during shutdown: %v", err)
	}
}

func TestStartupTaskOrchestrator_ShutdownWithContextCancellation(t *testing.T) {
	orchestrator := NewStartupTaskOrchestrator(1)

	taskStarted := make(chan struct{})
	unblockTask := make(chan struct{})

	orchestrator.GoLight("blocking_task", func(ctx context.Context) error {
		close(taskStarted)
		select {
		case <-unblockTask:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	<-taskStarted

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := orchestrator.Shutdown(ctx)
	if err == nil {
		t.Errorf("Expected context cancellation error during shutdown")
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	close(unblockTask)
}

func TestStartupTaskOrchestrator_ShutdownTaskErrorSwallowed(t *testing.T) {
	orchestrator := NewStartupTaskOrchestrator(1)
	var wg sync.WaitGroup
	wg.Add(1)

	expectedErr := errors.New("simulated task error")

	orchestrator.GoHeavy("error_task", func(ctx context.Context) error {
		defer wg.Done()
		return expectedErr
	})

	wg.Wait()

	err := orchestrator.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected nil error because task errors are swallowed, got '%v'", err)
	}
}

func TestStartupTaskOrchestrator_GoNil(t *testing.T) {
	orchestrator := NewStartupTaskOrchestrator(1)
	orchestrator.GoLight("nil_task", nil)
	orchestrator.GoHeavy("nil_task", nil)

	var nilOrchestrator *StartupTaskOrchestrator
	nilOrchestrator.GoLight("nil_task", func(ctx context.Context) error { return nil })
	err := nilOrchestrator.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected nil error for nil orchestrator shutdown")
	}
}

func TestResolveParallelism(t *testing.T) {
	tests := []struct {
		name     string
		resolver func(int) int
		inputs   map[int]int
	}{
		{"RuntimeStartup", ResolveRuntimeStartupParallelism, map[int]int{0: 1, 1: 1, 2: 2, 3: 3, 10: 3}},
		{"RuntimeBackground", ResolveRuntimeBackgroundParallelism, map[int]int{0: 1, 1: 1, 2: 2, 10: 2}},
		{"StartupLight", ResolveStartupLightParallelism, map[int]int{0: 2, 1: 2, 2: 3, 3: 4, 10: 4}},
		{"StartupLightQueue", ResolveStartupLightQueueSize, map[int]int{0: 4, 1: 4, 2: 6, 3: 6, 10: 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for in, expected := range tt.inputs {
				if got := tt.resolver(in); got != expected {
					t.Errorf("%s(%d) = %d; want %d", tt.name, in, got, expected)
				}
			}
		})
	}
}

func TestControlServerHolder_SetAndStop(t *testing.T) {
	var h *controlServerHolder

	// Nil holder
	h.Set(&control.Server{})
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("expected nil error on nil holder stop, got %v", err)
	}

	h = &controlServerHolder{}
	h.Set(nil) // Should ignore
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("expected nil error on empty holder stop, got %v", err)
	}

	srv := &control.Server{}
	h.Set(srv)
}

func TestScheduleControlServerStartup(t *testing.T) {
	opts := controlStartupTaskOptions{
		runOptions: RunOptions{
			DisableControl: true,
		},
	}
	scheduleControlServerStartup(nil, opts)

	opts.runOptions.DisableControl = false
	scheduleControlServerStartup(nil, opts)
}

func TestScheduleStartupWebhookEmbedUpdates(t *testing.T) {
	scheduleStartupWebhookEmbedUpdates(nil, &files.BotConfig{}, func(g string) *discordgo.Session { return nil })
}

func TestStartControlServerStartupTask(t *testing.T) {
	runtimes := make(map[string]*botRuntime)
	resolver := newBotRuntimeResolver(files.NewConfigManagerWithStore(nil, nil), runtimes)
	opts := controlStartupTaskOptions{
		runOptions: RunOptions{
			DisableControl: false,
		},
		configManager:      files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil),
		runtimeResolver:    resolver,
		controlBearerToken: "test",
	}
	err := startControlServerStartupTask(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
