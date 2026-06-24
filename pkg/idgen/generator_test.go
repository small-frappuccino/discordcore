package idgen

import (
	"testing"
)

func TestGenerator(t *testing.T) {
	t.Parallel()
	// Backup and restore globalNode
	oldGlobalNode := globalNode.Load()
	defer func() {
		globalNode.Store(oldGlobalNode)
	}()

	// 1. Test panic behaviors when globalNode is nil
	globalNode.Store(nil)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected GenerateID to panic when globalNode is nil")
			}
		}()
		GenerateID()
	}()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected GenerateString to panic when globalNode is nil")
			}
		}()
		GenerateString()
	}()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected GenerateHex to panic when globalNode is nil")
			}
		}()
		GenerateHex()
	}()

	// 2. Test successful Init and Generation
	err := Init(42)
	if err != nil {
		t.Fatalf("Init(42) failed: %v", err)
	}

	if globalNode.Load() == nil {
		t.Fatalf("globalNode should not be nil after Init")
	}

	id := GenerateID()
	if id == 0 {
		t.Errorf("expected non-zero ID from GenerateID")
	}

	strVal := GenerateString()
	if len(strVal) == 0 {
		t.Errorf("expected non-empty string from GenerateString")
	}

	hexVal := GenerateHex()
	if len(hexVal) == 0 {
		t.Errorf("expected non-empty string from GenerateHex")
	}

	// 3. Test ParseID on a newly generated string
	parsedID, err := ParseID(strVal)
	if err != nil {
		t.Fatalf("ParseID failed: %v", err)
	}
	if parsedID == 0 {
		t.Errorf("expected non-zero ID parsed from Base58 string")
	}

	// Test invalid ParseID
	_, err = ParseID("invalid-base58-chars!@#")
	if err == nil {
		t.Errorf("expected ParseID to fail with invalid base58 characters")
	}

	// 4. Test modulo 1024 behavior on Init
	err = Init(2049) // 2049 % 1024 = 1
	if err != nil {
		t.Fatalf("Init(2049) failed: %v", err)
	}

	// 5. Test Init with invalid node ID (snowflake.NewNode returns error for < 0 or > 1023)
	// Passing -1 will result in -1 % 1024 = -1 which should trigger an error in snowflake.NewNode.
	err = Init(-1)
	if err == nil {
		t.Errorf("expected Init(-1) to fail")
	}
}

// TestHostNameParsing tests (indirectly) that Init runs without error regardless of the hostname.
func TestHostNameParsing(t *testing.T) {
	t.Parallel()
	oldGlobalNode := globalNode.Load()
	defer func() {
		globalNode.Store(oldGlobalNode)
	}()

	// Init should handle hostnames that do or don't end in numbers gracefully.
	err := Init(999)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify we can generate and parse
	s := GenerateString()
	id, err := ParseID(s)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if id == 0 {
		t.Errorf("expected non-zero ID")
	}
}
