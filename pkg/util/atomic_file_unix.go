//go:build !windows

package util

import "os"

func replaceFile(sourcePath, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}

func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}
