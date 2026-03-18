package util

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONManagerSaveWritesAtomically(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	path := filepath.Join(t.TempDir(), "settings.json")
	manager := NewJSONManager(path)

	if err := manager.Save(payload{Name: "first", Count: 1}); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := manager.Save(payload{Name: "second", Count: 2}); err != nil {
		t.Fatalf("second save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}

	var got payload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal saved file: %v", err)
	}
	if got != (payload{Name: "second", Count: 2}) {
		t.Fatalf("unexpected saved payload: %+v", got)
	}

	tmpMatches, err := filepath.Glob(path + ".*.tmp")
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(tmpMatches) != 0 {
		t.Fatalf("expected no temp files left behind, got %v", tmpMatches)
	}
}
