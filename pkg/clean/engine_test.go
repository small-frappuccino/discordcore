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
