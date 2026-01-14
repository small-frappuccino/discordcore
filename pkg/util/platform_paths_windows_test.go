//go:build windows

package util

import (
	"path/filepath"
	"testing"
)

func TestPlatformPathsWindows(t *testing.T) {
	t.Setenv("APPDATA", `C:\\AppData\\Roaming`)
	expectedCfg := filepath.Join(`C:\AppData\Roaming`, "Alice-Bot")
	if cfg := platformConfigDir("Alice:Bot "); cfg != expectedCfg {
		t.Fatalf("unexpected config dir: %q", cfg)
	}

	expectedCache := filepath.Join(expectedCfg, "Cache")
	if cache := platformCacheDir("Alice:Bot "); cache != expectedCache {
		t.Fatalf("unexpected cache dir: %q", cache)
	}

	expectedLog := filepath.Join(expectedCfg, "Logs")
	if logDir := platformLogDir("Alice:Bot "); logDir != expectedLog {
		t.Fatalf("unexpected log dir: %q", logDir)
	}
}
