package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/messages"
)

func TestStore_Messages_UpsertMessage(t *testing.T) {
	t.Run("with expiry", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create mock: %v", err)
		}
		defer mock.Close()

		store, _ := NewStore(mock, nil)
		now := time.Now()
		expiry := now.Add(time.Hour)

		rec := messages.Record{
			GuildID:        "123",
			MessageID:      "456",
			ChannelID:      "789",
			AuthorID:       "999",
			AuthorUsername: "username",
			AuthorAvatar:   "avatar",
			Content:        "hello",
			CachedAt:       now,
			ExpiresAt:      expiry,
			HasExpiry:      true,
		}

		mock.ExpectExec(`INSERT INTO messages`).
			WithArgs("123", "456", "789", "999", "username", "avatar", "hello", now.UTC(), expiry.UTC()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = store.UpsertMessage(rec)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("without expiry", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create mock: %v", err)
		}
		defer mock.Close()

		store, _ := NewStore(mock, nil)
		now := time.Now()

		rec := messages.Record{
			GuildID:        "123",
			MessageID:      "456",
			ChannelID:      "789",
			AuthorID:       "999",
			AuthorUsername: "username",
			AuthorAvatar:   "avatar",
			Content:        "hello",
			CachedAt:       now,
			HasExpiry:      false,
		}

		mock.ExpectExec(`INSERT INTO messages`).
			WithArgs("123", "456", "789", "999", "username", "avatar", "hello", now.UTC(), nil).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = store.UpsertMessage(rec)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create mock: %v", err)
		}
		defer mock.Close()

		store, _ := NewStore(mock, nil)
		mock.ExpectExec(`INSERT INTO messages`).
			WillReturnError(errors.New("db error"))

		err = store.UpsertMessage(messages.Record{})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_UpsertMessagesContext(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.UpsertMessagesContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error on nil: %v", err)
		}
	})

	t.Run("with records and validation", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		expiry := now.Add(time.Hour)

		records := []messages.Record{
			{GuildID: "123", MessageID: "456", ChannelID: "ch1", CachedAt: now, ExpiresAt: expiry, HasExpiry: true},
			// Duplicate, should be deduplicated
			{GuildID: "123", MessageID: "456", ChannelID: "ch1_dup", CachedAt: now, ExpiresAt: expiry, HasExpiry: true},
			// Invalid keys, should be filtered
			{GuildID: "", MessageID: "invalid"},
			{GuildID: "invalid", MessageID: ""},
			// Another valid one without expiry
			{GuildID: "789", MessageID: "101", ChannelID: "ch2", CachedAt: now},
		}

		expectedGuilds := []string{"123", "789"}
		expectedMessages := []string{"456", "101"}
		expectedChannels := []string{"ch1_dup", "ch2"}
		expectedCachedAts := []time.Time{now.UTC(), now.UTC()}
		expiryTime := expiry.UTC()
		expectedExpiresAts := []*time.Time{&expiryTime, nil}

		mock.ExpectExec(`INSERT INTO messages`).
			WithArgs(expectedGuilds, expectedMessages, expectedChannels, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), expectedCachedAts, expectedExpiresAts).
			WillReturnResult(pgxmock.NewResult("INSERT", 2))

		err := store.UpsertMessagesContext(context.Background(), records)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`INSERT INTO messages`).
			WillReturnError(errors.New("db exec error"))

		err := store.UpsertMessagesContext(context.Background(), []messages.Record{{GuildID: "123", MessageID: "456"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_GetMessage(t *testing.T) {
	t.Run("found with expiry", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		expiry := now.Add(time.Hour)

		rows := pgxmock.NewRows([]string{"guild_id", "message_id", "channel_id", "author_id", "author_username", "author_avatar", "content", "cached_at", "expires_at"}).
			AddRow("123", "456", "789", "999", "user", "avatar", "hello", now, &expiry)

		mock.ExpectQuery(`SELECT guild_id, message_id`).
			WithArgs("123", "456").
			WillReturnRows(rows)

		rec, err := store.GetMessage(context.Background(), "123", "456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec == nil {
			t.Fatal("expected record, got nil")
		}
		if rec.GuildID != "123" || !rec.HasExpiry || !rec.ExpiresAt.Equal(expiry) {
			t.Errorf("unexpected record contents: %+v", rec)
		}
	})

	t.Run("found without expiry", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()

		rows := pgxmock.NewRows([]string{"guild_id", "message_id", "channel_id", "author_id", "author_username", "author_avatar", "content", "cached_at", "expires_at"}).
			AddRow("123", "456", "789", "999", "user", "avatar", "hello", now, nil)

		mock.ExpectQuery(`SELECT guild_id, message_id`).
			WithArgs("123", "456").
			WillReturnRows(rows)

		rec, err := store.GetMessage(context.Background(), "123", "456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec == nil {
			t.Fatal("expected record, got nil")
		}
		if rec.HasExpiry {
			t.Error("expected HasExpiry to be false")
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT guild_id, message_id`).
			WithArgs("123", "456").
			WillReturnError(pgx.ErrNoRows)

		rec, err := store.GetMessage(context.Background(), "123", "456")
		if err != nil {
			t.Errorf("unexpected error on ErrNoRows: %v", err)
		}
		if rec != nil {
			t.Errorf("expected nil record, got: %+v", rec)
		}
	})

	t.Run("scan error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT guild_id, message_id`).
			WithArgs("123", "456").
			WillReturnError(errors.New("scan error"))

		_, err := store.GetMessage(context.Background(), "123", "456")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_DeleteMessagesContext(t *testing.T) {
	t.Run("empty keys", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.DeleteMessagesContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error on nil keys: %v", err)
		}
	})

	t.Run("valid keys and duplicates", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		keys := []messages.DeleteKey{
			{GuildID: "123", MessageID: "456"},
			{GuildID: "123", MessageID: "456"}, // Duplicate
			{GuildID: "", MessageID: "invalid"},
			{GuildID: "789", MessageID: "101"},
		}

		mock.ExpectExec(`DELETE FROM messages`).
			WithArgs([]string{"123", "789"}, []string{"456", "101"}).
			WillReturnResult(pgxmock.NewResult("DELETE", 2))

		err := store.DeleteMessagesContext(context.Background(), keys)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`DELETE FROM messages`).
			WillReturnError(errors.New("db error"))

		err := store.DeleteMessagesContext(context.Background(), []messages.DeleteKey{{GuildID: "123", MessageID: "456"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_InsertMessageVersionsMixedBatchContext(t *testing.T) {
	t.Run("empty versions", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error on empty versions: %v", err)
		}
	})

	t.Run("begin tx error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin().WillReturnError(errors.New("begin error"))

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), []messages.Version{{GuildID: "g", MessageID: "m", EventType: "e"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("lock counter error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO message_version_counters`).
			WillReturnError(errors.New("insert counter error"))
		mock.ExpectRollback()

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), []messages.Version{{GuildID: "g", MessageID: "m", EventType: "e"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("successful reservation and insertion", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		versions := []messages.Version{
			{GuildID: "123", MessageID: "456", ChannelID: "ch1", AuthorID: "a1", EventType: "edit", Content: "hello v1"},
			{GuildID: "123", MessageID: "456", ChannelID: "ch1", AuthorID: "a1", EventType: "edit", Content: "hello v2"},
			// Invalid one
			{GuildID: "", MessageID: "invalid", EventType: "edit"},
		}

		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO message_version_counters`).
			WithArgs("123", "456", "123", "456").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		mock.ExpectQuery(`SELECT last_version FROM message_version_counters`).
			WithArgs("123", "456").
			WillReturnRows(pgxmock.NewRows([]string{"last_version"}).AddRow(int64(2)))

		mock.ExpectExec(`UPDATE message_version_counters`).
			WithArgs(4, "123", "456").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		mock.ExpectExec(`INSERT INTO messages_history`).
			WithArgs(
				[]string{"123", "123"},
				[]string{"456", "456"},
				[]string{"ch1", "ch1"},
				[]string{"a1", "a1"},
				[]int{3, 4},
				[]string{"edit", "edit"},
				[]string{"hello v1", "hello v2"},
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 2))

		mock.ExpectCommit()
		mock.ExpectRollback()

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), versions)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("failed rollback scenario", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO message_version_counters`).
			WillReturnError(errors.New("db error"))
		mock.ExpectRollback().WillReturnError(errors.New("rollback failed"))

		err := store.InsertMessageVersionsMixedBatchContext(context.Background(), []messages.Version{{GuildID: "123", MessageID: "456", EventType: "edit"}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_CleanupExpiredMessages(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`DELETE FROM messages WHERE expires_at IS NOT NULL`).
			WillReturnResult(pgxmock.NewResult("DELETE", 5))

		err := store.CleanupExpiredMessages()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`DELETE FROM messages WHERE expires_at IS NOT NULL`).
			WillReturnError(errors.New("cleanup error"))

		err := store.CleanupExpiredMessages()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_IncrementDailyMessageCountsContext(t *testing.T) {
	t.Run("empty deltas", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.IncrementDailyMessageCountsContext(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error on empty deltas: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		deltas := []messages.DailyCountDelta{
			{GuildID: "123", Day: now, Count: 5},
			{GuildID: "456", Day: now, Count: 10},
		}

		mock.ExpectExec(`INSERT INTO daily_message_metrics`).
			WithArgs("123", now, 5).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		mock.ExpectExec(`INSERT INTO daily_message_metrics`).
			WithArgs("456", now, 10).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.IncrementDailyMessageCountsContext(context.Background(), deltas)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		deltas := []messages.DailyCountDelta{
			{GuildID: "123", Day: time.Now(), Count: 5},
		}

		mock.ExpectExec(`INSERT INTO daily_message_metrics`).
			WillReturnError(errors.New("db error"))

		err := store.IncrementDailyMessageCountsContext(context.Background(), deltas)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Messages_DeleteMessage(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectExec(`DELETE FROM messages WHERE guild_id = \$1 AND message_id = \$2`).
		WithArgs("123", "456").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := store.DeleteMessage(context.Background(), "123", "456")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStore_Messages_InsertMessageVersion(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	v := messages.Version{
		GuildID:     "123",
		MessageID:   "456",
		ChannelID:   "789",
		AuthorID:    "999",
		Version:     2,
		EventType:   "edit",
		Content:     "new text",
		Attachments: 0,
		Embeds:      1,
		Stickers:    0,
		CreatedAt:   time.Now(),
	}

	mock.ExpectExec(`INSERT INTO messages_history`).
		WithArgs(v.GuildID, v.MessageID, v.ChannelID, v.AuthorID, v.Version, v.EventType, v.Content, v.Attachments, v.Embeds, v.Stickers, v.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.InsertMessageVersion(context.Background(), v)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStore_Messages_IncrementDailyMessageCount(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectExec(`INSERT INTO daily_message_metrics`).
		WithArgs("123").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.IncrementDailyMessageCount(context.Background(), "123")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
