package files

import (
	"os"
	"path/filepath"
	"testing"
)

func setTestEnv(t *testing.T, env map[string]string) {
	testEnvOverrides.Store(t.Name(), env)
	t.Cleanup(func() {
		testEnvOverrides.Delete(t.Name())
	})
}

func TestLoadEnvWithLocalBinFallbackUsesHomeFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	fakeHome := filepath.Join(tmp, "home")
	if err := os.MkdirAll(filepath.Join(fakeHome, ".local", "bin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	envPath := filepath.Join(fakeHome, ".local", "bin", ".env")
	if err := os.WriteFile(envPath, []byte("TOKEN=fromfile"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	orig := fallbackEnvPath
	fallbackEnvPath = envPath
	t.Cleanup(func() { fallbackEnvPath = orig })

	setTestEnv(t, map[string]string{})

	got, err := LoadEnvWithLocalBinFallback("TOKEN")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got != "fromfile" {
		t.Fatalf("expected value from file, got %q", got)
	}

	// When env already set, file should not override.
	setTestEnv(t, map[string]string{
		"TOKEN": "envwins",
	})
	got, err = LoadEnvWithLocalBinFallback("TOKEN")
	if err != nil || got != "envwins" {
		t.Fatalf("expected existing env to win, got %q err=%v", got, err)
	}
}

func TestEnvHelpers(t *testing.T) {
	t.Parallel()
	setTestEnv(t, map[string]string{
		"BOOL_TRUE":  "YeS",
		"BOOL_FALSE": "0",
		"STR_EMPTY":  "  ",
		"INT_OK":     "42",
		"INT_BAD":    "oops",
	})

	if !EnvBool("BOOL_TRUE") {
		t.Fatalf("expected truthy value")
	}
	if EnvBool("BOOL_FALSE") {
		t.Fatalf("expected falsy value")
	}

	if got := EnvString("STR_EMPTY", "default"); got != "default" {
		t.Fatalf("expected default, got %q", got)
	}

	if got := EnvInt64("INT_OK", 1); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	if got := EnvInt64("INT_BAD", 7); got != 7 {
		t.Fatalf("expected fallback, got %d", got)
	}
}
