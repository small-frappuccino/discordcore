package util

// HasPrefix reports whether s begins with prefix.
//
// This helper avoids importing "strings" in hot paths and performs a
// constant-time slice comparison without allocations. It is safe for empty
// inputs: an empty prefix always matches; an empty s only matches an empty prefix.
func HasPrefix(s, prefix string) bool {
	lp := len(prefix)
	if lp == 0 {
		return true
	}
	if len(s) < lp {
		return false
	}
	return s[:lp] == prefix
}

// HasAnyPrefix reports whether s begins with any of the provided prefixes.
// The check is performed in order, short-circuiting on first match.
//
// If no prefixes are provided, it returns false.
func HasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// TrimPrefixIf returns s with the provided prefix removed when present, and
// a boolean indicating whether the prefix was removed.
//
// When prefix is empty, it returns s unchanged and true.
func TrimPrefixIf(s, prefix string) (string, bool) {
	lp := len(prefix)
	if lp == 0 {
		return s, true
	}
	if len(s) < lp {
		return s, false
	}
	if s[:lp] == prefix {
		return s[lp:], true
	}
	return s, false
}
