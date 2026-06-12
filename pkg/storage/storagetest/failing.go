// Package storagetest provides test helpers for code that depends on
// *storage.Store. It is intended for import from _test.go files only.
package storagetest

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// NewFailingStore returns a *storage.Store backed by a connector that always
// returns an error. Every Store method that touches the database surfaces
// the connector error, exercising the persistence-unavailable branch without
// requiring a real Postgres connection.
func NewFailingStore() *storage.Store {
	config, _ := pgxpool.ParseConfig("postgres://postgres:password@localhost:5432/postgres")
	config.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, errors.New("storagetest: connector always fails")
	}
	pool, _ := pgxpool.NewWithConfig(context.Background(), config)
	store, _ := storage.NewStore(pool)
	return store
}
