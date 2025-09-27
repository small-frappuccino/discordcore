package util

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alice-bnuy/discordcore/pkg/log"
	"github.com/alice-bnuy/discordcore/pkg/storage"
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
		log.Error().Errorf("Failed to read git HEAD: %v", err)
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

// EffectiveBotName returns the current bot name or a safe fallback if unset.
func EffectiveBotName() string {
	n := strings.TrimSpace(DiscordBotName)
	if n == "" {
		return "DiscordBot"
	}
	return n
}

// GetApplicationSupportPath returns the OS-specific path for application support files.
// - macOS: ~/Library/Application Support/[BotName]
// - Linux: ~/.local/lib/[BotName]
// Preferences are stored here.
func GetApplicationSupportPath(_ string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir(), "Library", "Application Support", EffectiveBotName())
	case "linux":
		return filepath.Join(homeDir(), ".local", "lib", EffectiveBotName())
	default:
		// Fallback for other platforms (e.g., Windows), using a common convention.
		return filepath.Join(homeDir(), "AppData", "Roaming", EffectiveBotName())
	}
}

// GetApplicationCachesPath returns the OS-specific path for cache files.
// - macOS: ~/Library/Cache/[BotName]
// - Linux: ~/.local/lib/[BotName]
// All caches are stored here.
func GetApplicationCachesPath() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir(), "Library", "Cache", EffectiveBotName())
	case "linux":
		return filepath.Join(homeDir(), ".local", "lib", EffectiveBotName())
	default:
		// Fallback for other platforms, using a common convention.
		return filepath.Join(homeDir(), "AppData", "Local", EffectiveBotName())
	}
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
// - macOS: ~/Library/Cache/[BotName]/messages/messages.db
// - Linux: ~/.local/lib/[BotName]/messages/messages.db
func GetMessageDBPath() string {
	return filepath.Join(ApplicationCachesPath, "messages", "messages.db")
}

// GetSettingsFilePath returns the standardized path for settings.json.
// - macOS: ~/Library/Application Support/[BotName]/preferences/settings.json
// - Linux: ~/.local/lib/[BotName]/preferences/settings.json
func GetSettingsFilePath() string {
	return filepath.Join(ApplicationSupportPath, "preferences", "settings.json")
}

// GetLogFilePath returns the path to the log file.
// - macOS: ~/Library/Application Support/[BotName]/logs/discordcore.log
// - Linux: ~/.local/lib/[BotName]/logs/discordcore.log
func GetLogFilePath() string {
	return filepath.Join(ApplicationSupportPath, "logs", "discordcore.log")
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

// MigrateLegacyAvatarCache migrates the old avatar cache file to the new caches location
// on the first run of the new system. It copies and then removes the legacy file (best-effort).
func MigrateLegacyAvatarCache() error {
	// Deprecated: JSON copying removed. Migration is handled by MigrateAvatarJSONToSQLite.
	return nil
}

// MigrateAvatarJSONToSQLite migrates avatar cache JSON (legacy or new) into SQLite and removes JSON files.
// Safe to call after SQLite directories are ready.
func MigrateAvatarJSONToSQLite() error {
	// Determine source (prefer legacy, then new)
	oldPath := LegacyMigrationCacheFilePath()
	newPath := MigrationCacheFilePath()
	var src string
	if fileExists(oldPath) {
		src = oldPath
	} else if fileExists(newPath) {
		src = newPath
	} else {
		return nil // nothing to migrate
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	type guildCache struct {
		Avatars     map[string]string    `json:"avatars"`
		BotSince    time.Time            `json:"bot_since,omitempty"`
		MemberJoins map[string]time.Time `json:"member_joins,omitempty"`
		GuildID     string               `json:"guild_id,omitempty"`
	}
	type cachePayload struct {
		Guilds map[string]*guildCache `json:"guilds"`
	}

	var payload cachePayload
	if err := json.Unmarshal(data, &payload); err != nil || len(payload.Guilds) == 0 {
		// try legacy single guild
		var single guildCache
		if err2 := json.Unmarshal(data, &single); err2 != nil || single.GuildID == "" {
			return nil // unknown format; skip
		}
		payload.Guilds = map[string]*guildCache{single.GuildID: &single}
	}

	// Open store
	if err := os.MkdirAll(filepath.Dir(GetMessageDBPath()), 0o755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}
	store := storage.NewStore(GetMessageDBPath())
	if err := store.Init(); err != nil {
		return fmt.Errorf("failed to init sqlite store: %w", err)
	}
	defer store.Close()

	// Migrate
	for gid, g := range payload.Guilds {
		if g == nil {
			continue
		}
		// BotSince
		if !g.BotSince.IsZero() {
			_ = store.SetBotSince(gid, g.BotSince)
		}
		// Members
		for uid, t := range g.MemberJoins {
			if !t.IsZero() {
				_ = store.UpsertMemberJoin(gid, uid, t)
			}
		}
		// Avatars
		for uid, h := range g.Avatars {
			if h == "" {
				h = "default"
			}
			_, _, _ = store.UpsertAvatar(gid, uid, h, time.Now())
		}
	}

	// Cleanup JSON files (best-effort)
	_ = os.Remove(oldPath)
	_ = os.Remove(newPath)
	_ = removeDirIfEmpty(filepath.Dir(oldPath))
	_ = removeDirIfEmpty(filepath.Dir(newPath))

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
