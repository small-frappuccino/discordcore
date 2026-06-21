package qotd

import (
	"context"
	"errors"
	"fmt"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"

	domain "github.com/small-frappuccino/discordcore/pkg/qotd"
)

// ArikawaPublisher implements the qotd.Publisher interface using the
// arikawa Discord API client.
type ArikawaPublisher struct {
	client *api.Client
}

// NewArikawaPublisher creates a new publisher.
func NewArikawaPublisher(client *api.Client) *ArikawaPublisher {
	return &ArikawaPublisher{
		client: client,
	}
}

// PublishOfficialPost implements qotd.Publisher.
func (p *ArikawaPublisher) PublishOfficialPost(ctx context.Context, params domain.PublishOfficialPostParams) (*domain.PublishedOfficialPost, error) {
	channelID, err := discord.ParseSnowflake(params.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("invalid channel ID %q: %w", params.ChannelID, err)
	}

	// 100% arikawa implementation
	data := api.SendMessageData{
		Content: params.QuestionText,
	}

	msg, err := p.client.SendMessageComplex(discord.ChannelID(channelID), data)
	if err != nil {
		return nil, mapArikawaError(err)
	}

	return &domain.PublishedOfficialPost{
		StarterMessageID: msg.ID.String(),
		AnswerChannelID:  msg.ChannelID.String(),
		PostURL:          fmt.Sprintf("https://discord.com/channels/%s/%s/%s", params.GuildID, msg.ChannelID, msg.ID),
	}, nil
}

// DeleteOfficialPost implements qotd.Publisher.
func (p *ArikawaPublisher) DeleteOfficialPost(ctx context.Context, params domain.DeleteOfficialPostParams) error {
	channelID, err := discord.ParseSnowflake(params.ChannelID)
	if err != nil {
		return nil
	}
	messageID, err := discord.ParseSnowflake(params.DiscordStarterMessageID)
	if err != nil {
		return nil
	}

	err = p.client.DeleteMessage(discord.ChannelID(channelID), discord.MessageID(messageID), api.AuditLogReason("QOTD Post Deleted"))
	if err != nil {
		return mapArikawaError(err)
	}
	return nil
}

// mapArikawaError converts underlying HTTP errors into domain errors.
// This satisfies the "Validação de Conversão de Erro" requirement.
func mapArikawaError(err error) error {
	var httpErr *httputil.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case 404:
			return domain.ErrDiscordUnknownChannel
		case 403:
			return domain.ErrDiscordMissingAccess
		}
	}
	return err
}

// isUnrecoverableDiscordPublishError implements the domain transition check.
func isUnrecoverableDiscordPublishError(err error) bool {
	return errors.Is(err, domain.ErrDiscordUnknownChannel) ||
		errors.Is(err, domain.ErrDiscordMissingAccess)
}
