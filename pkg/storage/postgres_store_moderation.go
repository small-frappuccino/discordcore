package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
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
func (s *Store) CreateModerationWarning(guildID, userID, moderatorID, reason string, createdAt time.Time) (ModerationWarning, error) {

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

	tx, err := s.db.Begin()
	if err != nil {
		return ModerationWarning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	caseNumber, err := nextModerationCaseNumberTx(tx, guildID)
	if err != nil {
		return ModerationWarning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}

	warning := ModerationWarning{
		GuildID:     guildID,
		UserID:      userID,
		CaseNumber:  caseNumber,
		ModeratorID: moderatorID,
		Reason:      reason,
		CreatedAt:   createdAt,
	}
	if err := txQueryRow(
		tx,
		`INSERT INTO moderation_warnings (guild_id, user_id, case_number, moderator_id, reason, created_at)
         VALUES ($1, $2, $3, $4, $5, $6)
         RETURNING id, created_at`,
		warning.GuildID,
		warning.UserID,
		warning.CaseNumber,
		warning.ModeratorID,
		warning.Reason,
		warning.CreatedAt,
	).Scan(&warning.ID, &warning.CreatedAt); err != nil {
		return ModerationWarning{}, err
	}

	if err := tx.Commit(); err != nil {
		return ModerationWarning{}, fmt.Errorf("Store.CreateModerationWarning: %w", err)
	}
	return warning, nil
}

// ListModerationWarnings lists moderation warnings.
func (s *Store) ListModerationWarnings(guildID, userID string, limit int) ([]ModerationWarning, error) {

	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return nil, nil
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
		return nil, fmt.Errorf("Store.ListModerationWarnings: %w", err)
	}
	defer rows.Close()

	warnings := make([]ModerationWarning, 0, limit)
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
			return nil, err
		}
		warning.CreatedAt = warning.CreatedAt.UTC()
		warnings = append(warnings, warning)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Store.ListModerationWarnings: %w", err)
	}
	return warnings, nil
}

func nextModerationCaseNumberTx(tx *sql.Tx, guildID string) (int64, error) {
	var next int64
	if err := txQueryRow(
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
	var owner sql.NullString
	if err := row.Scan(&owner); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("Store.GetGuildOwnerID: %w", err)
	}
	if !owner.Valid || owner.String == "" {
		return "", false, nil
	}
	return owner.String, true, nil
}

// UpsertMemberRoles replaces the current set of roles for a member in a guild atomically.
func (s *Store) UpsertMemberRoles(guildID, userID string, roles []string, updatedAt time.Time) error {
	if guildID == "" || userID == "" {
		return nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("Store.UpsertMemberRoles: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := txExec(tx, `DELETE FROM roles_current WHERE guild_id=$1 AND user_id=$2`, guildID, userID); err != nil {
		return fmt.Errorf("Store.UpsertMemberRoles: %w", err)
	}
	for _, rid := range roles {
		if rid == "" {
			continue
		}
		if _, err := txExec(tx,
			`INSERT INTO roles_current (guild_id, user_id, role_id, updated_at) VALUES ($1, $2, $3, $4)`,
			guildID, userID, rid, updatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetMemberRoles returns the current cached roles for a member in a guild.
func (s *Store) GetMemberRoles(guildID, userID string) ([]string, error) {
	rows, err := s.query(`SELECT role_id FROM roles_current WHERE guild_id=$1 AND user_id=$2`, guildID, userID)
	if err != nil {
		return nil, fmt.Errorf("Store.GetMemberRoles: %w", err)
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var rid string
		if err := rows.Scan(&rid); err != nil {
			return nil, fmt.Errorf("Store.GetMemberRoles: %w", err)
		}
		if rid != "" {
			roles = append(roles, rid)
		}
	}
	return roles, rows.Err()
}

// DiffMemberRoles compares the cached set of roles with the provided current set and returns deltas.
func (s *Store) DiffMemberRoles(guildID, userID string, current []string) (added []string, removed []string, err error) {
	cached, err := s.GetMemberRoles(guildID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("Store.DiffMemberRoles: %w", err)
	}
	curSet := make(map[string]struct{}, len(current))
	for _, r := range current {
		if r != "" {
			curSet[r] = struct{}{}
		}
	}
	cacheSet := make(map[string]struct{}, len(cached))
	for _, r := range cached {
		if r != "" {
			cacheSet[r] = struct{}{}
		}
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
