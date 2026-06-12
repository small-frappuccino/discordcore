package persistence

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxPostgresMessageBodyBytes is the hard ceiling enforced on every Postgres
// protocol message received from the server. The wire protocol encodes the
// upcoming message length in a 4-byte big-endian prefix; if that prefix is
// corrupted (e.g. a load balancer cuts in with an HTML 503 page during
// connect), pgx will otherwise allocate exactly that many bytes for the
// receive buffer. We have seen this manifest in production as a 1.55 GiB
// allocation request leading to `fatal error: runtime: cannot allocate
// memory`, killing the process with no recoverable error.
//
// 64 MiB is roughly two orders of magnitude above any legitimate row
// payload this codebase sends or receives — QOTD question rows, moderation
// records, embeds — so it leaves abundant headroom while turning the
// corrupted-stream scenario into a normal `error` that bubbles up through
// database/sql and is logged like any other connection failure.
const maxPostgresMessageBodyBytes = 64 * 1024 * 1024

// Open creates a pgxpool handle configured for PostgreSQL.
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	normalized := cfg.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, fmt.Errorf("Open: %w", err)
	}

	config, err := pgxpool.ParseConfig(normalized.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres connection config: %w", err)
	}

	// Compose with any pre-existing BuildFrontend (today nil, but a future pgx
	// or middleware may install one) instead of clobbering it.
	previousBuildFrontend := config.ConnConfig.BuildFrontend
	config.ConnConfig.BuildFrontend = func(r io.Reader, w io.Writer) *pgproto3.Frontend {
		var frontend *pgproto3.Frontend
		if previousBuildFrontend != nil {
			frontend = previousBuildFrontend(r, w)
		} else {
			frontend = pgproto3.NewFrontend(r, w)
		}
		frontend.SetMaxBodyLen(maxPostgresMessageBodyBytes)
		return frontend
	}

	config.MaxConns = int32(normalized.MaxOpenConns)
	config.MinConns = int32(normalized.MaxIdleConns)
	config.MaxConnLifetime = time.Duration(normalized.ConnMaxLifetimeSecs) * time.Second
	config.MaxConnIdleTime = time.Duration(normalized.ConnMaxIdleTimeSecs) * time.Second
	config.ConnConfig.Tracer = newQueryTracer()

	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	pingCtx := ctx
	if pingCtx == nil {
		pingCtx = context.Background()
	}
	var cancel context.CancelFunc
	pingCtx, cancel = context.WithTimeout(pingCtx, time.Duration(normalized.PingTimeoutMS)*time.Millisecond)
	defer cancel()

	if err := Ping(pingCtx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres connection: %w", err)
	}
	return db, nil
}
