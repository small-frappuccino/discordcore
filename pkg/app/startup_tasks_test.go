package app

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

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

	// Cannot easily verify Stop side effects without deep mocking of control.Server,
	// but we can ensure it doesn't panic.
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
