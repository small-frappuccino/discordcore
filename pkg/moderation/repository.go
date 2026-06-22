package moderation

import (
	"context"
	"iter"
	"time"
)

type Repository interface {
	NextModerationCaseNumber(ctx context.Context, guildID string) (int64, error)
	CreateModerationWarning(ctx context.Context, guildID, userID, moderatorID, reason string, createdAt time.Time) (Warning, error)
	ListModerationWarnings(ctx context.Context, guildID, userID string, limit int) iter.Seq2[Warning, error]
	SetGuildOwnerID(ctx context.Context, guildID, ownerID string) error
	GetGuildOwnerID(ctx context.Context, guildID string) (string, bool, error)
}
