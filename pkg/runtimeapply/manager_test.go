package runtimeapply

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
)

func TestManagerApply_IgnoresAutomodServiceStartStopErrors(t *testing.T) {
	serviceManager := service.NewServiceManager()
	manager := New(serviceManager, nil)
	manager.SetInitial(files.RuntimeConfig{DisableAutomodLogs: false})

	if err := manager.Apply(context.Background(), files.RuntimeConfig{DisableAutomodLogs: true}); err != nil {
		t.Fatalf("apply disable automod logs: %v", err)
	}
	if !manager.lastApplied.DisableAutomodLogs {
		t.Fatalf("expected lastApplied.DisableAutomodLogs=true after disable apply")
	}

	if err := manager.Apply(context.Background(), files.RuntimeConfig{DisableAutomodLogs: false}); err != nil {
		t.Fatalf("apply enable automod logs: %v", err)
	}
	if manager.lastApplied.DisableAutomodLogs {
		t.Fatalf("expected lastApplied.DisableAutomodLogs=false after enable apply")
	}
}
