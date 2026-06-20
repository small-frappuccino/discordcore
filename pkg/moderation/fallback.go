package moderation

import "sync"

var (
	fallbackCaseSeqMu sync.Mutex
	fallbackCaseSeq   = make(map[string]int64)
)

// NextFallbackCaseNumber atomically allocates a monotonically increasing
// case number for the specified guild when the primary database is unavailable.
// It leverages a global mutex to ensure safe concurrent access.
func NextFallbackCaseNumber(guildID string) int64 {
	fallbackCaseSeqMu.Lock()
	defer fallbackCaseSeqMu.Unlock()

	fallbackCaseSeq[guildID]++
	return fallbackCaseSeq[guildID]
}
