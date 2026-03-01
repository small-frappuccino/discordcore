package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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
	if !strings.Contains(content, `data-dashboard-shell="embed-loader-v1"`) {
		t.Fatalf("embedded index.html missing embed shell sentinel: %q", content)
	}
	if !strings.Contains(content, `var dashboardBase = "/dashboard/";`) ||
		!strings.Contains(content, `.vite/manifest.json`) {
		t.Fatalf("embedded index.html missing dashboard manifest loader: %q", content)
	}
	if strings.Contains(content, "/dashboard/assets/") {
		t.Fatalf("embedded index.html should not hardcode built asset paths: %q", content)
	}
}

func TestTrackedEmbedIndexMatchesTemplate(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	uiDir := filepath.Dir(testFile)
	templatePath := filepath.Join(uiDir, "embed_index.template.html")
	trackedIndexPath := filepath.Join(uiDir, "dist", "index.html")

	template, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("ReadFile(template) error = %v", err)
	}

	tracked, err := os.ReadFile(trackedIndexPath)
	if err != nil {
		t.Fatalf("ReadFile(dist/index.html) error = %v", err)
	}

	if string(tracked) != string(template) {
		t.Fatalf("tracked dist/index.html drifted from embed template")
	}
}
