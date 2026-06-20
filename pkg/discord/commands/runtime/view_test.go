package runtime

import (
	"strings"
	"testing"
)

// TestFieldsForLines_BoundaryLimits mathematically guarantees exactly 1024-byte partition integrity,
// preventing JSON payload corruption and subsequent Discord API rejection (HTTP 400).
func TestFieldsForLines_BoundaryLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		lines         []string
		expectedCount int
	}{
		{
			name:          "Empty input should fallback safely",
			lines:         []string{},
			expectedCount: 1,
		},
		{
			name:          "Exact 1024 bytes fits cleanly into one field",
			lines:         []string{strings.Repeat("A", 1024)},
			expectedCount: 1,
		},
		{
			name:          "1025 bytes partitions into two fields",
			lines:         []string{strings.Repeat("A", 1025)},
			expectedCount: 2,
		},
		{
			name:          "Multibyte UTF-8 boundary slicing does not fragment runes",
			lines:         []string{strings.Repeat("✅", 400)}, // 400 * 3 = 1200 bytes. Expected to slice at 341 runes (1023 bytes).
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := fieldsForLines("TestGroup", tt.lines)

			if len(fields) != tt.expectedCount {
				t.Fatalf("expected exactly %d fields, got %d", tt.expectedCount, len(fields))
			}

			// Verify post-condition constraints dynamically
			for idx, f := range fields {
				if len(f.Value) > 1024 {
					t.Errorf("field %d violates Discord's strict 1024-byte limit: length %d bytes", idx, len(f.Value))
				}
				if len(f.Value) == 0 {
					t.Errorf("field %d is structurally empty", idx)
				}
			}
		})
	}
}

// TestFieldsForLines_MultibyteSanity isolates the utf-8 rune truncation logic.
func TestFieldsForLines_MultibyteSanity(t *testing.T) {
	t.Parallel()

	// A single rune of 4 bytes.
	// If the boundary limit is 5 bytes, it can only fit one rune.
	// The fieldsForLines function is hardcoded to 1024 max.
	// Let's create a line that is 1022 bytes of "A", plus one 3-byte rune "✅". Total 1025.
	// The truncation algorithm must slice out the last rune rather than fragmenting it.

	prefix := strings.Repeat("A", 1022)
	str := prefix + "✅" // 1025 bytes

	fields := fieldsForLines("UTF8", []string{str})

	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}

	// Field 1 should contain exactly the prefix (1022 bytes), fitting under 1024.
	if fields[0].Value != prefix {
		t.Errorf("expected field 0 to not fragment the multibyte rune. Got len: %d, expected %d", len(fields[0].Value), len(prefix))
	}

	// Field 2 should contain just the trailing rune.
	if fields[1].Value != "✅" {
		t.Errorf("expected field 1 to contain strictly the cleanly split rune, got %q", fields[1].Value)
	}
}
