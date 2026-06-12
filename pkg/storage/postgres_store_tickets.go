package storage

import (
	"context"
	"fmt"
)

// NextTicketID atomically increments and returns the next available ticket sequence ID for the given guild.
func (s *Store) NextTicketID(ctx context.Context, guildID string) (int64, error) {
	var nextID int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO ticket_sequences (guild_id, last_id)
		VALUES ($1, 1)
		ON CONFLICT (guild_id) DO UPDATE
		SET last_id = ticket_sequences.last_id + 1
		RETURNING last_id
	`, guildID).Scan(&nextID)
	if err != nil {
		return 0, fmt.Errorf("Store.NextTicketID: %w", err)
	}
	return nextID, nil
}
