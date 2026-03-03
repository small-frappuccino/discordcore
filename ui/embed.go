package ui

import (
	"embed"
	"fmt"
	"io/fs"
)

// rawDist keeps the dashboard assets available for embedding into the final host binary.
//
//go:embed all:dist
var rawDist embed.FS

// DistFS exposes the embedded dashboard build output as a file system rooted at dist/.
func DistFS() (fs.FS, error) {
	sub, err := fs.Sub(rawDist, "dist")
	if err != nil {
		return nil, fmt.Errorf("embedded ui dist: %w", err)
	}
	return sub, nil
}
