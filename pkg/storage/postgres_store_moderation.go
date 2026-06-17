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

// ModerationWarning is a stored warning against a user in a guild. CaseNumber is
// the per-guild sequential case identifier surfaced to moderators.
type ModerationWarning struct {
	ID          int64
	GuildID     string
	UserID      string
	CaseNumber  int64
	ModeratorID string
	Reason      string
	CreatedAt   time.Time
}

// NextModerationCaseNumber atomically increments and returns the next moderation case number for a guild.
func (s *Store) NextModerationCaseNumber(guildID string) (int64, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return 0, fmt.Errorf("guildID is empty")
	}

	var next int64
	if err := s.queryRow(
		`INSERT INTO moderation_cases (guild_id, last_case_number)
         VALUES ($1, 1)
         ON CONFLICT(guild_id) DO UPDATE
         SET last_case_number = moderation_cases.last_case_number + 1
         RETURNING last_case_number`,
		guildID,
	).Scan(&next); err != nil {
		return 0, err
	}
	return next, nil
}

// CreateModerationWarning creates moderation warning.
func (s *Store) CreateModerationWarning(guildID, userID, moderatorID, reason string, createdAt time.Time) (warning ModerationWarning, err error) {

	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	moderatorID = strings.TrimSpace(moderatorID)
	reason = strings.TrimSpace(reason)
	if guildID == "" {
		return ModerationWarning{}, fmt.Errorf("guildID is empty")
	}
	if userID == "" {
		return ModerationWarning{}, fmt.Errorf("userID is empty")
	}
	if moderatorID == "" {
		return ModerationWarning{}, fmt.Errorf("moderatorID is empty")
	}
	if reason == "" {
		return ModerationWarning{}, fmt.Errorf("reason is empty")
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	} else {
		createdAt = createdAt.UTC()
	}

	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return ModerationWarning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	caseNumber, err := nextModerationCaseNumberTx(ctx, tx, guildID)
	if err != nil {
		return ModerationWarning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}

	warning = ModerationWarning{
		GuildID:     guildID,
		UserID:      userID,
		CaseNumber:  caseNumber,
		ModeratorID: moderatorID,
		Reason:      reason,
		CreatedAt:   createdAt,
	}
	if err := txQueryRowContext(
		ctx,
		tx,
		`INSERT INTO moderation_warnings (id, guild_id, user_id, case_number, moderator_id, reason, created_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7)
         RETURNING id, created_at`,
		idgen.GenerateID(),
		warning.GuildID,
		warning.UserID,
		warning.CaseNumber,
		warning.ModeratorID,
		warning.Reason,
		warning.CreatedAt,
	).Scan(&warning.ID, &warning.CreatedAt); err != nil {
		return ModerationWarning{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ModerationWarning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}
	return warning, nil
}

// ListModerationWarnings lists moderation warnings.
func (s *Store) ListModerationWarnings(guildID, userID string, limit int) iter.Seq2[ModerationWarning, error] {
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

		rows, err := s.query(
			`SELECT id, guild_id, user_id, case_number, moderator_id, reason, created_at
	         FROM moderation_warnings
	         WHERE guild_id=$1 AND user_id=$2
	         ORDER BY case_number DESC
	         LIMIT $3`,
			guildID,
			userID,
			limit,
		)
		if err != nil {
			yield(ModerationWarning{}, fmt.Errorf("Store.ListModerationWarnings: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			var warning ModerationWarning
			if err := rows.Scan(
				&warning.ID,
				&warning.GuildID,
				&warning.UserID,
				&warning.CaseNumber,
				&warning.ModeratorID,
				&warning.Reason,
				&warning.CreatedAt,
			); err != nil {
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

func nextModerationCaseNumberTx(ctx context.Context, tx pgx.Tx, guildID string) (int64, error) {
	var next int64
	if err := txQueryRowContext(
		ctx,
		tx,
		`INSERT INTO moderation_cases (guild_id, last_case_number)
         VALUES ($1, 1)
         ON CONFLICT(guild_id) DO UPDATE
         SET last_case_number = moderation_cases.last_case_number + 1
         RETURNING last_case_number`,
		guildID,
	).Scan(&next); err != nil {
		return 0, err
	}
	return next, nil
}

// SetGuildOwnerID sets or updates the cached owner ID for a guild.
func (s *Store) SetGuildOwnerID(guildID, ownerID string) error {
	if guildID == "" || ownerID == "" {
		return nil
	}
	_, err := s.exec(
		`INSERT INTO guild_meta (guild_id, owner_id)
         VALUES ($1, $2)
         ON CONFLICT(guild_id) DO UPDATE SET
           owner_id=excluded.owner_id`,
		guildID, ownerID,
	)
	return err
}

// GetGuildOwnerID retrieves the cached owner ID for a guild, if any.
func (s *Store) GetGuildOwnerID(guildID string) (string, bool, error) {
	row := s.queryRow(`SELECT owner_id FROM guild_meta WHERE guild_id=$1`, guildID)
	var owner *string
	if err := row.Scan(&owner); err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("Store.GetGuildOwnerID: %w", err)
	}
	if owner == nil || strings.TrimSpace(*owner) == "" {
		return "", false, nil
	}
	return *owner, true, nil
}

// UpsertMemberRoles replaces the current set of roles for a member in a guild atomically.
func (s *Store) UpsertMemberRoles(guildID, userID string, roles []string, updatedAt time.Time) (err error) {
	if guildID == "" || userID == "" {
		return nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	var validRoles []string
	for _, rid := range roles {
		if rid != "" {
			validRoles = append(validRoles, rid)
		}
	}

	if _, err = tx.Exec(ctx, `
		UPDATE roles_current
		SET deleted_at=$3, updated_at=$3
		WHERE guild_id=$1 AND user_id=$2
		  AND deleted_at IS NULL
		  AND role_id <> ALL($4::text[])
	`, guildID, userID, updatedAt, validRoles); err != nil {
		return err
	}

	if len(validRoles) > 0 {
		userIDs := make([]string, len(validRoles))
		for i := range userIDs {
			userIDs[i] = userID
		}
		updatedAts := make([]time.Time, len(validRoles))
		for i := range updatedAts {
			updatedAts[i] = updatedAt
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO roles_current (guild_id, user_id, role_id, updated_at)
			SELECT $1::text, * FROM UNNEST($2::text[], $3::text[], $4::timestamptz[])
			ON CONFLICT(guild_id, user_id, role_id) DO UPDATE SET updated_at=excluded.updated_at, deleted_at=NULL
			WHERE roles_current.deleted_at IS NOT NULL
			   OR roles_current.updated_at < excluded.updated_at - interval '5 minutes'`,
			guildID, userIDs, validRoles, updatedAts,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) GetMemberRoles(guildID, userID string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		rows, err := s.query(`SELECT role_id FROM roles_current WHERE guild_id=$1 AND user_id=$2 AND deleted_at IS NULL`, guildID, userID)
		if err != nil {
			yield("", fmt.Errorf("Store.GetMemberRoles: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			var rid string
			if err := rows.Scan(&rid); err != nil {
				yield("", fmt.Errorf("Store.GetMemberRoles: %w", err))
				return
			}
			if rid != "" {
				if !yield(rid, nil) {
					return
				}
			}
		}
		if err := rows.Err(); err != nil {
			yield("", fmt.Errorf("Store.GetMemberRoles: %w", err))
		}
	}
}

// DiffMemberRoles compares the cached set of roles with the provided current set and returns deltas.
func (s *Store) DiffMemberRoles(guildID, userID string, current []string) (added []string, removed []string, err error) {
	curSet := make(map[string]struct{}, len(current))
	for _, r := range current {
		if r != "" {
			curSet[r] = struct{}{}
		}
	}
	cacheSet := make(map[string]struct{})
	for r, err := range s.GetMemberRoles(guildID, userID) {
		if err != nil {
			return nil, nil, fmt.Errorf("Store.DiffMemberRoles: %w", err)
		}
		cacheSet[r] = struct{}{}
	}

	for r := range curSet {
		if _, ok := cacheSet[r]; !ok {
			added = append(added, r)
		}
	}
	for r := range cacheSet {
		if _, ok := curSet[r]; !ok {
			removed = append(removed, r)
		}
	}
	return added, removed, nil
}
