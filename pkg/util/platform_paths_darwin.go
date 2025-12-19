//go:build darwin

package util

import (
	"os"
	"path/filepath"
	"strings"
)

// macOS (darwin) unified filesystem layout for this repo:
//   - Config: ~/Library/Preferences/<AppName>
//   - Cache:  ~/Library/Caches/<AppName>
//   - Logs:   ~/Library/Logs/<AppName>
//
// These helpers return base directories only. Callers should os.MkdirAll as needed.
//
// NOTE: This file intentionally does NOT deal with .env locations/lookup.

// platformConfigDir returns the base directory for configuration files on macOS.
func platformConfigDir(appName string) string {
	home := darwinHomeDir()
	return filepath.Join(home, "Library", "Preferences", sanitizeAppNameForPath(appName))
}

// platformCacheDir returns the base directory for cache files on macOS.
func platformCacheDir(appName string) string {
	home := darwinHomeDir()
	return filepath.Join(home, "Library", "Caches", sanitizeAppNameForPath(appName))
}

// platformLogDir returns the base directory for log files on macOS.
func platformLogDir(appName string) string {
	home := darwinHomeDir()
	return filepath.Join(home, "Library", "Logs", sanitizeAppNameForPath(appName))
}

func darwinHomeDir() string {
	// Prefer os.UserHomeDir() on macOS.
	if h, err := os.UserHomeDir(); err == nil && strings.TrimSpace(h) != "" {
		return h
	}

	// Best-effort fallbacks.
	if h := strings.TrimSpace(os.Getenv("HOME")); h != "" {
		return h
	}
	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		return wd
	}
	return "."
}

// sanitizeAppNameForPath normalizes an application name so it is safe as a single path segment.
func sanitizeAppNameForPath(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "alicebot"
	}

	// Avoid path separators.
	n = strings.ReplaceAll(n, "/", "-")
	n = strings.ReplaceAll(n, "\\", "-")

	n = strings.TrimSpace(n)
	if n == "" {
		return "alicebot"
	}
	return n
}
