package storage

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
)

// ModerationWarning is a stored warning against a user in a guild.
type ModerationWarning struct {
	ID          int64
	GuildID     string
	UserID      string
	CaseNumber  int64
	ModeratorID string
	Reason      string
	CreatedAt   time.Time
}

// NextModerationCaseNumber atomically increments and returns the next case number.
func (s *Store) NextModerationCaseNumber(ctx context.Context, guildID string) (int64, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return 0, fmt.Errorf("guildID is empty")
	}

	var next int64
	err := s.db.QueryRow(ctx,
		`INSERT INTO moderation_cases (guild_id, last_case_number)
         VALUES ($1, 1)
         ON CONFLICT(guild_id) DO UPDATE
         SET last_case_number = moderation_cases.last_case_number + 1
         RETURNING last_case_number`,
		guildID,
	).Scan(&next)
	if err != nil {
		return 0, err
	}
	return next, nil
}

// CreateModerationWarning creates a moderation warning transactionally.
func (s *Store) CreateModerationWarning(ctx context.Context, guildID, userID, moderatorID, reason string, createdAt time.Time) (warning ModerationWarning, err error) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	moderatorID = strings.TrimSpace(moderatorID)
	reason = strings.TrimSpace(reason)
	if guildID == "" || userID == "" || moderatorID == "" || reason == "" {
		return ModerationWarning{}, fmt.Errorf("missing required fields for warning")
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	} else {
		createdAt = createdAt.UTC()
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return ModerationWarning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	var caseNumber int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO moderation_cases (guild_id, last_case_number)
         VALUES ($1, 1)
         ON CONFLICT(guild_id) DO UPDATE
         SET last_case_number = moderation_cases.last_case_number + 1
         RETURNING last_case_number`,
		guildID,
	).Scan(&caseNumber); err != nil {
		return ModerationWarning{}, err
	}

	warning = ModerationWarning{
		GuildID:     guildID,
		UserID:      userID,
		CaseNumber:  caseNumber,
		ModeratorID: moderatorID,
		Reason:      reason,
		CreatedAt:   createdAt,
	}

	if err := tx.QueryRow(ctx,
		`INSERT INTO moderation_warnings (id, guild_id, user_id, case_number, moderator_id, reason, created_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7)
         RETURNING id, created_at`,
		idgen.GenerateID(), warning.GuildID, warning.UserID, warning.CaseNumber, warning.ModeratorID, warning.Reason, warning.CreatedAt,
	).Scan(&warning.ID, &warning.CreatedAt); err != nil {
		return ModerationWarning{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ModerationWarning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}
	return warning, nil
}

// ListModerationWarnings lists moderation warnings utilizing iter.Seq2.
func (s *Store) ListModerationWarnings(ctx context.Context, guildID, userID string, limit int) iter.Seq2[ModerationWarning, error] {
	return func(yield func(ModerationWarning, error) bool) {
		guildID = strings.TrimSpace(guildID)
		userID = strings.TrimSpace(userID)
		if guildID == "" || userID == "" {
			return
		}
		if limit <= 0 {
			limit = 5
		}
		if limit > 25 {
			limit = 25
		}

		rows, err := s.db.Query(ctx,
			`SELECT id, guild_id, user_id, case_number, moderator_id, reason, created_at
             FROM moderation_warnings
             WHERE guild_id=$1 AND user_id=$2
             ORDER BY case_number DESC
             LIMIT $3`,
			guildID, userID, limit,
		)
		if err != nil {
			yield(ModerationWarning{}, fmt.Errorf("Store.ListModerationWarnings: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			var warning ModerationWarning
			if err := rows.Scan(&warning.ID, &warning.GuildID, &warning.UserID, &warning.CaseNumber, &warning.ModeratorID, &warning.Reason, &warning.CreatedAt); err != nil {
				yield(ModerationWarning{}, err)
				return
			}
			warning.CreatedAt = warning.CreatedAt.UTC()
			if !yield(warning, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(ModerationWarning{}, fmt.Errorf("Store.ListModerationWarnings: %w", err))
		}
	}
}

// SetGuildOwnerID sets or updates the cached owner ID for a guild.
func (s *Store) SetGuildOwnerID(ctx context.Context, guildID, ownerID string) error {
	if guildID == "" || ownerID == "" {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO guild_meta (guild_id, owner_id)
         VALUES ($1, $2)
         ON CONFLICT(guild_id) DO UPDATE SET owner_id=excluded.owner_id`,
		guildID, ownerID,
	)
	return err
}

// GetGuildOwnerID retrieves the cached owner ID for a guild.
func (s *Store) GetGuildOwnerID(ctx context.Context, guildID string) (string, bool, error) {
	var owner *string
	if err := s.db.QueryRow(ctx, `SELECT owner_id FROM guild_meta WHERE guild_id=$1`, guildID).Scan(&owner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("Store.GetGuildOwnerID: %w", err)
	}
	if owner == nil || strings.TrimSpace(*owner) == "" {
		return "", false, nil
	}
	return *owner, true, nil
}
