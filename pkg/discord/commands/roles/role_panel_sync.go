package roles

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// Discord REST error codes the posting sync flow treats as terminal
// (the posting is gone and the config row should be dropped).
//
// The full list is at
// https://discord.com/developers/docs/topics/opcodes-and-status-codes#json-json-error-codes;
// only the two values that map to "the message we tracked is no
// longer there" are special-cased.
const (
	discordErrUnknownChannel = 10003
	discordErrUnknownMessage = 10008
)

// rolePanelSyncFailure describes one posting the sync helper could
// not reconcile with Discord. Callers format this for the operator
// reply and decide whether to keep or drop the posting.
type rolePanelSyncFailure struct {
	Posting files.RolePanelPostingConfig
	Err     error
}

// rolePanelSyncResult summarizes what the sync helper did across the
// posting set for one panel.
type rolePanelSyncResult struct {
	Edited  int
	Dropped []files.RolePanelPostingConfig
	Failed  []rolePanelSyncFailure
}

// HasIssues reports whether the operator needs to be told about
// posting-level fallout (anything that wasn't a plain edit success).
func (r rolePanelSyncResult) HasIssues() bool {
	return len(r.Dropped) > 0 || len(r.Failed) > 0
}

// rolePanelPostingSyncer edits the recorded postings of a panel to
// match a target state. The state is expressed as the embed +
// components arguments to Sync: pass the current embed/components
// pair to refresh, or the embed with nil components to strip buttons
// on delete.
//
// The session edit and per-posting drop hooks are injectable so tests
// can substitute deterministic behavior without standing up an
// httptest Discord.
type rolePanelPostingSyncer struct {
	configManager      *files.ConfigManager
	editMessage        func(s *discordgo.Session, edit *discordgo.MessageEdit) error
	editWebhookMessage func(s *discordgo.Session, webhookID, webhookToken, messageID string, edit *discordgo.WebhookEdit) error
	dropPostings       func(cm *files.ConfigManager, guildID, key string, messageIDs []string) error
}

func newRolePanelPostingSyncer(cm *files.ConfigManager) *rolePanelPostingSyncer {
	return &rolePanelPostingSyncer{
		configManager:      cm,
		editMessage:        defaultRolePanelEditMessage,
		editWebhookMessage: defaultRolePanelEditWebhookMessage,
		dropPostings:       defaultRolePanelDropPostings,
	}
}

// rolePanelSyncRequest describes one posting-sync pass: the target panel
// (GuildID/Key), its current Postings, and the Embed/Components to apply.
// Embed must be non-nil; nil Components clears the buttons.
type rolePanelSyncRequest struct {
	Session    *discordgo.Session
	GuildID    string
	Key        string
	Postings   []files.RolePanelPostingConfig
	Embed      *discordgo.MessageEmbed
	Components []discordgo.MessageComponent
}

// Sync iterates the supplied postings and edits each Discord message
// to carry the supplied embed + components. Postings whose message no
// longer exists (Discord 10003 / 10008) are dropped from the panel's
// config; other failures are recorded so the caller can surface them
// to the operator without aborting the rest of the pass.
func (s *rolePanelPostingSyncer) Sync(req rolePanelSyncRequest) rolePanelSyncResult {
	var result rolePanelSyncResult
	if len(req.Postings) == 0 {
		return result
	}

	embeds := []*discordgo.MessageEmbed{}
	if req.Embed != nil {
		embeds = []*discordgo.MessageEmbed{req.Embed}
	}
	componentsCopy := append([]discordgo.MessageComponent(nil), req.Components...)

	for _, posting := range req.Postings {
		var err error
		if posting.WebhookID != "" && posting.WebhookToken != "" {
			err = s.editWebhookMessage(req.Session, posting.WebhookID, posting.WebhookToken, posting.MessageID, &discordgo.WebhookEdit{
				Embeds:     &embeds,
				Components: &componentsCopy,
			})
		} else {
			err = s.editMessage(req.Session, &discordgo.MessageEdit{
				ID:         strings.TrimSpace(posting.MessageID),
				Channel:    strings.TrimSpace(posting.ChannelID),
				Embeds:     &embeds,
				Components: &componentsCopy,
			})
		}
		if err == nil {
			result.Edited++
			continue
		}

		if isRolePanelPostingMissingError(err) {
			result.Dropped = append(result.Dropped, posting)
			continue
		}

		result.Failed = append(result.Failed, rolePanelSyncFailure{Posting: posting, Err: err})
	}

	if len(result.Dropped) > 0 {
		ids := make([]string, 0, len(result.Dropped))
		for _, p := range result.Dropped {
			ids = append(ids, p.MessageID)
		}
		if dropErr := s.dropPostings(s.configManager, req.GuildID, req.Key, ids); dropErr != nil {
			slog.Warn("Role panel batch posting cleanup failed",
				"guildID", req.GuildID,
				"key", req.Key,
				"err", dropErr,
			)
		}
	}

	return result
}

// formatRolePanelSyncSummary turns the sync outcome into one or two
// human-readable lines for the operator reply. Returns an empty
// string when there is nothing to report.
func formatRolePanelSyncSummary(result rolePanelSyncResult, action string) string {
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

// isRolePanelPostingMissingError reports whether err is a Discord
// REST error indicating the channel or message no longer exists.
func isRolePanelPostingMissingError(err error) bool {
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

func defaultRolePanelEditMessage(s *discordgo.Session, edit *discordgo.MessageEdit) error {
	if s == nil {
		return errors.New("discord session is nil")
	}
	_, err := s.ChannelMessageEditComplex(edit)
	return err
}

func defaultRolePanelEditWebhookMessage(s *discordgo.Session, webhookID, webhookToken, messageID string, edit *discordgo.WebhookEdit) error {
	if s == nil {
		return errors.New("discord session is nil")
	}
	_, err := s.WebhookMessageEdit(webhookID, webhookToken, messageID, edit)
	return err
}

func defaultRolePanelDropPostings(cm *files.ConfigManager, guildID, key string, messageIDs []string) error {
	if cm == nil {
		return errors.New("config manager is nil")
	}
	return cm.RemoveRolePanelPostings(guildID, key, messageIDs)
}
