package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open creates a SQL handle configured for PostgreSQL.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	normalized := cfg.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	db, err := sql.Open("pgx", normalized.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	db.SetMaxOpenConns(normalized.MaxOpenConns)
	db.SetMaxIdleConns(normalized.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(normalized.ConnMaxLifetimeSecs) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(normalized.ConnMaxIdleTimeSecs) * time.Second)

	pingCtx := ctx
	if pingCtx == nil {
		pingCtx = context.Background()
	}
	var cancel context.CancelFunc
	pingCtx, cancel = context.WithTimeout(pingCtx, time.Duration(normalized.PingTimeoutMS)*time.Millisecond)
	defer cancel()

	if err := Ping(pingCtx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres connection: %w", err)
	}
	return db, nil
}
