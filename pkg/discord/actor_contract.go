package discord

import (
	"sync"
)

// GatewayEvent represents a single, decoded WebSocket payload.
// Memory Ownership: Produced by Gateway-RX. Ownership is TRANSFERRED to the Actor.
// The Actor MUST call Release() when processing is complete.
type GatewayEvent struct {
	Op        int
	Seq       int64
	Type      string
	GuildID   uint64
	Data      []byte  // Sliced from a pooled buffer
	bufferPtr *[]byte // Pointer to the pooled buffer to prevent allocation on Release
}

var eventPool = sync.Pool{
	New: func() any {
		return &GatewayEvent{}
	},
}

// AcquireEvent returns a zeroed GatewayEvent from the pool.
func AcquireEvent() *GatewayEvent {
	evt := eventPool.Get().(*GatewayEvent)
	// Zero state
	evt.Op = -1
	evt.Seq = 0
	evt.Type = ""
	evt.GuildID = 0
	evt.Data = nil
	evt.bufferPtr = nil
	return evt
}

// Release returns the event to the pool.
func (e *GatewayEvent) Release() {
	if e.bufferPtr != nil {
		PayloadPool.Put(e.bufferPtr)
		e.Data = nil
		e.bufferPtr = nil
	}
	eventPool.Put(e)
}

// ActorInbox is the fan-out sink contract owned by Gateway-RX.
type ActorInbox interface {
	// EnqueueEvent delivers an event to the actor.
	// Must implement explicit load-shedding if the inbox is full, returning an error.
	// Must NEVER block the caller (Gateway-RX).
	EnqueueEvent(evt *GatewayEvent) error
}

// ActorDirectory provides O(1) routing from guild_id to the responsible Actor.
type ActorDirectory interface {
	// Route returns the inbox for the given guild ID.
	// If the guild is not found, it may return nil.
	Route(guildID uint64) ActorInbox
	// SystemRoute returns the inbox for guildless/system events.
	SystemRoute() ActorInbox
}
