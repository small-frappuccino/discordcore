package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"strings"

	"github.com/small-frappuccino/discordcore/pkg/theme"
)

var (
	// ConfiguredAppName can be set by host before Discord auth; when non-empty, EffectiveBotName() uses it.
	ConfiguredAppName string

	// DiscordBotName is set at runtime via SetBotName using the Discord API username.
	// It has no hardcoded default to avoid stale paths; when empty, EffectiveBotName() provides a fallback.
	DiscordBotName string

	// Paths are recalculated when SetBotName or SetAppName is called.
	ApplicationSupportPath string
	ApplicationCachesPath  string

	CurrentGitBranch string
)

func init() {
	// Detect current git branch (best-effort; used for token selection).
	CurrentGitBranch = getCurrentGitBranch()

	// Initialize base paths with a fallback bot name; SetBotName will recompute them once the session is available.
	ApplicationSupportPath = GetApplicationSupportPath(CurrentGitBranch)
	ApplicationCachesPath = GetApplicationCachesPath()
}

func getCurrentGitBranch() string {
	data, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return "unknown"
	}
	line := strings.TrimSpace(string(data))
	if strings.HasPrefix(line, "ref: ") {
		parts := strings.Split(line, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return line
}

// GetDiscordBotToken removed.
//
// Token selection by branch and automatic environment lookups were intentionally removed
// from this package to avoid implicit behavior shared across projects. Use
// `LoadEnvWithLocalBinFallback(tokenEnvName)` from this package to load a token from
// environment with the fallback to `$HOME/.local/bin/.env` when needed.

// SetBotName sets the bot name (from Discord API) and recomputes base paths.
// It also attempts a one-time migration of legacy cache files to the new caches location.
func SetBotName(name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	DiscordBotName = sanitizeName(name)

	// Recompute base paths now that we have a proper bot name.
	ApplicationSupportPath = GetApplicationSupportPath(CurrentGitBranch)
	ApplicationCachesPath = GetApplicationCachesPath()

}

// SetAppName sets a configured application name and recomputes base paths.
// This allows host applications to control folder names before Discord auth.
func SetAppName(name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	ConfiguredAppName = sanitizeName(name)

	// Recompute base paths to use configured name.
	ApplicationSupportPath = GetApplicationSupportPath(CurrentGitBranch)
	ApplicationCachesPath = GetApplicationCachesPath()
}

// SetTheme sets the active theme by name. Empty name resets to default.
func SetTheme(name string) error {
	if strings.TrimSpace(name) == "" {
		return theme.SetCurrent("")
	}
	return theme.SetCurrent(name)
}

// ConfigureThemeFromEnv loads theme from ALICE_BOT_THEME, if set.
func ConfigureThemeFromEnv() error {
	if v := os.Getenv("ALICE_BOT_THEME"); strings.TrimSpace(v) != "" {
		return theme.SetCurrent(v)
	}
	return nil
}

// EffectiveBotName returns the current application/bot name, preferring a configured
// name when available, otherwise falling back to the Discord username, then a default.
func EffectiveBotName() string {
	// Prefer explicitly configured app name.
	if n := strings.TrimSpace(ConfiguredAppName); n != "" {
		return n
	}
	// Fallback to Discord-provided bot username.
	if n := strings.TrimSpace(DiscordBotName); n != "" {
		return n
	}
	// Final fallback.
	return "alicebot"
}

// GetApplicationSupportPath (Linux-only) returns the base path for configuration files.
// New layout: ~/.config/[BotName]
func GetApplicationSupportPath(_ string) string {
	return filepath.Join(homeDir(), ".config", EffectiveBotName())
}

// GetApplicationCachesPath (Linux-only) returns the base path for cache data.
// New layout: ~/.cache/[BotName]
func GetApplicationCachesPath() string {
	return filepath.Join(homeDir(), ".cache", EffectiveBotName())
}

// Deprecated: MigrationCacheFilePath returns the path to the avatar cache JSON used only for migration.
// Location (new): ~/Library/Cache/[BotName]/avatar/avatar_cache.json
func MigrationCacheFilePath() string {
	return filepath.Join(ApplicationCachesPath, "avatar", "avatar_cache.json")
}

// Deprecated: LegacyMigrationCacheFilePath returns the previous JSON cache path, used only for migration.
// Location (legacy): ~/Library/Application Support/[BotName]/data/application_cache.json
func LegacyMigrationCacheFilePath() string {
	return filepath.Join(ApplicationSupportPath, "data", "application_cache.json")
}

// GetMessageDBPath returns the SQLite DB path for message persistence.
// New Linux layout: ~/.cache/[BotName]/messages/messages.db
func GetMessageDBPath() string {
	return filepath.Join(ApplicationCachesPath, "messages", "messages.db")
}

// GetSettingsFilePath returns the path for the primary settings JSON.
// Layout (explicit): ~/.config/[BotName]/preferences/settings.json
func GetSettingsFilePath() string {
	return filepath.Join(ApplicationSupportPath, "preferences", "settings.json")
}

// GetLogFilePath returns the path to the main log file.
// New Linux layout: ~/.log/[BotName]/discordcore.log
func GetLogFilePath() string {
	return filepath.Join(homeDir(), ".log", EffectiveBotName(), "discordcore.log")
}

// EnsureCacheDirs creates base cache directories as needed.
// Safe to call multiple times.
func EnsureCacheDirs() error {
	dirs := []string{
		filepath.Dir(GetMessageDBPath()),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create cache directory %s: %w", d, err)
		}
	}
	return nil
}

func removeDirIfEmpty(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	entries, err := f.ReadDir(1)
	if err != nil && err != io.EOF {
		return err
	}
	// If we got at least one entry, it's not empty.
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(dir)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	// Fallback to current working directory if HOME is not set (unlikely on macOS).
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func sanitizeName(s string) string {
	// Keep it simple: trim spaces and replace slashes to avoid path issues.
	out := strings.TrimSpace(s)
	out = strings.ReplaceAll(out, "/", "-")
	out = strings.ReplaceAll(out, string(filepath.Separator), "-")
	if out == "" {
		return "DiscordBot"
	}
	return out
}

// EnsureCacheInitialized creates the minimal cache structure if it is not present.
// It is safe to call multiple times.
func EnsureCacheInitialized() error {
	dirs := []string{
		filepath.Dir(GetMessageDBPath()),               // messages db directory
		filepath.Join(ApplicationCachesPath, "avatar"), // avatar cache (even if now migrated to sqlite; kept for future artifacts)
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create cache directory %s: %w", d, err)
		}
	}

	// Best-effort marker file so we can detect initialization later (ignore errors).
	_ = os.WriteFile(filepath.Join(ApplicationCachesPath, "avatar", ".keep"), []byte{}, 0o644)

	return nil
}
