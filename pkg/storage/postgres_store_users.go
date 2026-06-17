package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type UserPreferences struct {
	UserID   string `json:"user_id"`
	Theme    string `json:"theme"`
	Timezone string `json:"timezone"`
}

// GetUserPreferences retrieves the preferences for a given user. If the user
// does not have explicit preferences saved, it returns a set of default values.
func (s *Store) GetUserPreferences(ctx context.Context, userID string) (*UserPreferences, error) {
	var prefs UserPreferences
	err := s.db.QueryRow(ctx, `
		SELECT user_id, theme, timezone
		FROM user_preferences
		WHERE user_id = $1
	`, userID).Scan(&prefs.UserID, &prefs.Theme, &prefs.Timezone)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &UserPreferences{
				UserID:   userID,
				Theme:    "system",
				Timezone: "UTC",
			}, nil
		}
		return nil, fmt.Errorf("GetUserPreferences scan: %w", err)
	}

	return &prefs, nil
}

// UpdateUserPreferences upserts the given user preferences.
func (s *Store) UpdateUserPreferences(ctx context.Context, prefs *UserPreferences) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_preferences (user_id, theme, timezone, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			theme = EXCLUDED.theme,
			timezone = EXCLUDED.timezone,
			updated_at = NOW()
	`, prefs.UserID, prefs.Theme, prefs.Timezone)

	if err != nil {
		return fmt.Errorf("UpdateUserPreferences exec: %w", err)
	}

	return nil
}
