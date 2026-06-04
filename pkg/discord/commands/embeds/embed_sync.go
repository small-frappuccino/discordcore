package embeds

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	discordErrUnknownChannel = 10003
	discordErrUnknownMessage = 10008
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
	editMessage   func(s *discordgo.Session, edit *discordgo.MessageEdit) error
	dropPostings  func(cm *files.ConfigManager, guildID, key string, messageIDs []string) error
}

func newCustomEmbedPostingSyncer(cm *files.ConfigManager) *customEmbedPostingSyncer {
	return &customEmbedPostingSyncer{
		configManager: cm,
		editMessage:   defaultCustomEmbedEditMessage,
		dropPostings:  defaultCustomEmbedDropPostings,
	}
}

// Sync syncs.
func (s *customEmbedPostingSyncer) Sync(
	session *discordgo.Session,
	guildID string,
	key string,
	postings []files.CustomEmbedPostingConfig,
	embed *discordgo.MessageEmbed,
) customEmbedSyncResult {
	var result customEmbedSyncResult
	if len(postings) == 0 {
		return result
	}

	embeds := []*discordgo.MessageEmbed{}
	if embed != nil {
		embeds = []*discordgo.MessageEmbed{embed}
	}

	for _, posting := range postings {
		edit := &discordgo.MessageEdit{
			ID:      strings.TrimSpace(posting.MessageID),
			Channel: strings.TrimSpace(posting.ChannelID),
			Embeds:  &embeds,
		}
		err := s.editMessage(session, edit)
		if err == nil {
			result.Edited++
			continue
		}

		if isCustomEmbedPostingMissingError(err) {
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

func isCustomEmbedPostingMissingError(err error) bool {
	var rest *discordgo.RESTError
	if !errors.As(err, &rest) || rest.Message == nil {
		return false
	}
	switch rest.Message.Code {
	case discordErrUnknownChannel, discordErrUnknownMessage:
		return true
	}
	return false
}

func defaultCustomEmbedEditMessage(s *discordgo.Session, edit *discordgo.MessageEdit) error {
	if s == nil {
		return errors.New("discord session is nil")
	}
	_, err := s.ChannelMessageEditComplex(edit)
	return err
}

func defaultCustomEmbedDropPostings(cm *files.ConfigManager, guildID, key string, messageIDs []string) error {
	if cm == nil {
		return errors.New("config manager is nil")
	}
	return cm.RemoveCustomEmbedPostings(guildID, key, messageIDs)
}
