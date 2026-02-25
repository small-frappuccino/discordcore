package metrics

import (
	"context"
	"database/sql"
	"testing"
)

func newTestMetricsDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite memory db: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func TestQuerySum_ReturnsAggregateValue(t *testing.T) {
	db := newTestMetricsDB(t)
	if _, err := db.Exec(`CREATE TABLE metric_values (value INTEGER)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO metric_values(value) VALUES (2), (3), (5)`); err != nil {
		t.Fatalf("insert rows: %v", err)
	}

	got, err := querySum(context.Background(), db, `SELECT SUM(value) FROM metric_values`)
	if err != nil {
		t.Fatalf("querySum returned unexpected error: %v", err)
	}
	if got != 10 {
		t.Fatalf("expected sum=10, got %d", got)
	}
}

func TestQuerySum_ReturnsZeroWhenAggregateIsNull(t *testing.T) {
	db := newTestMetricsDB(t)
	if _, err := db.Exec(`CREATE TABLE metric_values (value INTEGER)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	got, err := querySum(context.Background(), db, `SELECT SUM(value) FROM metric_values`)
	if err != nil {
		t.Fatalf("querySum returned unexpected error: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected sum=0 for NULL aggregate, got %d", got)
	}
}

func TestQuerySum_ReturnsErrorForInvalidSQL(t *testing.T) {
	db := newTestMetricsDB(t)

	got, err := querySum(context.Background(), db, `SELECT SUM(value) FROM missing_table`)
	if err == nil {
		t.Fatalf("expected SQL error, got nil (value=%d)", got)
	}
	if got != 0 {
		t.Fatalf("expected fallback value=0 on SQL error, got %d", got)
	}
}
