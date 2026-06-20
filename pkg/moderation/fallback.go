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
