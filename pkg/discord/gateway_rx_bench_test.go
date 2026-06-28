package discord

import (
	"testing"
)

type mockDirectory struct {
	inbox ActorInbox
}

func (m *mockDirectory) Route(guildID uint64) ActorInbox {
	return m.inbox
}

func (m *mockDirectory) SystemRoute() ActorInbox {
	return m.inbox
}

type mockInbox struct{}

func (m *mockInbox) EnqueueEvent(evt *GatewayEvent) error {
	evt.Release() // Release immediately back to pool in benchmark
	return nil
}

func BenchmarkGatewayRX_DispatchRoute(b *testing.B) {
	// A captured DISPATCH frame simulating an interaction create
	payload := []byte(`{"op":0,"s":1024,"t":"INTERACTION_CREATE","d":{"guild_id":"123456789012345678","type":2,"id":"987654321098765432","application_id":"112233445566778899","data":{"name":"ban"}}}`)

	dir := &mockDirectory{inbox: &mockInbox{}}
	rx := NewGatewayRX(nil, dir)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// processPayload includes envelope sniff, routing, and transferring ownership.
		err := rx.processPayload(payload)
		if err != nil {
			b.Fatalf("processPayload failed: %v", err)
		}
	}
}
