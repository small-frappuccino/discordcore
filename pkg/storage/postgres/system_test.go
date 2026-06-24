package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/system"
)

func TestStore_System_NextTicketID(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("INSERT INTO ticket_sequences").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"last_id"}).AddRow(int64(1)))

	store.NextTicketID(context.Background(), "guild1")
}

func TestStore_System_BotSince_ErrNoRows(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT bot_since FROM guild_meta").WithArgs(pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	store.BotSince(context.Background(), "guild1")
}

func TestStore_System_GetCacheEntry_ErrNoRows(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT cache_type, data, expires_at FROM persistent_cache").WithArgs(pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	store.GetCacheEntry(context.Background(), "key1")
}

func TestStore_System_GetCacheEntry_Expired(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	expiredTime := time.Now().Add(-1 * time.Hour)
	mock.ExpectQuery("SELECT cache_type, data, expires_at FROM persistent_cache").WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"cache_type", "data", "expires_at"}).AddRow("type1", "data1", expiredTime))

	store.GetCacheEntry(context.Background(), "key1")
}

func TestStore_System_BotSince(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		mock.ExpectExec(`INSERT INTO guild_meta`).
			WithArgs("g1", now, now, now).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.SetBotSince(context.Background(), "g1", now)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		mock.ExpectQuery(`SELECT bot_since FROM guild_meta`).
			WithArgs("g1").
			WillReturnRows(pgxmock.NewRows([]string{"bot_since"}).AddRow(now))

		ts, ok, err := store.BotSince(context.Background(), "g1")
		if err != nil || !ok || !ts.Equal(now) {
			t.Errorf("unexpected result: ts=%v, ok=%v, err=%v", ts, ok, err)
		}
	})

	t.Run("empty guildID", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.SetBotSince(context.Background(), "", time.Now())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestStore_System_RuntimeMeta(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	now := time.Now()

	// SetHeartbeatForBot
	mock.ExpectExec(`INSERT INTO runtime_meta`).
		WithArgs("heartbeat_inst1", now.UTC()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err := store.SetHeartbeatForBot(context.Background(), "inst1", now)
	if err != nil {
		t.Errorf("SetHeartbeatForBot: %v", err)
	}

	// SetLastEventForBot
	mock.ExpectExec(`INSERT INTO runtime_meta`).
		WithArgs("last_event_inst1", now.UTC()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err = store.SetLastEventForBot(context.Background(), "inst1", now)
	if err != nil {
		t.Errorf("SetLastEventForBot: %v", err)
	}

	// SetMetadata
	mock.ExpectExec(`INSERT INTO runtime_meta`).
		WithArgs("meta_key1", now.UTC()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err = store.SetMetadata(context.Background(), "key1", now)
	if err != nil {
		t.Errorf("SetMetadata: %v", err)
	}

	// Heartbeat (getRuntimeTimestamp "heartbeat")
	mock.ExpectQuery(`SELECT ts FROM runtime_meta`).
		WithArgs("heartbeat").
		WillReturnRows(pgxmock.NewRows([]string{"ts"}).AddRow(now))
	ts, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok || !ts.Equal(now) {
		t.Errorf("Heartbeat: ts=%v, ok=%v, err=%v", ts, ok, err)
	}

	// HeartbeatForBot
	mock.ExpectQuery(`SELECT ts FROM runtime_meta`).
		WithArgs("heartbeat_inst1").
		WillReturnRows(pgxmock.NewRows([]string{"ts"}).AddRow(now))
	ts, ok, err = store.HeartbeatForBot(context.Background(), "inst1")
	if err != nil || !ok || !ts.Equal(now) {
		t.Errorf("HeartbeatForBot: ts=%v, ok=%v, err=%v", ts, ok, err)
	}

	// LastEventForBot
	mock.ExpectQuery(`SELECT ts FROM runtime_meta`).
		WithArgs("last_event_inst1").
		WillReturnRows(pgxmock.NewRows([]string{"ts"}).AddRow(now))
	ts, ok, err = store.LastEventForBot(context.Background(), "inst1")
	if err != nil || !ok || !ts.Equal(now) {
		t.Errorf("LastEventForBot: ts=%v, ok=%v, err=%v", ts, ok, err)
	}

	// Metadata
	mock.ExpectQuery(`SELECT ts FROM runtime_meta`).
		WithArgs("meta_key1").
		WillReturnRows(pgxmock.NewRows([]string{"ts"}).AddRow(now))
	ts, ok, err = store.Metadata(context.Background(), "key1")
	if err != nil || !ok || !ts.Equal(now) {
		t.Errorf("Metadata: ts=%v, ok=%v, err=%v", ts, ok, err)
	}
}

func TestStore_System_UpsertCacheEntriesContext(t *testing.T) {
	t.Parallel()
	t.Run("empty entries", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.UpsertCacheEntriesContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("entries with and without guild_id", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		entries := []system.CacheEntryRecord{
			{Key: "k1", CacheType: "t1", GuildID: "g1", Data: "d1", ExpiresAt: now},
			{Key: "k2", CacheType: "t2", GuildID: "", Data: "d2", ExpiresAt: now},
			{Key: "", CacheType: "t3"}, // invalid, should be filtered
		}

		mock.ExpectExec(`INSERT INTO persistent_cache`).
			WithArgs("k1", "t1", "g1", "d1", now, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		mock.ExpectExec(`INSERT INTO persistent_cache`).
			WithArgs("k2", "t2", "d2", now, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.UpsertCacheEntriesContext(context.Background(), entries)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("foreign key error should be ignored/mitigated", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		entries := []system.CacheEntryRecord{
			{Key: "k1", CacheType: "t1", GuildID: "g1", Data: "d1"},
		}

		mock.ExpectExec(`INSERT INTO persistent_cache`).
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnError(errors.New("23503: foreign key constraint violation"))

		err := store.UpsertCacheEntriesContext(context.Background(), entries)
		if err != nil {
			t.Errorf("expected foreign key constraint violation to be ignored, got: %v", err)
		}
	})
}

func TestStore_System_GetCacheEntriesByType(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		rows := pgxmock.NewRows([]string{"cache_key", "data", "expires_at"}).
			AddRow("k1", "d1", now).
			AddRow("k2", "d2", now)

		mock.ExpectQuery(`SELECT cache_key, data, expires_at FROM persistent_cache`).
			WithArgs("t1", pgxmock.AnyArg()).
			WillReturnRows(rows)

		seq := store.GetCacheEntriesByType(context.Background(), "t1")
		var results []system.CacheEntry
		var err error
		for entry, e := range seq {
			if e != nil {
				err = e
				break
			}
			results = append(results, entry)
		}

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 || results[0].Key != "k1" || results[1].Key != "k2" {
			t.Errorf("unexpected results: %+v", results)
		}
	})

	t.Run("query error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT cache_key, data, expires_at FROM persistent_cache`).
			WillReturnError(errors.New("query error"))

		seq := store.GetCacheEntriesByType(context.Background(), "t1")
		var results []system.CacheEntry
		var err error
		for entry, e := range seq {
			if e != nil {
				err = e
				break
			}
			results = append(results, entry)
		}

		if err == nil {
			t.Error("expected error, got nil")
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestStore_System_CleanupExpiredCacheEntries(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectExec(`DELETE FROM persistent_cache WHERE expires_at <=`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("DELETE", 3))

	err := store.CleanupExpiredCacheEntries(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStore_System_GetCacheStatsContext(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		rows := pgxmock.NewRows([]string{"cache_type", "count"}).
			AddRow("t1", 5).
			AddRow("t2", 10)

		mock.ExpectQuery(`SELECT cache_type, COUNT\(\*\)`).
			WithArgs(pgxmock.AnyArg()).
			WillReturnRows(rows)

		stats, err := store.GetCacheStatsContext(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.Total != 15 || stats.ByType["t1"] != 5 || stats.ByType["t2"] != 10 {
			t.Errorf("unexpected stats: %+v", stats)
		}
	})

	t.Run("query error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT cache_type, COUNT\(\*\)`).
			WillReturnError(errors.New("db error"))

		_, err := store.GetCacheStatsContext(context.Background())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_System_PurgeGuildModerationData(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectExec(`DELETE FROM moderation_warnings WHERE guild_id =`).
			WithArgs("g1").
			WillReturnResult(pgxmock.NewResult("DELETE", 2))
		mock.ExpectExec(`DELETE FROM moderation_cases WHERE guild_id =`).
			WithArgs("g1").
			WillReturnResult(pgxmock.NewResult("DELETE", 4))
		mock.ExpectCommit()
		mock.ExpectRollback()

		err := store.PurgeGuildModerationData(context.Background(), "g1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("exec error and rollback", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectExec(`DELETE FROM moderation_warnings WHERE guild_id =`).
			WillReturnError(errors.New("db error"))
		mock.ExpectRollback()

		err := store.PurgeGuildModerationData(context.Background(), "g1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_System_IncrementDailyMemberEvents(t *testing.T) {
	t.Parallel()
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	// IncrementDailyMemberJoinContext
	mock.ExpectExec(`INSERT INTO daily_member_joins`).
		WithArgs("g1").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.IncrementDailyMemberJoinContext(context.Background(), "g1", "u1", time.Now())
	if err != nil {
		t.Errorf("IncrementDailyMemberJoinContext: %v", err)
	}

	// IncrementDailyMemberLeaveContext
	mock.ExpectExec(`INSERT INTO daily_member_leaves`).
		WithArgs("g1").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = store.IncrementDailyMemberLeaveContext(context.Background(), "g1", "u1", time.Now())
	if err != nil {
		t.Errorf("IncrementDailyMemberLeaveContext: %v", err)
	}
}
