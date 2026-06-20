package clean

import (
	"strconv"
	"strings"
	"time"
)

const (
	CleanMaxDeleteCount   = 100
	CleanSearchWindow     = 1000
	CleanBulkDeleteMaxAge = (14 * 24 * time.Hour) - time.Hour
)

type Message struct {
	ID        string
	AuthorID  string
	Content   string
	Timestamp time.Time
	Pinned    bool
}

type Filter struct {
	Count    int
	UserID   string
	Contains string
	FromID   string
	ToID     string
}

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

type CategorizedMessages struct {
	BulkIDs   []string
	SingleIDs []string
}

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

type FilterResult struct {
	Matched       []Message
	SkippedPinned int
	Scanned       int
}

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
