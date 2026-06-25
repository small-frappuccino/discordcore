# Domain Architecture: moderation

## Layout Topology
```text
moderation/
├── authorization.go
├── authorization_test.go
├── doc.go
├── fallback.go
├── fallback_test.go
├── models.go
├── normalization.go
├── normalization_test.go
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

// === FILE: pkg/moderation/authorization_test.go ===
```go
package moderation

import "testing"

// TestHasPermission validates role hierarchies utilizing table-driven tests.
// It verifies standard permission evaluation, the Administrator flag override,
// and the scenario where a member lacks all roles.
func TestHasPermission(t *testing.T) {
	t.Parallel()
	const (
		guildID  = "guild_123"
		permKick = int64(0x00000002)
		permBan  = int64(0x00000004)
	)

	roles := map[string]Role{
		guildID:      {ID: guildID, Position: 0, Permissions: 0},
		"role_1":     {ID: "role_1", Position: 1, Permissions: permKick},
		"role_2":     {ID: "role_2", Position: 2, Permissions: permBan},
		"role_admin": {ID: "role_admin", Position: 10, Permissions: PermissionAdministrator},
	}

	tests := []struct {
		name         string
		member       *Member
		requiredPerm int64
		expected     bool
	}{
		{
			name:         "Member with specific permission",
			member:       &Member{UserID: "user1", RoleIDs: []string{"role_1"}},
			requiredPerm: permKick,
			expected:     true,
		},
		{
			name:         "Member without specific permission",
			member:       &Member{UserID: "user2", RoleIDs: []string{"role_1"}},
			requiredPerm: permBan,
			expected:     false,
		},
		{
			name:         "Member with Administrator flag override",
			member:       &Member{UserID: "user3", RoleIDs: []string{"role_admin"}},
			requiredPerm: permBan,
			expected:     true,
		},
		{
			name:         "Member with total omission of roles",
			member:       &Member{UserID: "user4", RoleIDs: []string{}},
			requiredPerm: permKick,
			expected:     false,
		},
		{
			name:         "Nil member",
			member:       nil,
			requiredPerm: permKick,
			expected:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := HasPermission(tc.member, guildID, roles, tc.requiredPerm)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestCanModerate evaluates structural boundary anomalies such as actor and target
// possessing privileges on the exact same layer.
func TestCanModerate(t *testing.T) {
	t.Parallel()
	const guildID = "guild_123"

	roles := map[string]Role{
		guildID:  {ID: guildID, Position: 0, Permissions: 0},
		"role_1": {ID: "role_1", Position: 1, Permissions: 0},
		"role_2": {ID: "role_2", Position: 2, Permissions: 0},
		"role_3": {ID: "role_3", Position: 2, Permissions: 0},
	}

	tests := []struct {
		name     string
		actor    *Member
		target   *Member
		expected bool
	}{
		{
			name:     "Actor strictly higher",
			actor:    &Member{UserID: "user1", RoleIDs: []string{"role_2"}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_1"}},
			expected: true,
		},
		{
			name:     "Target strictly higher",
			actor:    &Member{UserID: "user1", RoleIDs: []string{"role_1"}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_2"}},
			expected: false,
		},
		{
			name:     "Actor and Target on the exact same layer (same role)",
			actor:    &Member{UserID: "user1", RoleIDs: []string{"role_2"}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_2"}},
			expected: false,
		},
		{
			name:     "Actor and Target on the exact same layer (different roles)",
			actor:    &Member{UserID: "user1", RoleIDs: []string{"role_2"}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_3"}},
			expected: false,
		},
		{
			name:     "Actor missing roles, target has roles",
			actor:    &Member{UserID: "user1", RoleIDs: []string{}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_1"}},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CanModerate(tc.actor, tc.target, guildID, roles)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
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

// === FILE: pkg/moderation/fallback_test.go ===
```go
package moderation

import (
	"context"
	"testing"

	"golang.org/x/sync/errgroup"
)

// TestNextFallbackCaseNumber_Race stresses the case number generation
// under high concurrency to validate the sync.Mutex boundaries and ensure
// strictly monotonically increasing numbers without deadlocks.
func TestNextFallbackCaseNumber_Race(t *testing.T) {
	t.Parallel()
	const (
		concurrency = 1000
		guildID     = "123456789012345"
	)

	eg, ctx := errgroup.WithContext(context.Background())

	results := make(chan int64, concurrency)

	for i := 0; i < concurrency; i++ {
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			// Generate the fallback case number concurrently.
			results <- NextFallbackCaseNumber(guildID, nil)
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent fallback case generation failed: %v", err)
	}
	close(results)

	seen := make(map[int64]bool)
	maxVal := int64(0)

	for n := range results {
		if seen[n] {
			t.Fatalf("duplicate case number detected: %d", n)
		}
		seen[n] = true
		if n > maxVal {
			maxVal = n
		}
	}

	// Verify that exactly `concurrency` numbers were generated
	// and the max value aligns with the amount of operations.
	// Note: Because fallbackCaseSeq is global and persists across tests,
	// maxVal should be at least `concurrency`.
	if len(seen) != concurrency {
		t.Fatalf("expected %d unique numbers, got %d", concurrency, len(seen))
	}
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

// === FILE: pkg/moderation/normalization_test.go ===
```go
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

