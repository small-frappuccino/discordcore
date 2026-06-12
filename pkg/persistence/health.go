package persistence

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Ping checks database readiness.
func Ping(ctx context.Context, db *pgxpool.Pool) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}
