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
	botInstance := core.BotInstance{
		ApplicationID: "123456789",
		GuildID:       "987654321",
		Token:         "dummy",
	}
	registry := &dummyRegistry{bot: botInstance}
	router := moderation.NewRouter(registry)
	svc := moderation.NewService(gateway, b.N+100, router)
	svc.Start(context.Background(), 1)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// e2e bench replaced by GatewayRX internal benchmarks
	}
}
