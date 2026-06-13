package embeds

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
)



type customEmbedSyncFailure struct {
	Posting files.CustomEmbedPostingConfig
	Err     error
}

type customEmbedSyncResult struct {
	Edited  int
	Dropped []files.CustomEmbedPostingConfig
	Failed  []customEmbedSyncFailure
}

// HasIssues has issues.
func (r customEmbedSyncResult) HasIssues() bool {
	return len(r.Dropped) > 0 || len(r.Failed) > 0
}

type customEmbedPostingSyncer struct {
	configManager *files.ConfigManager
	publisher     Publisher
	dropPostings  func(cm *files.ConfigManager, guildID, key string, messageIDs []string) error
}

func newCustomEmbedPostingSyncer(cm *files.ConfigManager, publisher Publisher) *customEmbedPostingSyncer {
	return &customEmbedPostingSyncer{
		configManager: cm,
		publisher:     publisher,
		dropPostings:  defaultCustomEmbedDropPostings,
	}
}

// Sync syncs.
func (s *customEmbedPostingSyncer) Sync(
	guildID string,
	key string,
	postings []files.CustomEmbedPostingConfig,
	embed files.CustomEmbedConfig,
) customEmbedSyncResult {
	var result customEmbedSyncResult
	if len(postings) == 0 {
		return result
	}

	for _, posting := range postings {
		err := s.publisher.UpdatePosting(strings.TrimSpace(posting.ChannelID), strings.TrimSpace(posting.MessageID), embed)
		if err == nil {
			result.Edited++
			continue
		}

		if errors.Is(err, ErrPostingMissing) {
			result.Dropped = append(result.Dropped, posting)
			continue
		}

		result.Failed = append(result.Failed, customEmbedSyncFailure{Posting: posting, Err: err})
	}

	if len(result.Dropped) > 0 {
		ids := make([]string, 0, len(result.Dropped))
		for _, p := range result.Dropped {
			ids = append(ids, p.MessageID)
		}
		if dropErr := s.dropPostings(s.configManager, guildID, key, ids); dropErr != nil {
			slog.Warn("Custom embed batch posting cleanup failed",
				"guildID", guildID,
				"key", key,
				"err", dropErr,
			)
		}
	}

	return result
}

func formatCustomEmbedSyncSummary(result customEmbedSyncResult, action string) string {
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



func defaultCustomEmbedDropPostings(cm *files.ConfigManager, guildID, key string, messageIDs []string) error {
	if cm == nil {
		return errors.New("config manager is nil")
	}
	return cm.RemoveCustomEmbedPostings(guildID, key, messageIDs)
}
