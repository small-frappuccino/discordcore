// Package storagetest provides test helpers for code that depends on
// *storage.Store. It is intended for import from _test.go files only.
package storagetest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// NewFailingStore returns a *storage.Store backed by a connector that always
// returns an error. Every Store method that touches the database surfaces
// the connector error, exercising the persistence-unavailable branch without
// requiring a real Postgres connection.
func NewFailingStore() *storage.Store {
	return storage.NewStore(sql.OpenDB(failingConnector{}))
}

type failingConnector struct{}

func (failingConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("storagetest: connector always fails")
}

func (failingConnector) Driver() driver.Driver { return failingDriver{} }

type failingDriver struct{}

func (failingDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("storagetest: driver always fails")
}
