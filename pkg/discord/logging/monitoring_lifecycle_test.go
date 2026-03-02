package logging

import (
	"context"
	stdErrors "errors"
	"path/filepath"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

func TestMonitoringServiceRestartRebuildsTaskPipeline(t *testing.T) {
	store, _ := newLoggingStore(t, "monitoring-restart.db")

	cfgMgr := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	if _, err := cfgMgr.UpdateRuntimeConfig(func(rc *files.RuntimeConfig) error {
		rc.DisableEntryExitLogs = true
		rc.DisableMessageLogs = true
		rc.DisableReactionLogs = true
		return nil
	}); err != nil {
		t.Fatalf("update runtime config: %v", err)
	}

	session := &discordgo.Session{State: discordgo.NewState()}
	session.State.User = &discordgo.User{ID: "bot-1"}

	ms, err := NewMonitoringService(session, cfgMgr, store)
	if err != nil {
		t.Fatalf("new monitoring service: %v", err)
	}

	firstRouter := ms.TaskRouter()
	if firstRouter == nil {
		t.Fatalf("expected initial task router")
	}
	if ms.adapters == nil || ms.adapters.Router != firstRouter {
		t.Fatalf("expected adapters to point at initial router")
	}
	if ms.messageEventService == nil || ms.messageEventService.taskRouter != firstRouter {
		t.Fatalf("expected message event service to point at initial router")
	}

	if err := ms.Start(context.Background()); err != nil {
		t.Fatalf("start monitoring service: %v", err)
	}
	if err := ms.Stop(context.Background()); err != nil {
		t.Fatalf("stop monitoring service: %v", err)
	}

	if err := firstRouter.Dispatch(context.Background(), task.Task{Type: "monitor.update_stats_channels"}); !stdErrors.Is(err, task.ErrRouterClosed) {
		t.Fatalf("expected old router to be closed, got %v", err)
	}

	if err := ms.Start(context.Background()); err != nil {
		t.Fatalf("restart monitoring service: %v", err)
	}
	t.Cleanup(func() {
		if ms.isRunning {
			_ = ms.Stop(context.Background())
		}
	})

	secondRouter := ms.TaskRouter()
	if secondRouter == nil {
		t.Fatalf("expected router after restart")
	}
	if secondRouter == firstRouter {
		t.Fatalf("expected restart to create a fresh router")
	}
	if ms.adapters == nil || ms.adapters.Router != secondRouter {
		t.Fatalf("expected adapters to be rewired to restarted router")
	}
	if ms.messageEventService == nil || ms.messageEventService.taskRouter != secondRouter {
		t.Fatalf("expected message event service router to be refreshed on restart")
	}
	if err := secondRouter.Dispatch(context.Background(), task.Task{Type: "monitor.update_stats_channels"}); err != nil {
		t.Fatalf("dispatch on restarted router: %v", err)
	}
}
