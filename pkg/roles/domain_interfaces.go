package roles

import (
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// PanelPublisher is a pure consumer-side interface that pushes
// a role panel's layout to Discord.
type PanelPublisher interface {
	Sync(guildID string, key string, postings []files.RolePanelPostingConfig, panel *files.RolePanelConfig) SyncResult
}

// SyncFailure describes one posting the sync helper could not reconcile.
type SyncFailure struct {
	Posting files.RolePanelPostingConfig
	Err     error
}

// SyncResult summarizes what the sync helper did across the
// posting set for one panel.
type SyncResult struct {
	Edited  int
	Dropped []files.RolePanelPostingConfig
	Failed  []SyncFailure
}

// HasIssues reports whether the operator needs to be told about
// posting-level fallout (anything that wasn't a plain edit success).
func (r SyncResult) HasIssues() bool {
	return len(r.Dropped) > 0 || len(r.Failed) > 0
}

// FormatSyncSummary turns the sync outcome into one or two
// human-readable lines for the operator reply. Returns an empty
// string when there is nothing to report.
func FormatSyncSummary(result SyncResult, action string) string {
	if !result.HasIssues() && result.Edited == 0 {
		return ""
	}
	var lines []string
	if result.Edited > 0 {
		lines = append(lines, fmt.Sprintf("%s %d posting(s).", action, result.Edited))
	}
	if len(result.Dropped) > 0 {
		ids := make([]string, 0, len(result.Dropped))
		for _, p := range result.Dropped {
			ids = append(ids, p.MessageID)
		}
		lines = append(lines, fmt.Sprintf("Dropped %d orphaned posting(s) (message gone): %s.", len(result.Dropped), strings.Join(ids, ", ")))
	}
	if len(result.Failed) > 0 {
		details := make([]string, 0, len(result.Failed))
		for _, f := range result.Failed {
			details = append(details, fmt.Sprintf("message_id=%s (%v)", f.Posting.MessageID, f.Err))
		}
		lines = append(lines, fmt.Sprintf("Could not reconcile %d posting(s); these are kept on file for retry: %s.", len(result.Failed), strings.Join(details, "; ")))
	}
	return strings.Join(lines, "\n")
}
