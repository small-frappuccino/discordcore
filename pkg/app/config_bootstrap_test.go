package app

import (
	"strings"
	"testing"
)

func TestResolveDatabaseBootstrapFromEnv(t *testing.T) {
	t.Setenv(databaseDriverEnv, "postgres")
	t.Setenv(databaseURLEnv, "postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable")
	t.Setenv(databaseMaxOpenConnsEnv, "9")
	t.Setenv(databaseMaxIdleConnsEnv, "4")
	t.Setenv(databaseConnMaxLifetimeSecsEnv, "180")
	t.Setenv(databaseConnMaxIdleTimeSecsEnv, "45")
	t.Setenv(databasePingTimeoutMSEnv, "2500")

	bootstrap, err := resolveDatabaseBootstrap()
	if err != nil {
		t.Fatalf("resolve bootstrap from env: %v", err)
	}

	if bootstrap.Source != "env" {
		t.Fatalf("expected env bootstrap source, got %q", bootstrap.Source)
	}
	if got := bootstrap.Config.DatabaseURL; got != "postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable" {
		t.Fatalf("unexpected database url %q", got)
	}
	if got := bootstrap.Config.MaxOpenConns; got != 9 {
		t.Fatalf("expected max open conns 9, got %d", got)
	}
	if got := bootstrap.Config.PingTimeoutMS; got != 2500 {
		t.Fatalf("expected ping timeout 2500, got %d", got)
	}
}

func TestResolveDatabaseBootstrapRequiresEnv(t *testing.T) {
	t.Setenv(databaseDriverEnv, "")
	t.Setenv(databaseURLEnv, "")
	t.Setenv(databaseMaxOpenConnsEnv, "")
	t.Setenv(databaseMaxIdleConnsEnv, "")
	t.Setenv(databaseConnMaxLifetimeSecsEnv, "")
	t.Setenv(databaseConnMaxIdleTimeSecsEnv, "")
	t.Setenv(databasePingTimeoutMSEnv, "")

	_, err := resolveDatabaseBootstrap()
	if err == nil {
		t.Fatalf("expected missing bootstrap environment to fail")
	}
	if !strings.Contains(err.Error(), databaseURLEnv) {
		t.Fatalf("expected error to mention %s, got %v", databaseURLEnv, err)
	}
}
