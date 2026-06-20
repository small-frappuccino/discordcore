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

type customEmbedSyncResult struct {
	Edited  int
	Dropped []files.CustomEmbedPostingConfig
	Failed  []customEmbedSyncFailure
}

// HasIssues has issues.
func (r customEmbedSyncResult) HasIssues() bool {
	return len(r.Dropped) > 0 || len(r.Failed) > 0
}

// EmbedService manages the rendering and synchronization of custom embeds.
type EmbedService struct {
	configManager *files.ConfigManager
	editMessage   func(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID, data api.EditMessageData) error
	dropPostings  func(cm *files.ConfigManager, guildID, key string, messageIDs []string) error
}

// NewEmbedService creates a new embed domain service.
func NewEmbedService(configManager *files.ConfigManager) *EmbedService {
	return &EmbedService{
		configManager: configManager,
		editMessage:   defaultCustomEmbedEditMessage,
		dropPostings:  defaultCustomEmbedDropPostings,
	}
}

// Post sends a custom embed to a specific channel.
func (s *EmbedService) Post(client *api.Client, channelID discord.ChannelID, ce files.CustomEmbedConfig) (*discord.Message, error) {
	embed := s.Render(ce)
	data := api.SendMessageData{
		Embeds: []discord.Embed{embed},
	}
	return client.SendMessageComplex(channelID, data)
}

// DeletePosting deletes a custom embed posting from a channel.
func (s *EmbedService) DeletePosting(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID) error {
	return client.DeleteMessage(channelID, messageID, "Embed unposted via command")
}

// Sync updates all active postings of a custom embed to match the provided layout.
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

// Render returns the Discord embed payload for a given custom embed configuration.
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

// Render returns the Discord embed payload for a given custom embed configuration.
func (s *EmbedService) Render(ce files.CustomEmbedConfig) discord.Embed {
	return Render(ce)
}

// FormatSyncSummary returns a human-readable summary of the sync operation.
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
