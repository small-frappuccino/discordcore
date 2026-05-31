//go:build !windows

package files

import (
	"fmt"
	"os"
)

func replaceFile(sourcePath, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}

func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("syncDir: %w", err)
	}
	defer handle.Close()
	return handle.Sync()
}
