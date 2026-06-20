package moderation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
)

// TestBuildModerationEmbed_Golden implements Snapshot Testing.
// It statically compares serialized embed structures against approved .golden files,
// exposing silent regressions in formatting before payloads are submitted to the API.
func TestBuildModerationEmbed_Golden(t *testing.T) {
	// Fixed timestamp to ensure deterministic golden file output.
	fixedTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	payload := ModerationLogPayload{
		Action:      "Ban",
		TargetID:    "123456789012345",
		TargetLabel: "BadUser",
		Reason:      "Spamming channels with unicode characters.",
		RequestedBy: "987654321098765",
		Extra:       "Removed 7 days of messages.",
		CaseID:      "req_xyz789",
		ActorID:     "111222333444555",
	}

	embed := BuildModerationEmbed(payload, discord.Color(0xFF0000), fixedTime)

	data, err := json.MarshalIndent(embed, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal embed: %v", err)
	}

	goldenPath := filepath.Join("testdata", "embed_ban_standard.golden")

	// If UPDATE_GOLDEN environment variable is set, it updates the files.
	if os.Getenv("UPDATE_GOLDEN") == "true" {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatalf("failed to create testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, data, 0644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s. Run with UPDATE_GOLDEN=true to create it. err: %v", goldenPath, err)
	}

	if string(data) != string(expected) {
		t.Errorf("embed serialization mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, data)
	}
}
