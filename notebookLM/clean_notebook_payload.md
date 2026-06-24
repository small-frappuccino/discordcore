# Domain Architecture: clean

## Layout Topology
```text
clean/
├── engine.go
└── engine_test.go
```

## Source Stream Aggregation

// === FILE: pkg/clean/engine.go ===
```go
package clean

import (
	"strconv"
	"strings"
	"time"
)

// Hard limits mitigating abuse and bounding memory.
const (
	// CleanMaxDeleteCount enforces the absolute ceiling of items a single /clean execution can remove.
	CleanMaxDeleteCount = 100
	// CleanSearchWindow bounds the paginated search iteration, preventing runaway database scans.
	CleanSearchWindow = 1000
	// CleanBulkDeleteMaxAge identifies Discord's 14-day hard boundary minus an operational 1-hour buffer.
	CleanBulkDeleteMaxAge = (14 * 24 * time.Hour) - time.Hour
)

// Message represents a normalized Discord message decoupled from any specific API implementation.
type Message struct {
	ID        string
	AuthorID  string
	Content   string
	Timestamp time.Time
	Pinned    bool
}

// Filter models the bounding parameters extracted directly from the user's slash command payload.
type Filter struct {
	Count    int
	UserID   string
	Contains string
	FromID   string
	ToID     string
}

// CompareSnowflakeIDs performs a deterministic chronological ordering validation on numeric snowflake identifiers.
func CompareSnowflakeIDs(left, right string) int {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == right {
		return 0
	}
	leftValue, leftErr := strconv.ParseUint(left, 10, 64)
	rightValue, rightErr := strconv.ParseUint(right, 10, 64)
	if leftErr == nil && rightErr == nil {
		switch {
		case leftValue < rightValue:
			return -1
		case leftValue > rightValue:
			return 1
		default:
			return 0
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	if left < right {
		return -1
	}
	return 1
}

// CategorizedMessages segments message targets into Bulk and Single execution queues.
type CategorizedMessages struct {
	BulkIDs   []string
	SingleIDs []string
}

// CategorizeMessages isolates elements into Bulk or Single execution bins evaluated against the injected time.Time.
func CategorizeMessages(messages []Message, now func() time.Time) CategorizedMessages {
	currentTime := now().UTC()
	var bulk []string
	var single []string

	for _, m := range messages {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if m.Timestamp.IsZero() {
			continue
		}
		age := currentTime.Sub(m.Timestamp.UTC())
		if age >= CleanBulkDeleteMaxAge {
			single = append(single, id)
		} else {
			bulk = append(bulk, id)
		}
	}

	return CategorizedMessages{
		BulkIDs:   bulk,
		SingleIDs: single,
	}
}

// FilterResult records the progression state of the linear filtering scan.
type FilterResult struct {
	Matched       []Message
	SkippedPinned int
	Scanned       int
}

// ApplyFilter systematically screens a slice of sequential messages against bounded rules.
func ApplyFilter(messages []Message, filter Filter, alreadyMatched int) FilterResult {
	var result FilterResult

	containsNeedle := strings.ToLower(filter.Contains)

	for _, m := range messages {
		if alreadyMatched+len(result.Matched) >= filter.Count {
			break
		}

		result.Scanned++

		if filter.FromID != "" && CompareSnowflakeIDs(m.ID, filter.FromID) <= 0 {
			// reached lower bound
			break
		}
		if filter.ToID != "" && CompareSnowflakeIDs(m.ID, filter.ToID) >= 0 {
			continue
		}

		if filter.UserID != "" && m.AuthorID != filter.UserID {
			continue
		}
		if filter.Contains != "" && !strings.Contains(strings.ToLower(m.Content), containsNeedle) {
			continue
		}
		if m.Pinned {
			result.SkippedPinned++
			continue
		}

		result.Matched = append(result.Matched, m)
	}

	return result
}

```

// === FILE: pkg/clean/engine_test.go ===
```go
package clean

import (
	"strings"
	"testing"
	"time"
)

func FuzzSnowflake(f *testing.F) {
	f.Add("1234567890", "1234567891")
	f.Add("0", "0")
	f.Add("999", "1000")
	f.Add("invalid", "123")
	f.Add("", "1")
	f.Add("00001", "0000000002")

	f.Fuzz(func(t *testing.T, a, b string) {
		res1 := CompareSnowflakeIDs(a, b)
		res2 := CompareSnowflakeIDs(b, a)
		if res1 != -res2 {
			t.Errorf("CompareSnowflakeIDs not symmetric: %v vs %v", res1, res2)
		}
	})
}

func TestCategorizeMessages(t *testing.T) {
	t.Parallel()
	mockClock := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	nowFunc := func() time.Time { return mockClock }

	tests := []struct {
		name       string
		messages   []Message
		wantBulk   int
		wantSingle int
	}{
		{
			name: "exact boundary - bulk",
			messages: []Message{
				{ID: "1", Timestamp: mockClock.Add(-CleanBulkDeleteMaxAge).Add(time.Second)},
			},
			wantBulk:   1,
			wantSingle: 0,
		},
		{
			name: "exact boundary - single",
			messages: []Message{
				{ID: "2", Timestamp: mockClock.Add(-CleanBulkDeleteMaxAge)},
			},
			wantBulk:   0,
			wantSingle: 1,
		},
		{
			name: "recent",
			messages: []Message{
				{ID: "3", Timestamp: mockClock.Add(-time.Hour)},
			},
			wantBulk:   1,
			wantSingle: 0,
		},
		{
			name: "old",
			messages: []Message{
				{ID: "4", Timestamp: mockClock.Add(-30 * 24 * time.Hour)},
			},
			wantBulk:   0,
			wantSingle: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := CategorizeMessages(tt.messages, nowFunc)
			if len(res.BulkIDs) != tt.wantBulk {
				t.Errorf("got %d bulk, want %d", len(res.BulkIDs), tt.wantBulk)
			}
			if len(res.SingleIDs) != tt.wantSingle {
				t.Errorf("got %d single, want %d", len(res.SingleIDs), tt.wantSingle)
			}
		})
	}
}

func TestApplyFilter(t *testing.T) {
	t.Parallel()
	messages := make([]Message, 200)
	for i := 0; i < 200; i++ {
		messages[i] = Message{
			ID:       "id" + strings.Repeat("0", i), // pseudo ID
			AuthorID: "user1",
			Content:  "hello world",
			Pinned:   i%10 == 0, // every 10th is pinned
		}
	}

	filter := Filter{
		Count:    100,
		UserID:   "user1",
		Contains: "hello",
	}

	res := ApplyFilter(messages, filter, 0)

	if len(res.Matched) != 100 {
		t.Errorf("want 100 matched, got %d", len(res.Matched))
	}

	if res.SkippedPinned != 11 && res.SkippedPinned != 12 {
		t.Errorf("expected around 11 skipped pinned, got %d", res.SkippedPinned)
	}

	for _, m := range res.Matched {
		if m.Pinned {
			t.Errorf("expected no pinned messages in matched")
		}
	}
}

```

