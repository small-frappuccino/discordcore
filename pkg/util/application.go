package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	// DiscordBotName is set at runtime via SetBotName using the Discord API username.
	// It has no hardcoded default to avoid stale paths; when empty, EffectiveBotName() provides a fallback.
	DiscordBotName string

	// Paths are recalculated when SetBotName is called.
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
		log.Printf("Failed to read git HEAD: %v", err)
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

// GetDiscordBotToken returns the Discord bot token based on the current Git branch.
func GetDiscordBotToken(tokenName string) string {
	var token string
	switch CurrentGitBranch {
	case "main":
		token = os.Getenv(fmt.Sprintf("%s_PRODUCTION_TOKEN", tokenName))
	case "alice-main":
		token = os.Getenv(fmt.Sprintf("%s_DEVELOPMENT_TOKEN", tokenName))
	default:
		token = os.Getenv(fmt.Sprintf("%s_TOKEN_DEFAULT", tokenName))
	}

	if token == "" {
		log.Fatalf("Discord bot token is not set for branch: %s", CurrentGitBranch)
	}

	log.Printf("Discord bot token set for branch: %s", CurrentGitBranch)
	return token
}

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

	// One-time migration of legacy caches (best-effort).
	_ = MigrateLegacyAvatarCache()
}

// EffectiveBotName returns the current bot name or a safe fallback if unset.
func EffectiveBotName() string {
	n := strings.TrimSpace(DiscordBotName)
	if n == "" {
		return "DiscordBot"
	}
	return n
}

// GetApplicationSupportPath returns ~/Library/Application Support/[BotName]
// Preferences remain stored here.
func GetApplicationSupportPath(_ string) string {
	return filepath.Join(homeDir(), "Library", "Application Support", EffectiveBotName())
}

// GetApplicationCachesPath returns ~/Library/Cache/[BotName]
// All caches move here.
func GetApplicationCachesPath() string {
	return filepath.Join(homeDir(), "Library", "Cache", EffectiveBotName())
}

// GetCacheFilePath returns the path to the avatar cache JSON under the new caches root.
// New location: ~/Library/Cache/[BotName]/avatar/avatar_cache.json
func GetCacheFilePath() string {
	return filepath.Join(ApplicationCachesPath, "avatar", "avatar_cache.json")
}

// LegacyCacheFilePath returns the previous location used for avatar cache JSON.
// Old location: ~/Library/Application Support/[BotName]/data/application_cache.json
func LegacyCacheFilePath() string {
	return filepath.Join(ApplicationSupportPath, "data", "application_cache.json")
}

// GetMessageDBPath returns the SQLite DB path for message persistence.
// Location: ~/Library/Cache/[BotName]/messages/messages.db
func GetMessageDBPath() string {
	return filepath.Join(ApplicationCachesPath, "messages", "messages.db")
}

// GetSettingsFilePath returns the standardized path for settings.json
// Remains under Application Support.
func GetSettingsFilePath() string {
	return filepath.Join(ApplicationSupportPath, "preferences", "settings.json")
}

// EnsureCacheDirs creates base cache directories as needed.
// Safe to call multiple times.
func EnsureCacheDirs() error {
	dirs := []string{
		filepath.Dir(GetCacheFilePath()),
		filepath.Dir(GetMessageDBPath()),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create cache directory %s: %w", d, err)
		}
	}
	return nil
}

// MigrateLegacyAvatarCache migrates the old avatar cache file to the new caches location
// on the first run of the new system. It copies and then removes the legacy file (best-effort).
func MigrateLegacyAvatarCache() error {
	oldPath := LegacyCacheFilePath()
	newPath := GetCacheFilePath()

	// If old doesn't exist or new already exists, nothing to do.
	if !fileExists(oldPath) || fileExists(newPath) {
		return nil
	}

	// Ensure destination directory exists.
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return fmt.Errorf("failed to create destination cache directory: %w", err)
	}

	// Copy content from old to new.
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("failed to read legacy cache file: %w", err)
	}
	if err := os.WriteFile(newPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write new cache file: %w", err)
	}

	// Best-effort cleanup: remove old file and its parent "data" directory if empty.
	_ = os.Remove(oldPath)
	_ = removeDirIfEmpty(filepath.Dir(oldPath))

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
