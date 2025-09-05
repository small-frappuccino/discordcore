package util

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

func SetBotName(name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	DiscordBotName = name
	// Recalcula o caminho base com o nome atualizado e branch atual
	ApplicationSupportPath = GetApplicationSupportPath(CurrentGitBranch)
}

func GetApplicationSupportPath(branch string) string {
	return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", DiscordBotName)
}

// getApplicationCacheFilePath returns the standardized path for application_cache.json
func GetCacheFilePath() string {
	return filepath.Join(ApplicationSupportPath, "data", "application_cache.json")
}

// GetSettingsFilePath returns the standardized path for settings.json
func GetSettingsFilePath() string {
	return filepath.Join(ApplicationSupportPath, "preferences", "settings.json")
}
