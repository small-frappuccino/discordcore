package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/observability"
)

// queryTracer records query latencies and logs excessively slow or failing queries.
type queryTracer struct {
	summary *observability.Summary
}

// newQueryTracer creates a new configured queryTracer.
func newQueryTracer() *queryTracer {
	return &queryTracer{
		summary: &observability.Summary{},
	}
}

type queryTracerCtxKey struct{}

type queryTraceData struct {
	start time.Time
	sql   string
}

// TraceQueryStart attaches the start time to the context.
func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryTracerCtxKey{}, queryTraceData{
		start: time.Now(),
		sql:   data.SQL,
	})
}

// TraceQueryEnd records the duration and emits warnings if necessary.
func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	qData, ok := ctx.Value(queryTracerCtxKey{}).(queryTraceData)
	if !ok {
		return
	}
	duration := time.Since(qData.start)
	t.summary.Observe(duration)

	logger := log.DatabaseLogger()
	if duration > 500*time.Millisecond {
		logger.WarnContext(ctx, "slow database query",
			"duration_ms", duration.Milliseconds(),
			"sql", qData.sql,
			"err", data.Err,
		)
	} else if data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows) {
		logger.WarnContext(ctx, "database query error",
			"duration_ms", duration.Milliseconds(),
			"sql", qData.sql,
			"err", data.Err,
		)
	}
}
