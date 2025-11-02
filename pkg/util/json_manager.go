package util

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sync"
)

// JSONManager handles reading and writing JSON data to a file.
type JSONManager struct {
	filePath    string
	projectRoot string // Optional: for safe saving
	mu          sync.RWMutex
}

// NewJSONManager creates a new JSONManager.
func NewJSONManager(filePath string) *JSONManager {
	return &JSONManager{
		filePath: filePath,
	}
}

// WithProjectRoot sets the project root for safe saving.
func (m *JSONManager) WithProjectRoot(projectRoot string) *JSONManager {
	m.projectRoot = projectRoot
	return m
}

// Load reads the JSON file and unmarshals it into the provided data structure.
func (m *JSONManager) Load(data any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fileData, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(fileData, data); err != nil {
		return fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return nil
}

// Save marshals the provided data structure and writes it to the JSON file.
func (m *JSONManager) Save(data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fileData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}

	dir := filepath.Dir(m.filePath)
	if m.projectRoot != "" {
		safeDir, err := safeJoin(m.projectRoot, dir)
		if err != nil {
			return fmt.Errorf("failed to resolve safe directory: %w", err)
		}
		dir = safeDir
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(m.filePath, fileData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// safeJoin ensures that the joined path is within the base directory.
func safeJoin(baseDir, relPath string) (string, error) {
	cleanBase := filepath.Clean(baseDir)
	cleanPath := filepath.Join(cleanBase, relPath)
	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil || HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid path: %s", relPath)
	}
	return cleanPath, nil
}
