package util

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadEnvWithLocalBinFallback ensures the specified environment variable is present.
// It always attempts to load a single fallback file located at $HOME/.local/bin/.env
// to populate any variables that are currently missing from the environment (without
// overwriting already-set variables). Then it reads and returns the requested variable.
//
// Behavior:
//   - Does NOT load .env from the current working directory.
//   - Always tries to load "$HOME/.local/bin/.env" if it exists, using non-overwriting semantics.
//   - After attempting the fallback load, returns the value of tokenEnvName if present.
//
// Returns the value of the environment variable when found, or a non-nil error if the
// variable remains unset after the fallback attempt. Errors are descriptive to help callers
// decide how to log or handle the situation.
//
// Callers should pass the exact environment variable name they expect (for example
// "ALICE_BOT_DEVELOPMENT_TOKEN" or a repo-specific token name).
func LoadEnvWithLocalBinFallback(tokenEnvName string) (string, error) {
	// Always attempt to load the fallback file to populate any missing vars (non-overwriting).
	// Determine fallback path: $HOME/.local/bin/.env
	home, homeErr := os.UserHomeDir()
	var envPath string
	if homeErr == nil && home != "" {
		envPath = filepath.Join(home, ".local", "bin", ".env")
		if info, statErr := os.Stat(envPath); statErr == nil && !info.IsDir() {
			// godotenv.Load will NOT override variables that are already set.
			_ = godotenv.Load(envPath)
		}
	}

	// After attempting fallback load, return the requested variable if present.
	if v := os.Getenv(tokenEnvName); v != "" {
		return v, nil
	}

	// Build a descriptive error message.
	if envPath == "" {
		return "", fmt.Errorf("environment variable %q not set and home directory unresolved", tokenEnvName)
	}
	return "", fmt.Errorf("environment variable %q not set; attempted to load fallback file %s", tokenEnvName, envPath)
}
