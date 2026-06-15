package qotd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/log"
	domain "github.com/small-frappuccino/discordcore/pkg/qotd"
)

func mapArikawaError(err error) error {
	if err == nil {
		return nil
	}
	var httpErr *httputil.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Code {
		case 10003:
			return fmt.Errorf("%w: %s", domain.ErrDiscordUnknownChannel, httpErr.Message)
		case 10004:
			return fmt.Errorf("%w: %s", domain.ErrDiscordUnknownGuild, httpErr.Message)
		case 10008:
			return fmt.Errorf("%w: %s", domain.ErrDiscordUnknownMessage, httpErr.Message)
		case 50001:
			return fmt.Errorf("%w: %s", domain.ErrDiscordMissingAccess, httpErr.Message)
		case 50013:
			return fmt.Errorf("%w: %s", domain.ErrDiscordMissingPermissions, httpErr.Message)
		case 50074:
			return fmt.Errorf("%w: %s", domain.ErrDiscordCannotSendMessagesInVoice, httpErr.Message)
		case 50007:
			return fmt.Errorf("%w: %s", domain.ErrDiscordCannotSendMessagesToUser, httpErr.Message)
		case 160004:
			return fmt.Errorf("%w: %s", domain.ErrDiscordThreadAlreadyCreatedForThisMessage, httpErr.Message)
		}
		if httpErr.Status == 401 {
			return fmt.Errorf("%w: %s", domain.ErrDiscordUnauthorized, httpErr.Message)
		}
	}
	return err
}

// ArikawaPublisher implements the domain.Publisher interface using Arikawa.
type ArikawaPublisher struct {
	client *api.Client
}

// NewArikawaPublisher creates a new ArikawaPublisher.
func NewArikawaPublisher(s *state.State) *ArikawaPublisher {
	return &ArikawaPublisher{
		client: s.Client,
	}
}

// PublishOfficialPost implements domain.Publisher.
func (p *ArikawaPublisher) PublishOfficialPost(ctx context.Context, params domain.PublishOfficialPostParams) (*domain.PublishedOfficialPost, error) {
	channelID, err := discord.ParseSnowflake(params.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("invalid channel ID %q: %w", params.ChannelID, err)
	}

	result := &domain.PublishedOfficialPost{
		QuestionListThreadID:       params.QuestionListThreadID,
		QuestionListEntryMessageID: params.QuestionListEntryMessageID,
		ThreadID:                   params.OfficialThreadID,
		StarterMessageID:           params.OfficialStarterMessageID,
		AnswerChannelID:            params.OfficialAnswerChannelID,
		PublishedAt:                params.ExistingPublishedAt,
	}

	if result.StarterMessageID == "" {
		embed := buildArikawaOfficialQuestionEmbed(params.DeckName, params.AvailableQuestions, params.QuestionText, params.DisplayID)

		sendData := api.SendMessageData{
			Embeds: []discord.Embed{embed},
		}

		nonce := strings.TrimSpace(params.Nonce)
		if nonce != "" {
			sendData.Nonce = nonce
		}

		c := p.client.WithContext(ctx)
		message, err := c.SendMessageComplex(discord.ChannelID(channelID), sendData)
		if err != nil {
			return result, fmt.Errorf("create qotd starter message: %w", mapArikawaError(err))
		}
		result.StarterMessageID = message.ID.String()
		if result.PublishedAt.IsZero() {
			result.PublishedAt = time.Now().UTC()
		}
	}

	if result.ThreadID == "" {
		messageID, err := discord.ParseSnowflake(result.StarterMessageID)
		if err != nil {
			return result, fmt.Errorf("invalid starter message ID %q: %w", result.StarterMessageID, err)
		}

		threadName := buildOfficialPostName(params.PublishDateUTC, params.DisplayID, params.ThreadName)
		threadData := api.StartThreadData{
			Name:                threadName,
			AutoArchiveDuration: discord.ArchiveDuration(defaultThreadAutoArchiveMinutes),
		}

		c := p.client.WithContext(ctx)
		thread, err := c.StartThreadWithMessage(discord.ChannelID(channelID), discord.MessageID(messageID), threadData)
		if err != nil {
			// Arikawa might return a specific error for already exists. For now, fallback logic:
			if strings.Contains(err.Error(), "already has a thread") || strings.Contains(err.Error(), "Thread already exists") || strings.Contains(err.Error(), "160004") {
				// thread already exists, let's fetch message
				existingMsg, lookupErr := c.Message(discord.ChannelID(channelID), discord.MessageID(messageID))
				if lookupErr != nil {
					return result, fmt.Errorf("lookup existing qotd thread after retry: %w", mapArikawaError(lookupErr))
				}
				result.ThreadID = existingMsg.ID.String() // In Discord, a thread created from a message shares the same ID as the message.
			} else if isArikawaAutoArchiveDurationRejection(err) {
				// Retry with fallback duration
				threadData.AutoArchiveDuration = discord.ArchiveDuration(fallbackThreadAutoArchiveMinutes)
				thread, err = c.StartThreadWithMessage(discord.ChannelID(channelID), discord.MessageID(messageID), threadData)
				if err != nil {
					return result, fmt.Errorf("create qotd daily thread fallback: %w", mapArikawaError(err))
				}
				result.ThreadID = thread.ID.String()
			} else {
				return result, fmt.Errorf("create qotd daily thread: %w", mapArikawaError(err))
			}
		} else {
			result.ThreadID = thread.ID.String()
		}

		if result.PublishedAt.IsZero() {
			result.PublishedAt = time.Now().UTC()
		}
	}

	if result.AnswerChannelID == "" && result.ThreadID != "" {
		result.AnswerChannelID = result.ThreadID
	}

	// Set PostURL
	if result.ThreadID != "" {
		result.PostURL = fmt.Sprintf("https://discord.com/channels/%s/%s/%s", params.GuildID, result.ThreadID, result.StarterMessageID)
	}

	return result, nil
}

