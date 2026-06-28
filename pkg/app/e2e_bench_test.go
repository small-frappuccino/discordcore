package app

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/core"
	"github.com/small-frappuccino/discordcore/pkg/moderation"
)

type dummyGateway struct{}

func (d *dummyGateway) ExecuteBan(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string, deleteSeconds int) error {
	return nil
}

func (d *dummyGateway) ExecuteKick(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string) error {
	return nil
}

type dummyRegistry struct {
	bot core.BotInstance
}

func (d *dummyRegistry) ResolveOwner(ctx context.Context, guildID string, feature core.Feature) (core.BotInstance, error) {
	return d.bot, nil
}

// BenchmarkE2E_IngressPipeline_ZeroAlloc tests the full path: Gateway Ingress -> Router -> Actor Inbox
func BenchmarkE2E_IngressPipeline_ZeroAlloc(b *testing.B) {
	b.ReportAllocs()

	gateway := &dummyGateway{}
	// Large queue to absorb burst
	svc := moderation.NewService(gateway, b.N+100)
	svc.Start(context.Background(), 1)

	botInstance := core.BotInstance{
		ApplicationID: "123456789",
		GuildID:       "987654321",
		Token:         "dummy",
	}

	registry := &dummyRegistry{bot: botInstance}
	router := moderation.NewRouter(registry, svc)

	// Mocking Gateway Connection
	messages := make(chan []byte, b.N)
	conn := &MockConnection{messages: messages}

	payload := []byte(`{
		"guild_id": "987654321",
		"application_id": "123456789",
		"data": {
			"name": "ban",
			"options": [
				{"name": "target_id", "value": "111222333"},
				{"name": "delete_days", "value": 7},
				{"name": "reason", "value": "spam"}
			]
		}
	}`)

	for i := 0; i < b.N; i++ {
		messages <- payload
	}
	close(messages)

	g := NewDiscordGatewayImpl(conn, 3)
	// Inject the moderation router as the handler
	g.OnInteraction(router)

	ctx := context.Background()

	b.ResetTimer()

	// Execute the E2E pipeline
	_ = g.ListenLoop(ctx)
}
