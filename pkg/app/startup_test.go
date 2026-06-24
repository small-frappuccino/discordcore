package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
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

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected missing bootstrap environment to panic")
		}
		if !strings.Contains(fmt.Sprintf("%v", r), databaseURLEnv) {
			t.Fatalf("expected panic to mention %s, got %v", databaseURLEnv, r)
		}
	}()
	resolveDatabaseBootstrap()
}

type MockTask struct {
	name string
	exec func(context.Context) error
}

func (m MockTask) Name() string { return m.name }

func (m MockTask) Execute(ctx context.Context) error {
	if m.exec != nil {
		return m.exec(ctx)
	}
	return nil
}

func TestStartupTaskOrchestrator_Go(t *testing.T) {
	t.Parallel()
	orchestrator := NewStartupTaskOrchestrator(context.Background(), 2)

	var executed int32
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		orchestrator.Go(MockTask{
			name: "test_task",
			exec: func(ctx context.Context) error {
				atomic.AddInt32(&executed, 1)
				wg.Done()
				return nil
			},
		})
	}

	wg.Wait()

	if atomic.LoadInt32(&executed) != 2 {
		t.Errorf("Expected Go task to execute exactly twice")
	}

	if err := orchestrator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Unexpected error during shutdown: %v", err)
	}
}

func TestStartupTaskOrchestrator_ShutdownWithContextCancellation(t *testing.T) {
	t.Parallel()
	orchestrator := NewStartupTaskOrchestrator(context.Background(), 1)

	taskStarted := make(chan struct{})
	unblockTask := make(chan struct{})

	orchestrator.Go(MockTask{
		name: "blocking_task",
		exec: func(ctx context.Context) error {
			close(taskStarted)
			<-unblockTask
			return nil
		},
	})

	<-taskStarted

	// Allow the task to finish so Shutdown doesn't block forever
	close(unblockTask)

	err := orchestrator.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected deterministic shutdown to return nil, got %v", err)
	}
}

func TestStartupTaskOrchestrator_ShutdownTaskErrorPropagates(t *testing.T) {
	t.Parallel()
	orchestrator := NewStartupTaskOrchestrator(context.Background(), 1)
	var wg sync.WaitGroup
	wg.Add(1)

	expectedErr := errors.New("simulated task error")

	orchestrator.Go(MockTask{
		name: "error_task",
		exec: func(ctx context.Context) error {
			defer wg.Done()
			return expectedErr
		},
	})

	wg.Wait()

	err := orchestrator.Shutdown(context.Background())
	if err == nil {
		t.Errorf("Expected an error because task errors are propagated, got nil")
	} else if !errors.Is(err, expectedErr) {
		t.Errorf("Expected simulated task error, got: %v", err)
	}
}

func TestStartupTaskOrchestrator_GoNil(t *testing.T) {
	t.Parallel()
	orchestrator := NewStartupTaskOrchestrator(context.Background(), 1)
	orchestrator.Go(MockTask{name: "nil_task", exec: nil})

	var nilOrchestrator *StartupTaskOrchestrator
	nilOrchestrator.Go(MockTask{name: "nil_task", exec: func(ctx context.Context) error { return nil }})
	err := nilOrchestrator.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected nil error for nil orchestrator shutdown")
	}
}

func TestResolveParallelism(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		resolver func(int) int
		inputs   map[int]int
	}{
		{"RuntimeStartup", ResolveRuntimeStartupParallelism, map[int]int{0: 1, 1: 1, 2: 2, 3: 3, 10: 3}},
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
	t.Parallel()
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
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected scheduleControlServerStartup to panic with nil startupTasks")
		}
	}()
	scheduleControlServerStartup(nil, resolvedControlRuntime{}, nil, nil, nil)
}

type mockSessionResolver struct{}

func (m mockSessionResolver) SessionForGuild(guildID string, feature string) (*session.LegacySession, error) {
	return nil, nil
}

func TestScheduleStartupWebhookEmbedUpdates(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected scheduleStartupWebhookEmbedUpdates to panic with nil startupTasks")
		}
	}()
	scheduleStartupWebhookEmbedUpdates(nil, &files.BotConfig{}, mockSessionResolver{})
}

func TestStartControlServerStartupTask(t *testing.T) {
	t.Parallel()
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	controlRuntime := resolvedControlRuntime{
		bindAddr: "127.0.0.1:0",
	}

	err := startControlServerStartupTask(context.Background(), controlRuntime, cfgMgr, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
