package control

import (
	"strings"
	"time"

	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type qotdQuestionResponse struct {
	ID                  int64      `json:"id"`
	DeckID              string     `json:"deck_id"`
	Body                string     `json:"body"`
	Status              string     `json:"status"`
	QueuePosition       int64      `json:"queue_position"`
	CreatedBy           string     `json:"created_by,omitempty"`
	ScheduledForDateUTC *time.Time `json:"scheduled_for_date_utc,omitempty"`
	UsedAt              *time.Time `json:"used_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type qotdOfficialPostResponse struct {
	DeckID            string     `json:"deck_id"`
	DeckName          string     `json:"deck_name"`
	PublishMode       string     `json:"publish_mode"`
	PublishDateUTC    time.Time  `json:"publish_date_utc"`
	State             string     `json:"state"`
	QuestionText      string     `json:"question_text"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
	BecomesPreviousAt time.Time  `json:"becomes_previous_at"`
	AnswersCloseAt    time.Time  `json:"answers_close_at"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	ArchivedAt        *time.Time `json:"archived_at,omitempty"`
	ThreadID          string     `json:"thread_id,omitempty"`
	ThreadURL         string     `json:"thread_url,omitempty"`
	AnswerChannelID   string     `json:"answer_channel_id,omitempty"`
	AnswerChannelURL  string     `json:"answer_channel_url,omitempty"`
	PostURL           string     `json:"post_url,omitempty"`
}

type qotdDeckSummaryResponse struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Enabled        bool                `json:"enabled"`
	QuestionCounts qotd.QuestionCounts `json:"counts"`
	CardsRemaining int                 `json:"cards_remaining"`
	IsActive       bool                `json:"is_active"`
	CanPublish     bool                `json:"can_publish"`
}

type qotdSummaryResponse struct {
	Settings                files.QOTDConfig          `json:"settings"`
	Counts                  qotd.QuestionCounts       `json:"counts"`
	Decks                   []qotdDeckSummaryResponse `json:"decks,omitempty"`
	CurrentPublishDateUTC   time.Time                 `json:"current_publish_date_utc"`
	PublishedForCurrentSlot bool                      `json:"published_for_current_slot"`
	CurrentPost             *qotdOfficialPostResponse `json:"current_post,omitempty"`
	PreviousPost            *qotdOfficialPostResponse `json:"previous_post,omitempty"`
}

type qotdCollectedQuestionResponse struct {
	ID               int64     `json:"id"`
	SourceChannelID  string    `json:"source_channel_id"`
	SourceMessageID  string    `json:"source_message_id"`
	SourceAuthorID   string    `json:"source_author_id,omitempty"`
	SourceAuthorName string    `json:"source_author_name,omitempty"`
	SourceCreatedAt  time.Time `json:"source_created_at"`
	EmbedTitle       string    `json:"embed_title"`
	QuestionText     string    `json:"question_text"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type qotdCollectorSummaryResponse struct {
	TotalQuestions  int                             `json:"total_questions"`
	RecentQuestions []qotdCollectedQuestionResponse `json:"recent_questions,omitempty"`
}

type qotdCollectorRunResultResponse struct {
	ScannedMessages int `json:"scanned_messages"`
	MatchedMessages int `json:"matched_messages"`
	NewQuestions    int `json:"new_questions"`
	TotalQuestions  int `json:"total_questions"`
}

type qotdCollectorRemoveDuplicatesResultResponse struct {
	DeckID             string `json:"deck_id"`
	ScannedMessages    int    `json:"scanned_messages"`
	MatchedMessages    int    `json:"matched_messages"`
	DuplicateQuestions int    `json:"duplicate_questions"`
	DeletedQuestions   int    `json:"deleted_questions"`
}

func buildQOTDQuestionsResponse(records []storage.QOTDQuestionRecord) []qotdQuestionResponse {
	out := make([]qotdQuestionResponse, 0, len(records))
	for _, record := range records {
		record := record
		out = append(out, qotdQuestionResponse{
			ID:                  record.ID,
			DeckID:              strings.TrimSpace(record.DeckID),
			Body:                record.Body,
			Status:              strings.TrimSpace(record.Status),
			QueuePosition:       record.QueuePosition,
			CreatedBy:           strings.TrimSpace(record.CreatedBy),
			ScheduledForDateUTC: record.ScheduledForDateUTC,
			UsedAt:              record.UsedAt,
			CreatedAt:           record.CreatedAt.UTC(),
			UpdatedAt:           record.UpdatedAt.UTC(),
		})
	}
	return out
}

func buildQOTDOfficialPostResponse(guildID string, record *storage.QOTDOfficialPostRecord) *qotdOfficialPostResponse {
	if record == nil {
		return nil
	}
	resolved := resolveQOTDOfficialPost(guildID, record)
	publishDate := resolved.PublishDateUTC.UTC()
	now := time.Now().UTC()
	state := strings.TrimSpace(resolved.State)
	if resolved.ArchivedAt != nil && !resolved.ArchivedAt.IsZero() {
		state = string(qotd.OfficialPostStateArchived)
	} else {
		switch state {
		case "", string(qotd.OfficialPostStateCurrent), string(qotd.OfficialPostStatePrevious):
			state = string(qotd.StateWithinWindow(resolved.GraceUntil, resolved.ArchiveAt, now))
		case string(qotd.OfficialPostStateProvisioning):
			// preserve provisioning until the publish finishes.
		case string(qotd.OfficialPostStateArchiving),
			string(qotd.OfficialPostStateMissingDiscord),
			string(qotd.OfficialPostStateFailed),
			string(qotd.OfficialPostStateArchived):
			// preserve explicitly managed non-live states.
		default:
			state = string(qotd.StateWithinWindow(resolved.GraceUntil, resolved.ArchiveAt, now))
		}
	}
	answerChannelID := strings.TrimSpace(resolved.AnswerChannelID)
	if answerChannelID == "" {
		answerChannelID = strings.TrimSpace(resolved.DiscordThreadID)
	}
	return &qotdOfficialPostResponse{
		DeckID:            strings.TrimSpace(resolved.DeckID),
		DeckName:          strings.TrimSpace(resolved.DeckNameSnapshot),
		PublishMode:       strings.TrimSpace(resolved.PublishMode),
		PublishDateUTC:    publishDate,
		State:             strings.TrimSpace(state),
		QuestionText:      resolved.QuestionTextSnapshot,
		PublishedAt:       resolved.PublishedAt,
		BecomesPreviousAt: resolved.GraceUntil.UTC(),
		AnswersCloseAt:    resolved.ArchiveAt.UTC(),
		ClosedAt:          resolved.ClosedAt,
		ArchivedAt:        resolved.ArchivedAt,
		ThreadID:          strings.TrimSpace(resolved.DiscordThreadID),
		ThreadURL:         buildQOTDOfficialThreadJumpURL(resolved),
		AnswerChannelID:   answerChannelID,
		AnswerChannelURL:  buildQOTDAnswerChannelJumpURL(resolved, answerChannelID),
		PostURL:           qotd.OfficialPostJumpURL(resolved),
	}
}

func buildQOTDSummaryResponse(guildID string, summary qotd.Summary) qotdSummaryResponse {
	return qotdSummaryResponse{
		Settings:                summary.Settings,
		Counts:                  summary.Counts,
		Decks:                   buildQOTDDeckSummaryResponse(summary.Decks),
		CurrentPublishDateUTC:   summary.CurrentPublishDateUTC.UTC(),
		PublishedForCurrentSlot: summary.PublishedForCurrentSlot,
		CurrentPost:             buildQOTDOfficialPostResponse(guildID, summary.CurrentPost),
		PreviousPost:            buildQOTDOfficialPostResponse(guildID, summary.PreviousPost),
	}
}

func buildQOTDDeckSummaryResponse(decks []qotd.DeckSummary) []qotdDeckSummaryResponse {
	if len(decks) == 0 {
		return nil
	}
	out := make([]qotdDeckSummaryResponse, 0, len(decks))
	for _, deck := range decks {
		out = append(out, qotdDeckSummaryResponse{
			ID:             strings.TrimSpace(deck.Deck.ID),
			Name:           strings.TrimSpace(deck.Deck.Name),
			Enabled:        deck.Deck.Enabled,
			QuestionCounts: deck.Counts,
			CardsRemaining: deck.CardsRemaining,
			IsActive:       deck.IsActive,
			CanPublish:     deck.CanPublish,
		})
	}
	return out
}

func buildQOTDCollectedQuestionResponse(records []storage.QOTDCollectedQuestionRecord) []qotdCollectedQuestionResponse {
	if len(records) == 0 {
		return nil
	}
	out := make([]qotdCollectedQuestionResponse, 0, len(records))
	for _, record := range records {
		out = append(out, qotdCollectedQuestionResponse{
			ID:               record.ID,
			SourceChannelID:  strings.TrimSpace(record.SourceChannelID),
			SourceMessageID:  strings.TrimSpace(record.SourceMessageID),
			SourceAuthorID:   strings.TrimSpace(record.SourceAuthorID),
			SourceAuthorName: strings.TrimSpace(record.SourceAuthorNameSnapshot),
			SourceCreatedAt:  record.SourceCreatedAt.UTC(),
			EmbedTitle:       record.EmbedTitle,
			QuestionText:     record.QuestionText,
			CreatedAt:        record.CreatedAt.UTC(),
			UpdatedAt:        record.UpdatedAt.UTC(),
		})
	}
	return out
}

func buildQOTDCollectorSummaryResponse(summary qotd.CollectorSummary) qotdCollectorSummaryResponse {
	return qotdCollectorSummaryResponse{
		TotalQuestions:  summary.TotalQuestions,
		RecentQuestions: buildQOTDCollectedQuestionResponse(summary.RecentQuestions),
	}
}

func buildQOTDCollectorRunResultResponse(result qotd.CollectorRunResult) qotdCollectorRunResultResponse {
	return qotdCollectorRunResultResponse{
		ScannedMessages: result.ScannedMessages,
		MatchedMessages: result.MatchedMessages,
		NewQuestions:    result.NewQuestions,
		TotalQuestions:  result.TotalQuestions,
	}
}

func buildQOTDCollectorRemoveDuplicatesResultResponse(result qotd.CollectorRemoveDuplicatesResult) qotdCollectorRemoveDuplicatesResultResponse {
	return qotdCollectorRemoveDuplicatesResultResponse{
		DeckID:             strings.TrimSpace(result.DeckID),
		ScannedMessages:    result.ScannedMessages,
		MatchedMessages:    result.MatchedMessages,
		DuplicateQuestions: result.DuplicateQuestions,
		DeletedQuestions:   result.DeletedQuestions,
	}
}

func buildQOTDOfficialPostJumpURL(guildID string, record *storage.QOTDOfficialPostRecord) string {
	if record == nil {
		return ""
	}
	resolved := resolveQOTDOfficialPost(guildID, record)
	return qotd.OfficialPostJumpURL(resolved)
}

func resolveQOTDOfficialPost(guildID string, record *storage.QOTDOfficialPostRecord) storage.QOTDOfficialPostRecord {
	resolved := *record
	if strings.TrimSpace(resolved.GuildID) == "" {
		resolved.GuildID = strings.TrimSpace(guildID)
	}
	return resolved
}

func buildQOTDOfficialThreadJumpURL(record storage.QOTDOfficialPostRecord) string {
	return discordqotd.BuildThreadJumpURL(record.GuildID, record.DiscordThreadID)
}

func buildQOTDAnswerChannelJumpURL(record storage.QOTDOfficialPostRecord, answerChannelID string) string {
	return discordqotd.BuildChannelJumpURL(record.GuildID, answerChannelID)
}
