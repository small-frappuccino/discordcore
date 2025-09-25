package util

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/alice-bnuy/discordcore/pkg/logging"
)

// WaitForInterrupt waits for an interrupt signal (SIGINT, SIGTERM)
// and blocks until one is received
func WaitForInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received
	sig := <-c
	logging.WithField("signal", sig.String()).Info("Received interrupt signal")
}

// WaitForInterruptWithCallback waits for an interrupt signal and executes
// a callback function before returning
func WaitForInterruptWithCallback(callback func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received
	sig := <-c
	logging.WithField("signal", sig.String()).Info("Received interrupt signal")

	if callback != nil {
		callback()
	}
}
