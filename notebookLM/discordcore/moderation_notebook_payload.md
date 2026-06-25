# Domain Architecture: moderation

## Layout Topology
```text
moderation/
├── authorization.go
├── doc.go
├── fallback.go
├── models.go
├── normalization.go
└── repository.go
```

## Source Stream Aggregation

// === FILE: pkg/moderation/authorization.go ===
```go
package moderation

// Role defines the properties of a guild role necessary for evaluating hierarchy
// and permissions in a Discord-agnostic manner.
type Role struct {
	ID          string
	Position    int
	Permissions int64
}

// Member defines the properties of a guild member necessary for evaluating permissions.
type Member struct {
	UserID  string
	RoleIDs []string
}

const (
	// PermissionAdministrator is the equivalent of the Discord Administrator flag (0x00000008).
	PermissionAdministrator int64 = 0x00000008
)

// HasPermission evaluates if a member possesses a specific bitwise permission.
// It checks all roles the member has, including the implicit @everyone role (guildID).
// If the member has a role with the Administrator flag (0x00000008), this function
// will always return true, short-circuiting other evaluations.
func HasPermission(member *Member, guildID string, rolesByID map[string]Role, requiredPerm int64) bool {
	if member == nil {
		return false
	}

	var accumulatedPerms int64

	// Accumulate permissions from the @everyone role.
	if everyoneRole, ok := rolesByID[guildID]; ok {
		accumulatedPerms |= everyoneRole.Permissions
	}

	// Accumulate permissions from all assigned roles.
	for _, roleID := range member.RoleIDs {
		if role, ok := rolesByID[roleID]; ok {
			accumulatedPerms |= role.Permissions
		}
	}

	// Administrator flag overrides any other permission checks.
	if (accumulatedPerms & PermissionAdministrator) != 0 {
		return true
	}

	return (accumulatedPerms & requiredPerm) != 0
}

// HighestRolePosition calculates the highest position of any role a member has.
// This is used for hierarchy evaluations (e.g., actor cannot moderate a target
// with an equal or higher role position).
func HighestRolePosition(member *Member, guildID string, rolesByID map[string]Role) int {
	if member == nil {
		return -1
	}

	pos := -1

	// Check @everyone role position.
	if everyoneRole, ok := rolesByID[guildID]; ok {
		pos = everyoneRole.Position
	}

	// Find the maximum position across all assigned roles.
	for _, roleID := range member.RoleIDs {
		if role, ok := rolesByID[roleID]; ok && role.Position > pos {
			pos = role.Position
		}
	}

	return pos
}

// CanModerate checks if the actor can take moderation action against the target based
// strictly on role hierarchy. The actor's highest role must be strictly greater than
// the target's highest role.
func CanModerate(actor, target *Member, guildID string, rolesByID map[string]Role) bool {
	actorPos := HighestRolePosition(actor, guildID, rolesByID)
	targetPos := HighestRolePosition(target, guildID, rolesByID)

	return actorPos > targetPos
}

```

// === FILE: pkg/moderation/doc.go ===
```go
/*
Package moderation provides Discord-agnostic core logic for moderation operations.

This package encapsulates structural evaluations such as role hierarchies, ID normalization,
and fallback case number generation. It strictly avoids any dependency on Discord network
structs or network operations.
*/
package moderation

```

// === FILE: pkg/moderation/fallback.go ===
```go
package moderation

import (
	"log/slog"
	"sync"
)

var (
	fallbackCaseSeqMu sync.Mutex
	fallbackCaseSeq   = make(map[string]int64)
)

// NextFallbackCaseNumber atomically allocates a monotonically increasing
// case number for the specified guild when the primary database is unavailable.
// It leverages a global mutex to ensure safe concurrent access.
func NextFallbackCaseNumber(guildID string, logger *slog.Logger) int64 {
	if logger == nil {
		logger = slog.Default()
	}

	fallbackCaseSeqMu.Lock()
	defer fallbackCaseSeqMu.Unlock()

	fallbackCaseSeq[guildID]++
	caseID := fallbackCaseSeq[guildID]

	logger.Warn("Mitigated service degradation: Local memory fallback case sequence allocated",
		slog.String("guild_id", guildID),
		slog.Int64("case_id", caseID),
	)

	return caseID
}

```

// === FILE: pkg/moderation/models.go ===
```go
package moderation

import "time"

type Warning struct {
	ID          int64
	GuildID     string
	UserID      string
	CaseNumber  int64
	ModeratorID string
	Reason      string
	CreatedAt   time.Time
}

```

// === FILE: pkg/moderation/normalization.go ===
```go
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

```

// === FILE: pkg/moderation/repository.go ===
```go
package moderation

import (
	"context"
	"iter"
	"time"
)

type Repository interface {
	NextModerationCaseNumber(ctx context.Context, guildID string) (int64, error)
	CreateModerationWarning(ctx context.Context, guildID, userID, moderatorID, reason string, createdAt time.Time) (Warning, error)
	ListModerationWarnings(ctx context.Context, guildID, userID string, limit int) iter.Seq2[Warning, error]
	SetGuildOwnerID(ctx context.Context, guildID, ownerID string) error
	GetGuildOwnerID(ctx context.Context, guildID string) (string, bool, error)
}

```

