package moderation

import (
	"sort"
	"strings"
)

// ParseMemberIDs extracts and cleans user IDs from a raw input string.
// It splits the input by common delimiters (comma, semicolon, space, newline, tab),
// removes duplicates, and filters out blatantly invalid snowflake formats.
func ParseMemberIDs(input string) ([]string, []string) {
	// Identify delimiters to split the massive string without panicking.
	rawIDs := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})

	unique := make(map[string]struct{})
	invalidSet := make(map[string]struct{})
	var invalid []string

	for _, id := range rawIDs {
		clean := strings.TrimSpace(id)
		if clean == "" {
			continue
		}

		if !isValidSnowflake(clean) {
			if _, exists := invalidSet[clean]; !exists {
				invalidSet[clean] = struct{}{}
				invalid = append(invalid, clean)
			}
			continue
		}

		unique[clean] = struct{}{}
	}

	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}

	sort.Strings(ids)
	sort.Strings(invalid)

	return ids, invalid
}

// isValidSnowflake checks if the given string resembles a valid Discord snowflake.
// This restricts length to between 15 and 21 characters and ensures all characters are digits.
func isValidSnowflake(value string) bool {
	if len(value) < 15 || len(value) > 21 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
