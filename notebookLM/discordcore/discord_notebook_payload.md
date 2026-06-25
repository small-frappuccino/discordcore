# Domain Architecture: discord

## Layout Topology
```text
discord/
└── moderation
    └── service.go
```

## Source Stream Aggregation

// === FILE: pkg/discord/moderation/service.go ===
```go
package moderation

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

// Client defines the subset of arikawa API operations required for moderation.
// Using an interface allows for strict transactional simulation via httptest.Server
// and granular mock injections during unit tests.
type Client interface {
	Ban(guildID discord.GuildID, userID discord.UserID, data api.BanData) error
	Kick(guildID discord.GuildID, userID discord.UserID, reason api.AuditLogReason) error
	ModifyMember(guildID discord.GuildID, userID discord.UserID, data api.ModifyMemberData) error
}

// Service provides high-level Discord moderation operations.
type Service struct {
	client Client
	logger *slog.Logger
}

// NewService instantiates a new moderation service using the provided arikawa client.
func NewService(client Client, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default() // Fallback safety, though strict DI is expected
	}
	return &Service{
		client: client,
		logger: logger,
	}
}

// Ban executes a guild ban against the target user.
// The context must be strictly respected to prevent dangling goroutines
// in the event of I/O failures.
func (s *Service) Ban(ctx context.Context, guildID discord.GuildID, userID discord.UserID, deleteMessageSeconds int, reason string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	data := api.BanData{
		DeleteDays: option.NewUint(uint(deleteMessageSeconds / 86400)),
	}

	s.logger.Debug("Granular transient state inspection: Executing ban payload",
		slog.String("guild_id", guildID.String()),
		slog.String("target_id", userID.String()),
		slog.Int("delete_days", deleteMessageSeconds/86400),
	)

	// Arikawa requires reason via audit log reason header, which is typically handled by WithContext and api.WithReason,
	// but for this abstract interface we assume the reason is either passed down or the caller wraps the context via api.WithReason.
	// Since we strictly enforce arikawa, the context should already carry the audit log reason.
	if err := s.client.Ban(guildID, userID, data); err != nil {
		s.logger.Warn("Mitigated service degradation: Ban execution rejected by network or permissions",
			slog.String("guild_id", guildID.String()),
			slog.String("target_id", userID.String()),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to execute ban: %w", err)
	}

	return nil
}

// Kick removes a user from the guild.
func (s *Service) Kick(ctx context.Context, guildID discord.GuildID, userID discord.UserID, reason api.AuditLogReason) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.logger.Debug("Granular transient state inspection: Executing kick payload",
		slog.String("guild_id", guildID.String()),
		slog.String("target_id", userID.String()),
	)

	if err := s.client.Kick(guildID, userID, reason); err != nil {
		s.logger.Warn("Mitigated service degradation: Kick execution rejected by network or permissions",
			slog.String("guild_id", guildID.String()),
			slog.String("target_id", userID.String()),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to execute kick: %w", err)
	}

	return nil
}

// Timeout applies a communication suspension to a member.
func (s *Service) Timeout(ctx context.Context, guildID discord.GuildID, userID discord.UserID, until discord.Timestamp) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	data := api.ModifyMemberData{
		CommunicationDisabledUntil: &until,
	}

	s.logger.Debug("Granular transient state inspection: Executing timeout payload",
		slog.String("guild_id", guildID.String()),
		slog.String("target_id", userID.String()),
		slog.Time("until", until.Time()),
	)

	if err := s.client.ModifyMember(guildID, userID, data); err != nil {
		s.logger.Warn("Mitigated service degradation: Timeout execution rejected by network or permissions",
			slog.String("guild_id", guildID.String()),
			slog.String("target_id", userID.String()),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to execute timeout: %w", err)
	}

	return nil
}

```

