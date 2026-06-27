package moderation

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

type dummyRegistry struct {
	bot *core.BotInstance
}

func (d *dummyRegistry) ResolveOwner(ctx context.Context, guildID string, featureName string) (*core.BotInstance, error) {
	return d.bot, nil
}

type dummyGateway struct{}

func (d *dummyGateway) ExecuteBan(ctx context.Context, bot *core.BotInstance, targetUserID uint64, reason string, deleteSeconds int) error {
	return nil
}

func (d *dummyGateway) ExecuteKick(ctx context.Context, bot *core.BotInstance, targetUserID uint64, reason string) error {
	return nil
}

func BenchmarkRouter_HandleInteraction_ZeroAlloc(b *testing.B) {
	gateway := &dummyGateway{}
	svc := NewService(gateway, 100)
	svc.Start(context.Background(), 1)

	registry := &dummyRegistry{
		bot: &core.BotInstance{
			ApplicationID: "123456789",
			GuildID:       "987654321",
			Token:         "dummy",
		},
	}
	router := NewRouter(registry, svc)

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

	interaction := core.InteractionPayload{
		Data: payload,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = router.HandleInteraction(context.Background(), interaction)
	}
}

func BenchmarkService_ActorInbox(b *testing.B) {
	gateway := &dummyGateway{}
	svc := NewService(gateway, b.N)
	svc.Start(context.Background(), 1)

	job := ModerationJob{
		Bot:          &core.BotInstance{GuildID: "987654321"},
		Action:       ActionBan,
		TargetUserID: 111222333,
		Reason:       "spam",
		DeleteDays:   7,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = svc.EnqueueTask(job)
	}
}
