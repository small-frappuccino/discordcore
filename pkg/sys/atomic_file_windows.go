//go:build windows

package sys

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func ReplaceFile(sourcePath, targetPath string) error {
	sourcePtr, err := windows.UTF16PtrFromString(sourcePath)
	if err != nil {
		return fmt.Errorf("replaceFile: %w", err)
	}
	targetPtr, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return fmt.Errorf("replaceFile: %w", err)
	}
	return windows.MoveFileEx(
		sourcePtr,
		targetPtr,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}

func SyncDir(string) error {
	return nil
}
