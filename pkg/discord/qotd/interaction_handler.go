package qotd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

var (
	ErrOfficialPostNotFound    = errors.New("qotd official post not found")
	ErrAnswerWindowClosed      = errors.New("qotd answer window closed")
	ErrReplyThreadProvisioning = errors.New("qotd reply thread provisioning in progress")
	ErrReplyThreadUnavailable  = errors.New("qotd reply thread creation is unavailable")
)

type EnsureReplyThreadParams struct {
	GuildID          string
	OfficialPostID   int64
	OfficialThreadID string
	UserID           string
	UserDisplayName  string
	InteractionID    string
}

type EnsureReplyThreadResult struct {
	ThreadID         string
	StarterMessageID string
	ThreadURL        string
	Reused           bool
}

type ReplyThreadService interface {
	EnsureReplyThread(ctx context.Context, session *discordgo.Session, params EnsureReplyThreadParams) (*EnsureReplyThreadResult, error)
}

func HandleAnswerButtonInteractions(service ReplyThreadService) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if service == nil || s == nil || i == nil || i.Interaction == nil || i.Type != discordgo.InteractionMessageComponent {
			return
		}

		officialPostID, ok := parseAnswerButtonCustomID(i.MessageComponentData().CustomID)
		if !ok {
			return
		}

		if err := respondQOTDInteractionDeferred(s, i); err != nil {
			log.ApplicationLogger().Warn(
				"QOTD interaction respond failed",
				"guildID", i.GuildID,
				"channelID", i.ChannelID,
				"interactionID", i.ID,
				"officialPostID", officialPostID,
				"err", err,
			)
			return
		}

		userID := interactionUserID(i)
		result, err := service.EnsureReplyThread(context.Background(), s, EnsureReplyThreadParams{
			GuildID:          strings.TrimSpace(i.GuildID),
			OfficialPostID:   officialPostID,
			OfficialThreadID: strings.TrimSpace(i.ChannelID),
			UserID:           userID,
			UserDisplayName:  interactionUserDisplayName(i),
			InteractionID:    strings.TrimSpace(i.ID),
		})
		if err != nil {
			log.ApplicationLogger().Warn(
				"QOTD reply-thread interaction failed",
				"guildID", i.GuildID,
				"channelID", i.ChannelID,
				"userID", userID,
				"interactionID", i.ID,
				"officialPostID", officialPostID,
				"err", err,
			)
			_ = editQOTDInteractionResponse(s, i, qotdInteractionErrorMessage(err))
			return
		}

		content := fmt.Sprintf("Reply thread ready: %s", result.ThreadURL)
		if result.Reused {
			content = fmt.Sprintf("You already have a reply thread for this question: %s", result.ThreadURL)
		}
		if err := editQOTDInteractionResponse(s, i, content); err != nil {
			log.ApplicationLogger().Warn(
				"QOTD interaction edit failed",
				"guildID", i.GuildID,
				"channelID", i.ChannelID,
				"userID", userID,
				"interactionID", i.ID,
				"officialPostID", officialPostID,
				"err", err,
			)
		}
	}
}

func parseAnswerButtonCustomID(customID string) (int64, bool) {
	customID = strings.TrimSpace(customID)
	prefix := strings.TrimSuffix(answerButtonCustomID, "%d")
	if !strings.HasPrefix(customID, prefix) {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(customID, prefix), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func respondQOTDInteractionDeferred(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
}

func editQOTDInteractionResponse(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
	return err
}

func qotdInteractionErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrOfficialPostNotFound):
		return "This QOTD post could not be resolved anymore."
	case errors.Is(err, ErrAnswerWindowClosed):
		return "This question is no longer accepting replies."
	case errors.Is(err, ErrReplyThreadProvisioning):
		return "Your reply thread is already being prepared. Try again in a moment."
	default:
		return "Could not open your QOTD reply thread right now."
	}
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i == nil || i.Interaction == nil {
		return ""
	}
	if i.Member != nil && i.Member.User != nil {
		return strings.TrimSpace(i.Member.User.ID)
	}
	if i.User != nil {
		return strings.TrimSpace(i.User.ID)
	}
	return ""
}

func interactionUserDisplayName(i *discordgo.InteractionCreate) string {
	if i == nil || i.Interaction == nil {
		return ""
	}
	if i.Member != nil {
		if value := strings.TrimSpace(i.Member.Nick); value != "" {
			return value
		}
		if i.Member.User != nil {
			if value := strings.TrimSpace(i.Member.User.GlobalName); value != "" {
				return value
			}
			if value := strings.TrimSpace(i.Member.User.Username); value != "" {
				return value
			}
		}
	}
	if i.User != nil {
		if value := strings.TrimSpace(i.User.GlobalName); value != "" {
			return value
		}
		if value := strings.TrimSpace(i.User.Username); value != "" {
			return value
		}
	}
	return ""
}
