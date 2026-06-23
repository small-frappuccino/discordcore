package files

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Declared as a var (not const) so tests can override it.
var fallbackEnvPath = `D:\Users\alice\.local\bin\.env`

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
	envPath := fallbackEnvPath
	if info, statErr := os.Stat(envPath); statErr == nil && !info.IsDir() {
		// godotenv.Load will NOT override variables that are already set.
		godotenv.Load(envPath)
	}

	if v := os.Getenv(tokenEnvName); v != "" {
		return v, nil
	}

	return "", fmt.Errorf("environment variable %q not set; attempted to load fallback file %s", tokenEnvName, envPath)
}

// EnvBool returns true if the named environment variable is set to a truthy value.
// Accepted truthy values (case-insensitive, trimmed):
// "1", "true", "yes", "y", "on"
func EnvBool(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// EnvString returns the trimmed value of the environment variable, or def if empty/unset.
func EnvString(name, def string) string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	return v
}

// EnvInt64 returns the parsed int64 value of the environment variable, or def if empty/unset/invalid.
func EnvInt64(name string, def int64) int64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
