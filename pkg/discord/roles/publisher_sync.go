package discordroles

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/roles"
	"github.com/small-frappuccino/discordgo"
)

const (
	discordErrUnknownChannel = 10003
	discordErrUnknownMessage = 10008
)

// Publisher edits the recorded postings of a panel to match a target state.
type Publisher struct {
	session            *discordgo.Session
	configManager      *files.ConfigManager
	editMessage        func(s *discordgo.Session, edit *discordgo.MessageEdit) error
	editWebhookMessage func(s *discordgo.Session, webhookID, webhookToken, messageID string, edit *discordgo.WebhookEdit) error
	dropPostings       func(cm *files.ConfigManager, guildID, key string, messageIDs []string) error
}

func NewPublisher(session *discordgo.Session, cm *files.ConfigManager) *Publisher {
	return &Publisher{
		session:            session,
		configManager:      cm,
		editMessage:        defaultRolePanelEditMessage,
		editWebhookMessage: defaultRolePanelEditWebhookMessage,
		dropPostings:       defaultRolePanelDropPostings,
	}
}

// Sync iterates the supplied postings and edits each Discord message
// to carry the supplied embed + components.
func (s *Publisher) Sync(guildID string, key string, postings []files.RolePanelPostingConfig, panel *files.RolePanelConfig) roles.SyncResult {
	var result roles.SyncResult
	if len(postings) == 0 {
		return result
	}

	embed := RenderEmbed(*panel)
	components := RenderComponents(*panel)

	embeds := []*discordgo.MessageEmbed{}
	if embed != nil {
		embeds = []*discordgo.MessageEmbed{embed}
	}

	for _, posting := range postings {
		var err error
		if posting.WebhookID != "" && posting.WebhookToken != "" {
			err = s.editWebhookMessage(s.session, posting.WebhookID, posting.WebhookToken, posting.MessageID, &discordgo.WebhookEdit{
				Embeds:     &embeds,
				Components: &components,
			})
		} else {
			err = s.editMessage(s.session, &discordgo.MessageEdit{
				ID:         strings.TrimSpace(posting.MessageID),
				Channel:    strings.TrimSpace(posting.ChannelID),
				Embeds:     &embeds,
				Components: &components,
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

		result.Failed = append(result.Failed, roles.SyncFailure{Posting: posting, Err: err})
	}

	if len(result.Dropped) > 0 {
		ids := make([]string, 0, len(result.Dropped))
		for _, p := range result.Dropped {
			ids = append(ids, p.MessageID)
		}
		if dropErr := s.dropPostings(s.configManager, guildID, key, ids); dropErr != nil {
			slog.Warn("Role panel batch posting cleanup failed",
				"guildID", guildID,
				"key", key,
				"err", dropErr,
			)
		}
	}

	return result
}

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
