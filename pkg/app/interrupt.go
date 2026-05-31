package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

// WaitForInterrupt waits for an interrupt signal (SIGINT, SIGTERM)
// and blocks until one is received
func WaitForInterrupt() {
	waitForInterruptContext(context.Background(), nil)
}

// WaitForInterruptWithCallback waits for an interrupt signal and executes
// a callback function before returning
func WaitForInterruptWithCallback(callback func()) {
	waitForInterruptContext(context.Background(), callback)
}

// waitForInterruptContext allows tests to inject a context that can be cancelled without real OS signals.
func waitForInterruptContext(parent context.Context, callback func()) {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.ApplicationLogger().Info("Received interrupt; executing shutdown callback")

	if callback != nil {
		callback()
	}
}
