//go:build windows

package util

import (
	"os"
	"path/filepath"
	"strings"
)

// Windows unified filesystem layout (per repo requirements):
//   - Config base: %APPDATA%/<AppName>
//   - Cache base:  %APPDATA%/<AppName>/Cache
//   - Logs base:   %APPDATA%/<AppName>/Logs
//
// These helpers return paths only. Callers should create directories via os.MkdirAll.

func platformConfigDir(appName string) string {
	base := windowsAppDataBase()
	return filepath.Join(base, sanitizeAppNameForPath(appName))
}

func platformCacheDir(appName string) string {
	return filepath.Join(platformConfigDir(appName), "Cache")
}

func platformLogDir(appName string) string {
	return filepath.Join(platformConfigDir(appName), "Logs")
}

func windowsAppDataBase() string {
	if v := strings.TrimSpace(os.Getenv("APPDATA")); v != "" {
		return v
	}

	// Best-effort fallback if APPDATA isn't defined.
	// Typical location: C:\Users\<User>\AppData\Roaming
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, "AppData", "Roaming")
	}

	// Last resort: relative directory.
	return "."
}

// sanitizeAppNameForPath normalizes an application name so it is safe as a single
// directory segment on Windows, while staying stable across platforms.
//
// Windows disallows these characters in filenames/dirnames: <>:"/\|?*
// and also disallows trailing dots/spaces.
func sanitizeAppNameForPath(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		n = "alicebot"
	}

	// Replace path separators first.
	n = strings.ReplaceAll(n, "/", "-")
	n = strings.ReplaceAll(n, "\\", "-")

	// Replace invalid Windows filename characters.
	n = strings.NewReplacer(
		"<", "-",
		">", "-",
		":", "-",
		"\"", "-",
		"|", "-",
		"?", "-",
		"*", "-",
	).Replace(n)

	// Windows does not allow trailing spaces or dots in directory names.
	n = strings.TrimRight(n, " .")

	if strings.TrimSpace(n) == "" {
		return "alicebot"
	}
	return n
}
