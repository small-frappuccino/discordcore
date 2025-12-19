//go:build !windows && !darwin

package util

import (
	"os"
	"path/filepath"
	"strings"
)

// Unix/Linux unified filesystem layout:
//   - Config: ~/.config/<AppName>
//   - Cache:  ~/.cache/<AppName>
//   - Logs:   ~/.log/<AppName>
//
// These helpers return base directories only; callers should create directories
// (e.g., via os.MkdirAll) as needed.

func platformConfigDir(appName string) string {
	home := platformHomeDir()
	return filepath.Join(home, ".config", sanitizeAppNameForPath(appName))
}

func platformCacheDir(appName string) string {
	home := platformHomeDir()
	return filepath.Join(home, ".cache", sanitizeAppNameForPath(appName))
}

func platformLogDir(appName string) string {
	home := platformHomeDir()
	return filepath.Join(home, ".log", sanitizeAppNameForPath(appName))
}

// platformHomeDir resolves the user's home directory in a robust way across
// Unix-like environments.
func platformHomeDir() string {
	if h := strings.TrimSpace(os.Getenv("HOME")); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil && strings.TrimSpace(h) != "" {
		return h
	}
	return "."
}

// sanitizeAppNameForPath normalizes an application name so it is safe as a single
// directory segment across platforms.
func sanitizeAppNameForPath(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "alicebot"
	}

	// Replace common path separators and characters that can break paths.
	n = strings.ReplaceAll(n, "/", "-")
	n = strings.ReplaceAll(n, "\\", "-")

	// Keep it conservative: no NULs, no leading/trailing whitespace.
	n = strings.ReplaceAll(n, "\x00", "")
	n = strings.TrimSpace(n)

	if n == "" {
		return "alicebot"
	}
	return n
}