// SetThreadState implements domain.Publisher.
func (p *ArikawaPublisher) SetThreadState(ctx context.Context, guildID string, threadID string, state domain.ThreadState) error {
	id, err := discord.ParseSnowflake(threadID)
	if err != nil {
		return fmt.Errorf("invalid thread ID %q: %w", threadID, err)
	}

	data := api.ModifyChannelData{
		Archived: option.PtrTo(state.Archived),
		Locked:   option.PtrTo(state.Locked),
	}

	c := p.client.WithContext(ctx)
	err = c.ModifyChannel(discord.ChannelID(id), data)
	if err != nil {
		return mapArikawaError(err)
	}
	return nil
}

// DeleteOfficialPost implements domain.Publisher.
func (p *ArikawaPublisher) DeleteOfficialPost(ctx context.Context, params domain.DeleteOfficialPostParams) error {
	c := p.client.WithContext(ctx)
	if params.DiscordThreadID != "" {
		threadID, err := discord.ParseSnowflake(params.DiscordThreadID)
		if err == nil {
			err = c.DeleteChannel(discord.ChannelID(threadID), api.AuditLogReason("qotd replace"))
			if err != nil {
				log.ApplicationLogger().Warn("qotd skip failed to delete discord thread", "guildID", params.GuildID, "threadID", params.DiscordThreadID, "err", mapArikawaError(err))
			}
		}
	}

	if params.DiscordStarterMessageID != "" && params.ChannelID != "" {
		channelID, errC := discord.ParseSnowflake(params.ChannelID)
		messageID, errM := discord.ParseSnowflake(params.DiscordStarterMessageID)
		if errC == nil && errM == nil {
			err := c.DeleteMessage(discord.ChannelID(channelID), discord.MessageID(messageID), "qotd replace")
			if err != nil {
				log.ApplicationLogger().Warn("qotd skip failed to delete starter message", "guildID", params.GuildID, "channelID", params.ChannelID, "messageID", params.DiscordStarterMessageID, "err", mapArikawaError(err))
			}
		}
	}

	if params.QuestionListEntryMessageID != "" && params.QuestionListThreadID != "" {
		threadID, errC := discord.ParseSnowflake(params.QuestionListThreadID)
		messageID, errM := discord.ParseSnowflake(params.QuestionListEntryMessageID)
		if errC == nil && errM == nil {
			err := c.DeleteMessage(discord.ChannelID(threadID), discord.MessageID(messageID), "qotd replace")
			if err != nil {
				log.ApplicationLogger().Warn("qotd skip failed to delete question list entry message", "guildID", params.GuildID, "threadID", params.QuestionListThreadID, "messageID", params.QuestionListEntryMessageID, "err", mapArikawaError(err))
			}
		}
	}

	return nil
}

func buildArikawaOfficialQuestionEmbed(deckName string, availableQuestions int, questionText string, displayID int64) discord.Embed {
	embed := discord.Embed{
		Title:       "☆ question!! ☆",
		Color:       officialQuestionEmbedColor,
		Description: normalizeOfficialQuestionText(questionText),
	}

	embed.Footer = &discord.EmbedFooter{
		Text: buildOfficialQuestionFooter(deckName, availableQuestions, displayID),
	}
	return embed
}

func isArikawaAutoArchiveDurationRejection(err error) bool {
	// Simple heuristic: check if the API error is a validation error containing archive duration.
	// We could parse the JSONError from Arikawa to be exact.
	return strings.Contains(err.Error(), "auto_archive_duration")
}
