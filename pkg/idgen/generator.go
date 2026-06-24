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
