package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestStore_System_NextTicketID(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("INSERT INTO ticket_sequences").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"last_id"}).AddRow(int64(1)))

	store.NextTicketID(context.Background(), "guild1")
}

func TestStore_System_BotSince_ErrNoRows(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT bot_since FROM guild_meta").WithArgs(pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	store.BotSince(context.Background(), "guild1")
}

func TestStore_System_GetCacheEntry_ErrNoRows(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT cache_type, data, expires_at FROM persistent_cache").WithArgs(pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	store.GetCacheEntry(context.Background(), "key1")
}

func TestStore_System_GetCacheEntry_Expired(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	expiredTime := time.Now().Add(-1 * time.Hour)
	mock.ExpectQuery("SELECT cache_type, data, expires_at FROM persistent_cache").WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"cache_type", "data", "expires_at"}).AddRow("type1", "data1", expiredTime))

	store.GetCacheEntry(context.Background(), "key1")
}
