package moderation

import (
	"context"
)

// DiscordGateway define a interface de saída (Port) estrita para a API do Discord.
type DiscordGateway interface {
	ExecuteBan(ctx context.Context, bot *app.BotInstance, targetUserID string, reason string, deleteSeconds int) error
	ExecuteKick(ctx context.Context, bot *app.BotInstance, targetUserID string, reason string) error
}
