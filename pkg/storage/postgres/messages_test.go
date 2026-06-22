package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/messages"
)

func TestStore_Messages_UpsertMessagesContext(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectExec("INSERT INTO messages").WithArgs(
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))

	store.UpsertMessagesContext(context.Background(), []messages.Record{{GuildID: "g", MessageID: "m"}})
}

func TestStore_Messages_GetMessage_ErrNoRows(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT guild_id, message_id").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	store.GetMessage(context.Background(), "g", "m")
}
