package app

import (
	"os"
	"testing"
)

func TestRun_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DISCORDCORE_DATABASE_URL")
	err := Run("testapp")
	if err == nil {
		t.Fatal("expected Run to fail without DISCORDCORE_DATABASE_URL")
	}
}

func TestRunWithOptions_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DISCORDCORE_DATABASE_URL")
	err := RunWithOptions("testapp", RunOptions{})
	if err == nil {
		t.Fatal("expected RunWithOptions to fail without DISCORDCORE_DATABASE_URL")
	}
}

func TestSetupStorage(t *testing.T) {
	// Should fail because databaseURL is empty
	dbb := resolvedDatabaseBootstrap{}
	_, _, err := setupStorage(dbb)
	if err == nil {
		t.Fatal("expected setupStorage to fail with bad config")
	}

	// Bogus URL should fail on ping
	dbb.Config.Driver = "postgres"
	dbb.Config.DatabaseURL = "postgres://username:password@127.0.0.1:5433/bogus?sslmode=disable"
	_, _, err = setupStorage(dbb)
	if err == nil {
		t.Fatal("expected setupStorage to fail with bogus URL")
	}
}
