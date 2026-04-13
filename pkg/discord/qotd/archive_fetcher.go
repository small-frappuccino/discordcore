package qotd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
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
		return nil, err
	}

	return archiveMessagesAscending(collected), nil
}

func (p *Publisher) FetchChannelMessages(ctx context.Context, session *discordgo.Session, channelID, beforeMessageID string, limit int) ([]ArchivedMessage, error) {
	page, err := fetchChannelMessagesPageRaw(ctx, session, channelID, beforeMessageID, limit)
	if err != nil {
		return nil, err
	}
	return archiveMessagesDescending(page), nil
}

func fetchThreadMessagesRaw(ctx context.Context, session *discordgo.Session, threadID string) ([]*discordgo.Message, error) {
	if session == nil {
		return nil, fmt.Errorf("fetch qotd thread messages: discord session is required")
	}
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return nil, fmt.Errorf("fetch qotd thread messages: thread id is required")
	}

	collected := make([]*discordgo.Message, 0, 32)
	before := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		page, err := session.ChannelMessages(threadID, 100, before, "", "")
		if err != nil {
			return nil, fmt.Errorf("fetch qotd thread messages: %w", err)
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

func fetchChannelMessagesPageRaw(ctx context.Context, session *discordgo.Session, channelID, beforeMessageID string, limit int) ([]*discordgo.Message, error) {
	if session == nil {
		return nil, fmt.Errorf("fetch qotd channel messages: discord session is required")
	}
	channelID = strings.TrimSpace(channelID)
	beforeMessageID = strings.TrimSpace(beforeMessageID)
	if channelID == "" {
		return nil, fmt.Errorf("fetch qotd channel messages: channel id is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	page, err := session.ChannelMessages(channelID, limit, beforeMessageID, "", "")
	if err != nil {
		return nil, fmt.Errorf("fetch qotd channel messages: %w", err)
	}
	return page, nil
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

func archiveMessagesDescending(collected []*discordgo.Message) []ArchivedMessage {
	out := make([]ArchivedMessage, 0, len(collected))
	for _, message := range collected {
		if archived, ok := archiveMessage(message); ok {
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

func listRecoveryCandidateThreads(ctx context.Context, session *discordgo.Session, forumChannelID string, since time.Time, nameFragment string) ([]*discordgo.Channel, error) {
	forumChannelID = strings.TrimSpace(forumChannelID)
	nameFragment = strings.ToLower(strings.TrimSpace(nameFragment))
	if forumChannelID == "" {
		return nil, fmt.Errorf("list qotd recovery candidate threads: forum channel id is required")
	}
	if session == nil {
		return nil, fmt.Errorf("list qotd recovery candidate threads: discord session is required")
	}
	if !since.IsZero() {
		since = since.UTC()
	}

	seen := make(map[string]*discordgo.Channel)
	appendMatches := func(list *discordgo.ThreadsList, archived bool) {
		if list == nil {
			return
		}
		for _, thread := range list.Threads {
			if !threadMatchesRecoveryCandidate(thread, forumChannelID, nameFragment, since, archived) {
				continue
			}
			threadID := strings.TrimSpace(thread.ID)
			if threadID == "" {
				continue
			}
			if _, ok := seen[threadID]; !ok {
				seen[threadID] = thread
			}
		}
	}

	active, err := session.ThreadsActive(forumChannelID)
	if err != nil {
		return nil, fmt.Errorf("list qotd recovery candidate threads: active threads: %w", err)
	}
	appendMatches(active, false)

	before := time.Now().UTC()
	for page := 0; page < 3; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		archived, err := session.ThreadsArchived(forumChannelID, &before, 100)
		if err != nil {
			return nil, fmt.Errorf("list qotd recovery candidate threads: archived threads: %w", err)
		}
		appendMatches(archived, true)
		if archived == nil || !archived.HasMore || len(archived.Threads) == 0 {
			break
		}
		last := archived.Threads[len(archived.Threads)-1]
		if last == nil || last.ThreadMetadata == nil || last.ThreadMetadata.ArchiveTimestamp.IsZero() {
			break
		}
		before = last.ThreadMetadata.ArchiveTimestamp.Add(-time.Second)
		if !since.IsZero() && before.Before(since) {
			break
		}
	}

	out := make([]*discordgo.Channel, 0, len(seen))
	for _, thread := range seen {
		out = append(out, thread)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.TrimSpace(out[i].ID) < strings.TrimSpace(out[j].ID)
	})
	return out, nil
}

func threadMatchesRecoveryCandidate(thread *discordgo.Channel, forumChannelID, nameFragment string, since time.Time, archived bool) bool {
	if thread == nil {
		return false
	}
	if strings.TrimSpace(thread.ParentID) != forumChannelID {
		return false
	}
	if nameFragment != "" && !strings.Contains(strings.ToLower(strings.TrimSpace(thread.Name)), nameFragment) {
		return false
	}
	if archived && !since.IsZero() {
		if thread.ThreadMetadata == nil || thread.ThreadMetadata.ArchiveTimestamp.IsZero() {
			return false
		}
		if thread.ThreadMetadata.ArchiveTimestamp.UTC().Before(since) {
			return false
		}
	}
	return true
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
