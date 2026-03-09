package files

import "errors"

var (
	// ErrGuildBootstrapPrerequisite indicates bootstrap could not proceed because
	// the guild is missing a required local precondition, such as a writable
	// target channel.
	ErrGuildBootstrapPrerequisite = errors.New("guild bootstrap prerequisite failed")
	// ErrGuildBootstrapDiscordFetch indicates bootstrap could not fetch Discord
	// state required to create a guild config.
	ErrGuildBootstrapDiscordFetch = errors.New("guild bootstrap discord fetch failed")
)
