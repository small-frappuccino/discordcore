package commands

import "strings"

// ResolveFeatureForCommandPath maps a slash command path (e.g. "rolepanel", "ban")
// to its canonical product feature key (e.g. "roles", "moderation").
// It guarantees that all commands resolve to a specific domain bucket, allowing
// operators to map features to distinct bot profiles in a predictable way.
// The default fallback is "commands" which historically represents the global
// slash command surface on the primary bot.
func ResolveFeatureForCommandPath(path string) string {
	switch {
	case strings.HasPrefix(path, "qotd"):
		return "qotd"
	case strings.HasPrefix(path, "ban"),
		strings.HasPrefix(path, "kick"),
		strings.HasPrefix(path, "timeout"),
		strings.HasPrefix(path, "clean"),
		strings.HasPrefix(path, "warn"),
		strings.HasPrefix(path, "case"),
		strings.HasPrefix(path, "unban"),
		strings.HasPrefix(path, "slowmode"),
		strings.HasPrefix(path, "lock"),
		strings.HasPrefix(path, "unlock"),
		strings.HasPrefix(path, "massban"),
		strings.HasPrefix(path, "mute"),
		strings.HasPrefix(path, "reaction_block"):
		return "moderation"
	case strings.HasPrefix(path, "rolepanel"),
		strings.HasPrefix(path, "role"):
		return "roles"
	case strings.HasPrefix(path, "partner"):
		return "partners"
	case strings.HasPrefix(path, "embed"):
		return "embeds"
	case strings.HasPrefix(path, "ticket"):
		return "tickets"
	case strings.HasPrefix(path, "stats"), path == "stats":
		return "stats"
	default:
		return "commands"
	}
}
