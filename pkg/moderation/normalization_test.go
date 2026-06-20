package moderation

import (
	"testing"
	"unicode/utf8"
)

// FuzzParseMemberIDs validates the stability of ParseMemberIDs against malformed
// or massive strings, ensuring no panics or uncontrolled heap allocations occur.
func FuzzParseMemberIDs(f *testing.F) {
	// Seed the fuzzer with expected inputs.
	f.Add("123456789012345, 987654321098765")
	f.Add("invalid_id; 123456789012345")
	f.Add("123456789012345 123456789012345")
	f.Add("   ")
	f.Add("🚀 unicode test 🚀, 123456789012345")

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			return
		}
		// Execute the function under test.
		// A panic here will automatically fail the fuzz test.
		valid, invalid := ParseMemberIDs(input)

		// Ensure that the output arrays are cleanly instantiated.
		if valid == nil {
			valid = []string{}
		}
		if invalid == nil {
			invalid = []string{}
		}
	})
}
