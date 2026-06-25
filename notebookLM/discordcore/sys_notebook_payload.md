# Domain Architecture: sys

## Layout Topology
```text
sys/
├── atomic_file_unix.go
├── atomic_file_windows.go
├── platform_paths_darwin.go
├── platform_paths_unix.go
└── platform_paths_windows.go
```

## Source Stream Aggregation

// === FILE: pkg/sys/atomic_file_unix.go ===
```go
//go:build !windows

package sys

import (
	"fmt"
	"os"
)

func ReplaceFile(sourcePath, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}

func SyncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("syncDir: %w", err)
	}
	defer handle.Close()
	return handle.Sync()
}

```

// === FILE: pkg/sys/atomic_file_windows.go ===
```go
//go:build windows

package sys

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func ReplaceFile(sourcePath, targetPath string) error {
	sourcePtr, err := windows.UTF16PtrFromString(sourcePath)
	if err != nil {
		return fmt.Errorf("replaceFile: %w", err)
	}
	targetPtr, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return fmt.Errorf("replaceFile: %w", err)
	}
	return windows.MoveFileEx(
		sourcePtr,
		targetPtr,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}

func SyncDir(string) error {
	return nil
}

```

// === FILE: pkg/sys/platform_paths_darwin.go ===
```go
//go:build darwin

package sys

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
func PlatformConfigDir(appName string) string {
	home := darwinHomeDir()
	return filepath.Join(home, "Library", "Preferences", sanitizeAppNameForPath(appName))
}

// platformCacheDir returns the base directory for cache files on macOS.
func PlatformCacheDir(appName string) string {
	home := darwinHomeDir()
	return filepath.Join(home, "Library", "Caches", sanitizeAppNameForPath(appName))
}

// platformLogDir returns the base directory for log files on macOS.
func PlatformLogDir(appName string) string {
	home := darwinHomeDir()
	return filepath.Join(home, "Library", "Logs", sanitizeAppNameForPath(appName))
}

func darwinHomeDir() string {
	// Prefer os.UserHomeDir() on macOS.
	if h, err := os.UserHomeDir(); err == nil && strings.TrimSpace(h) != "" {
		return h
	}

	// Best-effort fallbacks.
	if h := strings.TrimSpace(os.os.Getenv("HOME")); h != "" {
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
		return "discordmain"
	}

	// Avoid path separators.
	n = strings.ReplaceAll(n, "/", "-")
	n = strings.ReplaceAll(n, "\\", "-")

	n = strings.TrimSpace(n)
	if n == "" {
		return "discordmain"
	}
	return n
}

```

// === FILE: pkg/sys/platform_paths_unix.go ===
```go
//go:build !windows && !darwin

package sys

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

func PlatformConfigDir(appName string) string {
	home := platformHomeDir()
	return filepath.Join(home, ".config", sanitizeAppNameForPath(appName))
}

func PlatformCacheDir(appName string) string {
	home := platformHomeDir()
	return filepath.Join(home, ".cache", sanitizeAppNameForPath(appName))
}

func PlatformLogDir(appName string) string {
	home := platformHomeDir()
	return filepath.Join(home, ".log", sanitizeAppNameForPath(appName))
}

// platformHomeDir resolves the user's home directory in a robust way across
// Unix-like environments.
func platformHomeDir() string {
	if h := strings.TrimSpace(os.os.Getenv("HOME")); h != "" {
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
		return "discordmain"
	}

	// Replace common path separators and characters that can break paths.
	n = strings.ReplaceAll(n, "/", "-")
	n = strings.ReplaceAll(n, "\\", "-")

	// Keep it conservative: no NULs, no leading/trailing whitespace.
	n = strings.ReplaceAll(n, "\x00", "")
	n = strings.TrimSpace(n)

	if n == "" {
		return "discordmain"
	}
	return n
}

```

// === FILE: pkg/sys/platform_paths_windows.go ===
```go
//go:build windows

package sys

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

func PlatformConfigDir(appName string) string {
	base := windowsAppDataBase()
	return filepath.Join(base, sanitizeAppNameForPath(appName))
}

func PlatformCacheDir(appName string) string {
	return filepath.Join(PlatformConfigDir(appName), "Cache")
}

func PlatformLogDir(appName string) string {
	return filepath.Join(PlatformConfigDir(appName), "Logs")
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
		n = "discordmain"
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
		return "discordmain"
	}
	return n
}

```

