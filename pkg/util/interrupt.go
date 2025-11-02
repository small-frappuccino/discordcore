package util

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// WaitForInterrupt waits for an interrupt signal (SIGINT, SIGTERM)
// and blocks until one is received
func WaitForInterrupt() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Printf("Received interrupt; shutting down")
}

// WaitForInterruptWithCallback waits for an interrupt signal and executes
// a callback function before returning
func WaitForInterruptWithCallback(callback func()) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Printf("Received interrupt; executing shutdown callback")

	if callback != nil {
		callback()
	}
}
