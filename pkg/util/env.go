package util

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadEnvWithLocalBinFallback attempts to ensure the specified environment variable is present
// by loading a single fallback file located at $HOME/.local/bin/.env.
//
// Behavior:
//   - It does NOT load .env from the current working directory.
//   - It attempts to load the single file "$HOME/.local/bin/.env" (if it exists).
//   - After loading that file (if present), it checks whether the environment variable
//     named by tokenEnvName is set and returns its value.
//
// Returns the value of the environment variable when found, or a non-nil error if the
// variable remains unset after the fallback attempt. Errors are descriptive to help callers
// decide how to log or handle the situation.
//
// Callers should pass the exact environment variable name they expect (for example
// "ALICE_BOT_DEVELOPMENT_TOKEN" or a repo-specific token name).
func LoadEnvWithLocalBinFallback(tokenEnvName string) (string, error) {
	// First, honor already-set environment variable
	if v := os.Getenv(tokenEnvName); v != "" {
		return v, nil
	}

	// Determine fallback path: $HOME/.local/bin/.env
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("cannot determine home directory: %v", err)
	}
	envPath := filepath.Join(home, ".local", "bin", ".env")

	// If the fallback file exists, try loading it.
	if info, statErr := os.Stat(envPath); statErr == nil && !info.IsDir() {
		if loadErr := godotenv.Load(envPath); loadErr != nil {
			return "", fmt.Errorf("failed to load fallback env file %s: %v", envPath, loadErr)
		}
		// Check variable after loading fallback
		if v := os.Getenv(tokenEnvName); v != "" {
			return v, nil
		}
		// Loaded fallback but variable still missing
		return "", fmt.Errorf("environment variable %q not set after loading fallback file %s", tokenEnvName, envPath)
	}

	// Fallback file does not exist and env var not set
	return "", fmt.Errorf("environment variable %q not set and fallback env file not found: %s", tokenEnvName, envPath)
}
