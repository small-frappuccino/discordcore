//go:build windows

package files

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func replaceFile(sourcePath, targetPath string) error {
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

func syncDir(string) error {
	return nil
}
