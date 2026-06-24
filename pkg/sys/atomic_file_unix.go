//go:build !windows

package sys

import (
	"fmt"
	"os"
)

func ReplaceFile(sourcePath, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}

func SyncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("syncDir: %w", err)
	}
	defer handle.Close()
	return handle.Sync()
}
