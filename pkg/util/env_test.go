package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvWithLocalBinFallbackUsesHomeFile(t *testing.T) {
	tmp := t.TempDir()
	fakeHome := filepath.Join(tmp, "home")
	if err := os.MkdirAll(filepath.Join(fakeHome, ".local", "bin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	envPath := filepath.Join(fakeHome, ".local", "bin", ".env")
	if err := os.WriteFile(envPath, []byte("TOKEN=fromfile"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	t.Setenv("HOME", fakeHome)
	_ = os.Unsetenv("TOKEN")

	got, err := LoadEnvWithLocalBinFallback("TOKEN")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got != "fromfile" {
		t.Fatalf("expected value from file, got %q", got)
	}

	// When env already set, file should not override.
	t.Setenv("TOKEN", "envwins")
	got, err = LoadEnvWithLocalBinFallback("TOKEN")
	if err != nil || got != "envwins" {
		t.Fatalf("expected existing env to win, got %q err=%v", got, err)
	}
}

func TestEnvHelpers(t *testing.T) {
	t.Setenv("BOOL_TRUE", "YeS")
	t.Setenv("BOOL_FALSE", "0")
	if !EnvBool("BOOL_TRUE") {
		t.Fatalf("expected truthy value")
	}
	if EnvBool("BOOL_FALSE") {
		t.Fatalf("expected falsy value")
	}

	t.Setenv("STR_EMPTY", "  ")
	if got := EnvString("STR_EMPTY", "default"); got != "default" {
		t.Fatalf("expected default, got %q", got)
	}

	t.Setenv("INT_OK", "42")
	t.Setenv("INT_BAD", "oops")
	if got := EnvInt64("INT_OK", 1); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	if got := EnvInt64("INT_BAD", 7); got != 7 {
		t.Fatalf("expected fallback, got %d", got)
	}
}
