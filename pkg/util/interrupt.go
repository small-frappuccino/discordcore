package util

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// WaitForInterrupt waits for an interrupt signal (SIGINT, SIGTERM)
// and blocks until one is received
func WaitForInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received
	sig := <-c
	log.Printf("Received interrupt signal: %s", sig.String())
}

// WaitForInterruptWithCallback waits for an interrupt signal and executes
// a callback function before returning
func WaitForInterruptWithCallback(callback func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received
	sig := <-c
	log.Printf("Received interrupt signal: %s", sig.String())

	if callback != nil {
		callback()
	}
}
