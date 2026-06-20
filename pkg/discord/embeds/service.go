package embeds

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
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

// customEmbedSyncResult aggregates the outcomes of a batch synchronization.
// It explicitly separates successfully edited postings from irrecoverable drops
// and transient failures to allow callers to safely trigger compensatory logic.
type customEmbedSyncResult struct {
	Edited  int
	Dropped []files.CustomEmbedPostingConfig
	Failed  []customEmbedSyncFailure
}

// HasIssues indicates whether the synchronization cycle encountered irrecoverable
// drops or transient failures requiring downstream mitigation.
func (r customEmbedSyncResult) HasIssues() bool {
	return len(r.Dropped) > 0 || len(r.Failed) > 0
}

// EmbedService orchestrates the rendering and synchronization of custom embeds.
// It manages the conversion of configuration states into Discord-compatible
// payloads and executes lifecycle mutations on the Discord platform.
type EmbedService struct {
	configManager *files.ConfigManager
	editMessage   func(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID, data api.EditMessageData) error
	dropPostings  func(cm *files.ConfigManager, guildID, key string, messageIDs []string) error
}

// NewEmbedService instantiates the primary domain service for custom embed management.
// It mandates the injection of the configuration manager to enforce state constraints.
func NewEmbedService(configManager *files.ConfigManager) *EmbedService {
	return &EmbedService{
		configManager: configManager,
		editMessage:   defaultCustomEmbedEditMessage,
		dropPostings:  defaultCustomEmbedDropPostings,
	}
}

// Post generates a Discord embed payload and dispatches it to the designated channel.
// It isolates the creation of the payload from the persistence of the posting state.
func (s *EmbedService) Post(client *api.Client, channelID discord.ChannelID, ce files.CustomEmbedConfig) (*discord.Message, error) {
	embed := s.Render(ce)
	data := api.SendMessageData{
		Embeds: []discord.Embed{embed},
	}
	return client.SendMessageComplex(channelID, data)
}

// DeletePosting executes a permanent removal of an embed message from Discord.
// It appends an audit reason to the deletion request for moderation transparency.
func (s *EmbedService) DeletePosting(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID) error {
	return client.DeleteMessage(channelID, messageID, "Embed unposted via command")
}

// Sync updates all active postings of a custom embed to match the provided layout.
// It implements a fault-tolerant batch reconciliation loop that distinguishes
// between transient API errors and permanently dropped identifiers (HTTP 10003/10008).
func (s *EmbedService) Sync(
	client *api.Client,
	guildID string,
	key string,
	postings []files.CustomEmbedPostingConfig,
	embed *discord.Embed,
) customEmbedSyncResult {
	var result customEmbedSyncResult
	if len(postings) == 0 {
		return result
	}

	var embeds []discord.Embed
	if embed != nil {
		embeds = []discord.Embed{*embed}
	}

	for _, posting := range postings {
		chID, errCh := discord.ParseSnowflake(posting.ChannelID)
		msgID, errMsg := discord.ParseSnowflake(posting.MessageID)
		if errCh != nil || errMsg != nil {
			result.Failed = append(result.Failed, customEmbedSyncFailure{Posting: posting, Err: errors.New("invalid snowflake")})
			continue
		}

		data := api.EditMessageData{
			Embeds: &embeds,
		}

		err := s.editMessage(client, discord.ChannelID(chID), discord.MessageID(msgID), data)
		if err == nil {
			result.Edited++
			continue
		}

		if isCustomEmbedPostingMissingError(err) {
			// Operational annotation: HTTP 10003 (Unknown Channel) and 10008 (Unknown Message)
			// indicate the posting was deleted natively on Discord. We accumulate these
			// to trigger a bulk retirement cleanup off the primary loop.
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
		// Operational annotation: We execute dropPostings synchronously rather than spawning
		// a background goroutine to guarantee deterministic state resolution before returning.
		if dropErr := s.dropPostings(s.configManager, guildID, key, ids); dropErr != nil {
			slog.Warn("Service degradation intercepted and mitigated",
				slog.String("reason", "Custom embed batch posting cleanup failed"),
				slog.String("guildID", guildID),
				slog.String("key", key),
				slog.String("error", dropErr.Error()),
			)
		}
	}

	return result
}

// Render converts the native configuration into a strict Discord payload struct.
// It applies semantic trimming on all textual fields to prevent whitespace rendering anomalies.
func Render(ce files.CustomEmbedConfig) discord.Embed {
	embed := discord.Embed{}
	if title := strings.TrimSpace(ce.Title); title != "" {
		embed.Title = title
	}
	if desc := strings.TrimSpace(ce.Description); desc != "" {
		embed.Description = desc
	}
	if ce.Color > 0 {
		embed.Color = discord.Color(ce.Color)
	}

	authorName := strings.TrimSpace(ce.AuthorName)
	authorIcon := strings.TrimSpace(ce.AuthorIconURL)
	if authorName != "" || authorIcon != "" {
		embed.Author = &discord.EmbedAuthor{
			Name: authorName,
			Icon: authorIcon,
		}
	}

	footerText := strings.TrimSpace(ce.FooterText)
	footerIcon := strings.TrimSpace(ce.FooterIconURL)
	if footerText != "" || footerIcon != "" {
		embed.Footer = &discord.EmbedFooter{
			Text: footerText,
			Icon: footerIcon,
		}
	}

	if imageURL := strings.TrimSpace(ce.ImageURL); imageURL != "" {
		embed.Image = &discord.EmbedImage{URL: imageURL}
	}
	if thumbnailURL := strings.TrimSpace(ce.ThumbnailURL); thumbnailURL != "" {
		embed.Thumbnail = &discord.EmbedThumbnail{URL: thumbnailURL}
	}

	if len(ce.Fields) > 0 {
		embed.Fields = make([]discord.EmbedField, 0, len(ce.Fields))
		for _, f := range ce.Fields {
			embed.Fields = append(embed.Fields, discord.EmbedField{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}

	return embed
}

// Render translates the custom embed configuration using the core utility function.
func (s *EmbedService) Render(ce files.CustomEmbedConfig) discord.Embed {
	return Render(ce)
}

// FormatSyncSummary maps the aggregated sync result structure into a human-readable diagnostic.
// It guarantees that dropped resources and transient failure states are accurately formatted.
func (s *EmbedService) FormatSyncSummary(result customEmbedSyncResult, action string) string {
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
	var httpErr *httputil.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Code == discordErrUnknownChannel || httpErr.Code == discordErrUnknownMessage
	}
	return false
}

func defaultCustomEmbedEditMessage(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID, data api.EditMessageData) error {
	if client == nil {
		return errors.New("discord client is nil")
	}
	_, err := client.EditMessageComplex(channelID, messageID, data)
	return err
}

func defaultCustomEmbedDropPostings(cm *files.ConfigManager, guildID, key string, messageIDs []string) error {
	if cm == nil {
		return errors.New("config manager is nil")
	}
	return cm.RemoveCustomEmbedPostings(guildID, key, messageIDs)
}
