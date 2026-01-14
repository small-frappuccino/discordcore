package files

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/small-frappuccino/discordcore/pkg/util"
)

// EnsureCustomRPCFile ensures custom-rpc.json exists and has a valid shape.
func EnsureCustomRPCFile() error {
	return EnsureCustomRPCFileAtPath(util.GetCustomRPCFilePath())
}

// EnsureCustomRPCFileAtPath ensures custom-rpc.json exists at a custom location.
func EnsureCustomRPCFileAtPath(path string) error {
	if path == "" {
		return fmt.Errorf("custom rpc path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return writeDefaultCustomRPC(path)
		}
		return fmt.Errorf("failed to read custom rpc config: %w", err)
	}

	var tmp CustomRPCConfig
	if json.Unmarshal(data, &tmp) == nil {
		return nil
	}

	return writeDefaultCustomRPC(path)
}

// LoadCustomRPCFile loads custom-rpc.json from the default path.
func LoadCustomRPCFile() (*CustomRPCConfig, error) {
	return LoadCustomRPCFileFromPath(util.GetCustomRPCFilePath())
}

// LoadCustomRPCFileFromPath loads custom-rpc.json from a custom path.
func LoadCustomRPCFileFromPath(path string) (*CustomRPCConfig, error) {
	cfg := &CustomRPCConfig{Profiles: []CustomRPCProfile{}}
	if path == "" {
		return cfg, fmt.Errorf("custom rpc path is empty")
	}

	jsonManager := util.NewJSONManager(path)
	if err := jsonManager.Load(cfg); err != nil {
		return nil, fmt.Errorf("failed to load custom rpc config from %s: %w", path, err)
	}
	return cfg, nil
}

// SaveCustomRPCFile saves custom-rpc.json to the default path.
func SaveCustomRPCFile(config *CustomRPCConfig) error {
	return SaveCustomRPCFileToPath(util.GetCustomRPCFilePath(), config)
}

// SaveCustomRPCFileToPath saves custom-rpc.json to a custom path.
func SaveCustomRPCFileToPath(path string, config *CustomRPCConfig) error {
	if config == nil {
		return fmt.Errorf("cannot save nil custom rpc config")
	}
	if path == "" {
		return fmt.Errorf("custom rpc path is empty")
	}
	jsonManager := util.NewJSONManager(path)
	if err := jsonManager.Save(config); err != nil {
		return fmt.Errorf("failed to save custom rpc config to %s: %w", path, err)
	}
	return nil
}

func writeDefaultCustomRPC(path string) error {
	defaultConfig := CustomRPCConfig{Profiles: []CustomRPCProfile{}}
	configData, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal custom rpc config: %w", err)
	}
	if err := os.WriteFile(path, configData, 0644); err != nil {
		return fmt.Errorf("failed to write custom rpc config: %w", err)
	}
	return nil
}
