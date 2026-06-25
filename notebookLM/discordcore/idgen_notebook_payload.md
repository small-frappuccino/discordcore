# Domain Architecture: idgen

## Layout Topology
```text
idgen/
├── generator.go
└── generator_test.go
```

## Source Stream Aggregation

// === FILE: pkg/idgen/generator.go ===
```go
package idgen

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/bwmarrin/snowflake"
)

var (
	globalNode atomic.Pointer[snowflake.Node]
)

// Init initializes the global snowflake generator node.
// It parses the hostname (e.g., discordcore-0) to extract the StatefulSet ordinal
// to use as the Node ID. If the hostname does not end in a number, it falls back
// to a given fallback ID.
// It must be called exactly once at startup.
func Init(fallbackNodeID int64) error {
	nodeID := fallbackNodeID
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		// Look for a trailing number (e.g. discordcore-0)
		parts := strings.Split(hostname, "-")
		if len(parts) > 1 {
			if id, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil {
				// Snowflake node ID must be 0-1023
				nodeID = id % 1024
			}
		}
	}

	// Ensure nodeID fits in 10 bits
	nodeID = nodeID % 1024

	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return fmt.Errorf("failed to initialize snowflake node: %w", err)
	}
	globalNode.Store(node)
	return nil
}

// GenerateID returns a new distributed 64-bit integer ID.
// Init must have been called prior.
func GenerateID() int64 {
	node := globalNode.Load()
	if node == nil {
		panic("idgen: GenerateID called before Init")
	}
	return node.Generate().Int64()
}

// GenerateString returns a Base58 encoded version of the Snowflake ID.
// This is ideal for short, URL-safe configuration IDs.
func GenerateString() string {
	node := globalNode.Load()
	if node == nil {
		panic("idgen: GenerateString called before Init")
	}
	// snowflake.ID.Base58() uses standard Base58 encoding
	return node.Generate().Base58()
}

// GenerateHex returns a Hex encoded version of the Snowflake ID.
func GenerateHex() string {
	node := globalNode.Load()
	if node == nil {
		panic("idgen: GenerateHex called before Init")
	}
	return node.Generate().Base36() // Base36 is standard for Snowflakes
}

// ParseID parses a Base58 string back into a standard Snowflake integer.
func ParseID(base58 string) (int64, error) {
	id, err := snowflake.ParseBase58([]byte(base58))
	if err != nil {
		return 0, err
	}
	return id.Int64(), nil
}

```

// === FILE: pkg/idgen/generator_test.go ===
```go
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

```

