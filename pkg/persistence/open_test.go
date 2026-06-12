package persistence_test

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/persistence"
)

func TestOpen_InvalidConfig(t *testing.T) {
	_, err := persistence.Open(context.Background(), persistence.Config{
		DatabaseURL: "",
	})
	if err == nil {
		t.Errorf("expected error on empty database URL")
	}
}

func TestOpen_InvalidDSN(t *testing.T) {
	_, err := persistence.Open(context.Background(), persistence.Config{
		DatabaseURL: "not_a_valid_dsn://",
	})
	if err == nil {
		t.Errorf("expected error on invalid DSN format")
	}
}
