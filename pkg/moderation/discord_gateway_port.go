package moderation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

// DiscordGateway define a interface de saída (Port) estrita para a API do Discord.
type DiscordGateway interface {
	ExecuteBan(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string, deleteSeconds int) error
	ExecuteKick(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string) error
}

var (
	ErrFeatureUnauthorized = errors.New("feature unauthorized: user lacks required permissions")
)

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("429 too many requests: retry after %v", e.RetryAfter)
}
