package control

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type GuildEvent struct {
	GuildID    string `json:"guild_id"`
	BotPresent bool   `json:"bot_present"`
}

type guildEventBroker struct {
	mu          sync.RWMutex
	subscribers map[chan GuildEvent]struct{}
}

func newGuildEventBroker() *guildEventBroker {
	return &guildEventBroker{
		subscribers: make(map[chan GuildEvent]struct{}),
	}
}

func (b *guildEventBroker) Subscribe() chan GuildEvent {
	ch := make(chan GuildEvent, 100)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *guildEventBroker) Unsubscribe(ch chan GuildEvent) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *guildEventBroker) Broadcast(event GuildEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Non-blocking send; if channel is full, we drop the event for this slow client.
		}
	}
}

func (s *Server) BroadcastGuildEvent(guildID string, botPresent bool) {
	if s == nil || s.guildEventBroker == nil {
		return
	}
	s.guildEventBroker.Broadcast(GuildEvent{
		GuildID:    guildID,
		BotPresent: botPresent,
	})
}

func (s *Server) handleGuildEvents(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.guildEventBroker.Subscribe()
	defer s.guildEventBroker.Unsubscribe(ch)

	// Send an initial empty comment to ensure the connection is established
	fmt.Fprintf(w, ": heartbeat\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-ch:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
