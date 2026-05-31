package core

import (
	"fmt"
	"strings"
)

// ProcessCommaSeparatedList parses a comma-separated string
func ProcessCommaSeparatedList(input string) []string {
	if input == "" {
		return nil
	}

	items := strings.Split(input, ",")
	result := make([]string, 0, len(items))

	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// SanitizeInput sanitizes user input
func SanitizeInput(input string) string {
	// Remove control characters and extra spaces
	input = strings.TrimSpace(input)
	// Remove multiple line breaks
	input = strings.ReplaceAll(input, "\n\n", "\n")
	return input
}

// TruncateString truncates a string if it is too long
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ValidateStringLength validates a string length
func ValidateStringLength(s string, minLen, maxLen int, fieldName string) error {
	if len(s) < minLen {
		return NewValidationError(fieldName, fmt.Sprintf("%s must be at least %d characters", fieldName, minLen))
	}
	if len(s) > maxLen {
		return NewValidationError(fieldName, fmt.Sprintf("%s must be at most %d characters", fieldName, maxLen))
	}
	return nil
}
