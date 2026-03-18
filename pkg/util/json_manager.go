package util

import (
	"encoding/json"
	"errors"
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

	targetPath := m.filePath
	if m.projectRoot != "" {
		safePath, err := safeJoin(m.projectRoot, m.filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve safe file path: %w", err)
		}
		targetPath = safePath
	}
	dir := filepath.Dir(targetPath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fileMode := os.FileMode(0o644)
	if info, err := os.Stat(targetPath); err == nil {
		fileMode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat target file: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(fileMode); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}
	if _, err := tmpFile.Write(fileData); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := replaceFile(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to replace file atomically: %w", err)
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("failed to sync parent directory: %w", err)
	}
	cleanupTmp = false

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
