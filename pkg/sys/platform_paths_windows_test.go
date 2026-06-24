//go:build windows

package sys

import (
	"path/filepath"
	"testing"
)

func TestPlatformPathsWindows(t *testing.T) {
	setTestEnv(t, map[string]string{
		"APPDATA": `C:\AppData\Roaming`,
	})
	expectedCfg := filepath.Join(`C:\AppData\Roaming`, "Alice-Bot")
	if cfg := PlatformConfigDir("Alice:Bot "); cfg != expectedCfg {
		t.Fatalf("unexpected config dir: %q", cfg)
	}

	expectedCache := filepath.Join(expectedCfg, "Cache")
	if cache := PlatformCacheDir("Alice:Bot "); cache != expectedCache {
		t.Fatalf("unexpected cache dir: %q", cache)
	}

	expectedLog := filepath.Join(expectedCfg, "Logs")
	if logDir := PlatformLogDir("Alice:Bot "); logDir != expectedLog {
		t.Fatalf("unexpected log dir: %q", logDir)
	}
}

func setTestEnv(t *testing.T, env map[string]string) {
	for k, v := range env {
		t.Setenv(k, v)
	}
}
