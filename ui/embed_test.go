package ui

import (
	"io/fs"
	"strings"
	"testing"
)

func TestDistFSIncludesPlaceholderIndex(t *testing.T) {
	distFS, err := DistFS()
	if err != nil {
		t.Fatalf("DistFS() error = %v", err)
	}

	data, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		t.Fatalf("ReadFile(index.html) error = %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "<!doctype html>") {
		t.Fatalf("embedded index.html missing doctype: %q", content)
	}
	if !strings.Contains(strings.ToLower(content), "<html") {
		t.Fatalf("embedded index.html missing html element: %q", content)
	}
}
