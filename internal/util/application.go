package util

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alice-bnuy/logutil"
)

var (
	DiscordBotName         string = "Alice Bot"
	ApplicationSupportPath string
	CurrentGitBranch       string
)

func init() {
	// Get current git branch
	branch := getCurrentGitBranch()
	CurrentGitBranch = branch

	// Set ApplicationSupportPath
	ApplicationSupportPath = GetApplicationSupportPath(branch)

	// Ensure all application directories exist
	if err := ensureDirectories([]string{ApplicationSupportPath}); err != nil {
		log.Fatalf("Failed to initialize application directory: %v", err)
	}

	// Ensure config files exist in the new paths
	if err := EnsureConfigFiles(); err != nil {
		log.Fatalf("Failed to ensure config files: %v", err)
	}
}

func ensureDirectories(directories []string) error {
	for _, dir := range directories {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("Failed to create directory: %v", err)
				return fmt.Errorf("failed to create directory: %w", err)
			}
			log.Printf("Directory created at %s", dir)
			logutil.Infof("Directory created at %s", dir)
		}
	}
	return nil
}

// EnsureSettingsFile ensures the settings.json file exists and is properly initialized
func EnsureSettingsFile() error {
	// Create preferences subdirectory if it doesn't exist
	preferencesDir := filepath.Join(ApplicationSupportPath, "preferences")
	if err := os.MkdirAll(preferencesDir, 0755); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}

	// Check if settings file exists
	settingsFilePath := filepath.Join(preferencesDir, "settings.json")
	if _, err := os.Stat(settingsFilePath); os.IsNotExist(err) {
		logutil.Infof("Settings file not found, creating default at %s", settingsFilePath)

		// Create basic empty config
		defaultConfig := BotConfig{
			Guilds: []GuildConfig{},
		}
		configData, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to create settings file: %w", err)
		}
		if err := os.WriteFile(settingsFilePath, configData, 0644); err != nil {
			return fmt.Errorf("failed to write settings file: %w", err)
		}
	}

	return nil
}

// EnsureApplicationCacheFile ensures the application_cache.json file exists and is properly initialized
func EnsureApplicationCacheFile() error {
	// Create data subdirectory if it doesn't exist
	dataDir := filepath.Join(ApplicationSupportPath, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Check if cache file exists
	cacheFilePath := filepath.Join(dataDir, "application_cache.json")
	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
		logutil.Infof("Application cache file not found, creating default at %s", cacheFilePath)

		// Create basic empty cache
		defaultCache := `{"guilds":{},"last_updated":"","version":"2.0"}`
		if err := os.WriteFile(cacheFilePath, []byte(defaultCache), 0644); err != nil {
			return fmt.Errorf("failed to write application cache file: %w", err)
		}
	}

	return nil
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

func GetApplicationSupportPath(branch string) string {
	if branch == "main" {
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", DiscordBotName)
	}
	return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", fmt.Sprintf("%s (Development)", DiscordBotName))
}

// getApplicationCacheFilePath returns the standardized path for application_cache.json
func GetCacheFilePath() string {
	return filepath.Join(ApplicationSupportPath, "data", "application_cache.json")
}

// GetSettingsFilePath returns the standardized path for settings.json
func GetSettingsFilePath() string {
	return filepath.Join(ApplicationSupportPath, "preferences", "settings.json")
}
