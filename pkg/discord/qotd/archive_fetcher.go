package qotd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type ArchivedMessage struct {
	MessageID          string
	AuthorID           string
	AuthorNameSnapshot string
	AuthorIsBot        bool
	Content            string
	EmbedsJSON         json.RawMessage
	AttachmentsJSON    json.RawMessage
	CreatedAt          time.Time
}

func (p *Publisher) FetchThreadMessages(ctx context.Context, session *discordgo.Session, threadID string) ([]ArchivedMessage, error) {
	collected, err := fetchThreadMessagesRaw(ctx, session, threadID)
	if err != nil {
		return nil, fmt.Errorf("Publisher.FetchThreadMessages: %w", err)
	}

	return archiveMessagesAscending(collected), nil
}

func fetchThreadMessagesRaw(ctx context.Context, session *discordgo.Session, threadID string) (collected []*discordgo.Message, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("fetch qotd thread messages: %w", err)
		}
	}()
	if session == nil {
		return nil, errors.New("discord session is required")
	}
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return nil, errors.New("thread id is required")
	}

	collected = make([]*discordgo.Message, 0, 32)
	before := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("fetchThreadMessagesRaw: %w", err)
		}

		page, fetchErr := session.ChannelMessages(threadID, 100, before, "", "")
		if fetchErr != nil {
			return nil, fetchErr
		}
		if len(page) == 0 {
			break
		}

		collected = append(collected, page...)
		if len(page) < 100 {
			break
		}
		before = strings.TrimSpace(page[len(page)-1].ID)
		if before == "" {
			break
		}
	}

	return collected, nil
}

func archiveMessagesAscending(collected []*discordgo.Message) []ArchivedMessage {
	out := make([]ArchivedMessage, 0, len(collected))
	for idx := len(collected) - 1; idx >= 0; idx-- {
		if archived, ok := archiveMessage(collected[idx]); ok {
			out = append(out, archived)
		}
	}
	return out
}

func archiveMessage(message *discordgo.Message) (ArchivedMessage, bool) {
	if message == nil || strings.TrimSpace(message.ID) == "" {
		return ArchivedMessage{}, false
	}
	return ArchivedMessage{
		MessageID:          strings.TrimSpace(message.ID),
		AuthorID:           archiveAuthorID(message.Author),
		AuthorNameSnapshot: archiveAuthorName(message),
		AuthorIsBot:        message.Author != nil && message.Author.Bot,
		Content:            message.Content,
		EmbedsJSON:         marshalArchiveField(message.Embeds),
		AttachmentsJSON:    marshalArchiveField(message.Attachments),
		CreatedAt:          normalizeArchiveMessageTimestamp(message.Timestamp),
	}, true
}

func archiveAuthorID(author *discordgo.User) string {
	if author == nil {
		return ""
	}
	return strings.TrimSpace(author.ID)
}

func archiveAuthorName(message *discordgo.Message) string {
	if message == nil {
		return ""
	}
	if message.Member != nil {
		if value := strings.TrimSpace(message.Member.Nick); value != "" {
			return value
		}
	}
	if message.Author != nil {
		if value := strings.TrimSpace(message.Author.GlobalName); value != "" {
			return value
		}
		if value := strings.TrimSpace(message.Author.Username); value != "" {
			return value
		}
	}
	return ""
}

func marshalArchiveField(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil || string(raw) == "null" {
		return nil
	}
	return raw
}

func normalizeArchiveMessageTimestamp(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func isNotFoundRESTError(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr == nil {
		return false
	}
	if restErr.Response != nil && restErr.Response.StatusCode == http.StatusNotFound {
		return true
	}
	return restErr.Message != nil && restErr.Message.Code == discordgo.ErrCodeUnknownChannel
}
