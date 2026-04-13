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
	ErrOfficialPostNotFound   = errors.New("qotd official post not found")
	ErrAnswerWindowClosed     = errors.New("qotd answer window closed")
	ErrReplyThreadUnavailable = errors.New("qotd reply thread creation is unavailable")
)

const (
	answerModalCustomID = "qotd:answer:submit:%d"
	answerModalFieldID  = "qotd:answer:body"
	answerModalTitleFmt = "Answer QOTD #%d"
)

type SubmitAnswerParams struct {
	GuildID         string
	OfficialPostID  int64
	UserID          string
	UserDisplayName string
	UserAvatarURL   string
	InteractionID   string
	AnswerText      string
}

type SubmitAnswerResult struct {
	MessageID  string
	ChannelID  string
	MessageURL string
	Updated    bool
}

type AnswerSubmissionService interface {
	SubmitAnswer(ctx context.Context, session *discordgo.Session, params SubmitAnswerParams) (*SubmitAnswerResult, error)
}

func HandleQOTDInteractions(service AnswerSubmissionService) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if service == nil || s == nil || i == nil || i.Interaction == nil {
			return
		}

		switch i.Type {
		case discordgo.InteractionMessageComponent:
			handleAnswerButtonInteraction(s, i)
		case discordgo.InteractionModalSubmit:
			handleAnswerModalSubmit(service, s, i)
		}
	}
}

func handleAnswerButtonInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	officialPostID, ok := parseAnswerButtonCustomID(i.MessageComponentData().CustomID)
	if !ok {
		return
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf(answerModalCustomID, officialPostID),
			Title:    fmt.Sprintf(answerModalTitleFmt, officialPostID),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    answerModalFieldID,
							Label:       "Your answer",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "Write your answer here",
							Required:    true,
							MinLength:   1,
							MaxLength:   4000,
						},
					},
				},
			},
		},
	}); err != nil {
		log.ApplicationLogger().Warn(
			"QOTD interaction open-modal failed",
			"guildID", i.GuildID,
			"channelID", i.ChannelID,
			"interactionID", i.ID,
			"officialPostID", officialPostID,
			"err", err,
		)
	}
}

func handleAnswerModalSubmit(service AnswerSubmissionService, s *discordgo.Session, i *discordgo.InteractionCreate) {
	modalData := i.ModalSubmitData()
	officialPostID, ok := parseAnswerModalCustomID(modalData.CustomID)
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
	result, err := service.SubmitAnswer(context.Background(), s, SubmitAnswerParams{
		GuildID:         strings.TrimSpace(i.GuildID),
		OfficialPostID:  officialPostID,
		UserID:          userID,
		UserDisplayName: interactionUserDisplayName(i),
		UserAvatarURL:   interactionUserAvatarURL(i),
		InteractionID:   strings.TrimSpace(i.ID),
		AnswerText:      extractModalValue(modalData, answerModalFieldID),
	})
	if err != nil {
		log.ApplicationLogger().Warn(
			"QOTD answer submission failed",
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

	content := fmt.Sprintf("Your answer was posted: %s", result.MessageURL)
	if result.Updated {
		content = fmt.Sprintf("Your answer was updated: %s", result.MessageURL)
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

func parseAnswerModalCustomID(customID string) (int64, bool) {
	customID = strings.TrimSpace(customID)
	prefix := strings.TrimSuffix(answerModalCustomID, "%d")
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
	default:
		return "Could not post your QOTD answer right now."
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

func interactionUserAvatarURL(i *discordgo.InteractionCreate) string {
	if i == nil || i.Interaction == nil {
		return ""
	}
	if i.Member != nil && i.Member.User != nil {
		return strings.TrimSpace(i.Member.User.AvatarURL("256"))
	}
	if i.User != nil {
		return strings.TrimSpace(i.User.AvatarURL("256"))
	}
	return ""
}

func extractModalValue(m discordgo.ModalSubmitInteractionData, fieldID string) string {
	for _, comp := range m.Components {
		row, ok := comp.(*discordgo.ActionsRow)
		if !ok || row == nil {
			continue
		}
		for _, c := range row.Components {
			ti, ok := c.(*discordgo.TextInput)
			if ok && ti.CustomID == fieldID {
				return strings.TrimSpace(ti.Value)
			}
		}
	}
	return ""
}
