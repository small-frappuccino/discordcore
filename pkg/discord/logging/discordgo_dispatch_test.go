package logging

// # Contract: Unsafe and Linkname Usage for Testing
// The `unsafe` package is imported here solely to satisfy the Go compiler's requirement
// for using `//go:linkname`. We use `go:linkname` as a testing backdoor (Test Double)
// to access the unexported `handleEvent` method of `discordgo.Session`. This allows
// our test suites to synchronously inject and process mock Discord websocket events
// without needing an actual network connection or a patched upstream library.
// This file is strictly constrained to `_test.go` and does not leak `unsafe` into production logic.

import (
	_ "unsafe"

	"github.com/bwmarrin/discordgo"
)

//go:linkname discordgoHandleEvent github.com/bwmarrin/discordgo.(*Session).handleEvent
func discordgoHandleEvent(session *discordgo.Session, eventType string, payload any)

func dispatchDiscordEvent(session *discordgo.Session, eventType string, payload any) {
	if session == nil {
		return
	}
	session.SyncEvents = true
	discordgoHandleEvent(session, eventType, payload)
}
