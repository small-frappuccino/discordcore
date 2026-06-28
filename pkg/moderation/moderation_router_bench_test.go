package moderation

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/core"
	"github.com/small-frappuccino/discordcore/pkg/discord"
)

type dummyRegistry struct {
	bot core.BotInstance
}

func (d *dummyRegistry) ResolveOwner(ctx context.Context, guildID string, feature core.Feature) (core.BotInstance, error) {
	return d.bot, nil
}

type dummyGateway struct{}

func (d *dummyGateway) ExecuteBan(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string, deleteSeconds int) error {
	return nil
}

func (d *dummyGateway) ExecuteKick(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string) error {
	return nil
}

func BenchmarkRouter_HandleInteraction_ZeroAlloc(b *testing.B) {
	registry := &dummyRegistry{
		bot: core.BotInstance{
			ApplicationID: "123456789",
			GuildID:       "987654321",
			Token:         "dummy",
		},
	}
	router := NewRouter(registry)

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

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		seq, err := router.ParseInteraction(context.Background(), payload)
		if err == nil {
			seq.All(func(job ModerationJob) bool {
				_ = job
				return true
			})
		}
	}
}

func BenchmarkService_ActorInbox(b *testing.B) {
	gateway := &dummyGateway{}
	registry := &dummyRegistry{
		bot: core.BotInstance{
			ApplicationID: "123456789",
			GuildID:       "987654321",
			Token:         "dummy",
		},
	}
	router := NewRouter(registry)
	svc := NewService(gateway, b.N, router)
	svc.Start(context.Background(), 1)

	actor := newGuildActor(svc.ctx, svc.eg, "987654321", gateway, b.N, router)

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

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		evt := discord.AcquireEvent()
		evt.Type = "INTERACTION_CREATE"
		// To avoid retaining the payload in the pool for this bench, just point it.
		// In reality, RX copies to a pooled buffer. For the bench of the ActorInbox, this is fine.
		evt.Data = payload
		_ = actor.EnqueueEvent(evt)
	}
}
