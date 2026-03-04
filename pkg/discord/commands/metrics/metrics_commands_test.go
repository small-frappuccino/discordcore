package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func newMetricsTestStore(t *testing.T) *storage.Store {
	t.Helper()

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		t.Fatalf("resolve test database dsn: %v", err)
	}
	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	store := storage.NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestServerStatsAggregationsUsePostgresStore(t *testing.T) {
	store := newMetricsTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	guildID := "g1"
	userID := "u1"

	for range 3 {
		if err := store.IncrementDailyMemberJoin(guildID, userID, now); err != nil {
			t.Fatalf("increment join: %v", err)
		}
	}
	for range 2 {
		if err := store.IncrementDailyMemberLeave(guildID, userID, now); err != nil {
			t.Fatalf("increment leave: %v", err)
		}
	}

	cutoff := dayString(now.AddDate(0, 0, -1))
	joins, err := store.SumDailyMemberJoinsSince(ctx, guildID, cutoff)
	if err != nil {
		t.Fatalf("sum joins: %v", err)
	}
	if joins != 3 {
		t.Fatalf("expected joins=3, got %d", joins)
	}

	leaves, err := store.SumDailyMemberLeavesSince(ctx, guildID, cutoff)
	if err != nil {
		t.Fatalf("sum leaves: %v", err)
	}
	if leaves != 2 {
		t.Fatalf("expected leaves=2, got %d", leaves)
	}
}

func TestRenderTopWithMetricsTotals(t *testing.T) {
	got := renderTop([]storage.MetricTotal{
		{Key: "c1", Total: 10},
		{Key: "c2", Total: 3},
	}, 2, func(id string) string { return "#" + id })

	want := "1) #c1 — **10**\n2) #c2 — **3**\n"
	if got != want {
		t.Fatalf("unexpected renderTop output:\nwant:\n%s\ngot:\n%s", want, got)
	}
}
