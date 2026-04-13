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
	ID                      int64      `json:"id"`
	QuestionID              int64      `json:"question_id"`
	PublishMode             string     `json:"publish_mode"`
	PublishDateUTC          time.Time  `json:"publish_date_utc"`
	State                   string     `json:"state"`
	QuestionChannelID       string     `json:"question_channel_id"`
	DiscordThreadID         string     `json:"discord_thread_id,omitempty"`
	DiscordStarterMessageID string     `json:"discord_starter_message_id,omitempty"`
	QuestionTextSnapshot    string     `json:"question_text_snapshot"`
	IsPinned                bool       `json:"is_pinned"`
	PublishedAt             *time.Time `json:"published_at,omitempty"`
	GraceUntil              time.Time  `json:"grace_until"`
	ArchiveAt               time.Time  `json:"archive_at"`
	ClosedAt                *time.Time `json:"closed_at,omitempty"`
	ArchivedAt              *time.Time `json:"archived_at,omitempty"`
	PostURL                 string     `json:"post_url,omitempty"`
}

type qotdSummaryResponse struct {
	Settings                files.QOTDConfig          `json:"settings"`
	Counts                  qotd.QuestionCounts       `json:"counts"`
	CurrentPublishDateUTC   time.Time                 `json:"current_publish_date_utc"`
	PublishedForCurrentSlot bool                      `json:"published_for_current_slot"`
	CurrentPost             *qotdOfficialPostResponse `json:"current_post,omitempty"`
	PreviousPost            *qotdOfficialPostResponse `json:"previous_post,omitempty"`
}

func buildQOTDQuestionsResponse(records []storage.QOTDQuestionRecord) []qotdQuestionResponse {
	out := make([]qotdQuestionResponse, 0, len(records))
	for _, record := range records {
		record := record
		out = append(out, qotdQuestionResponse{
			ID:                  record.ID,
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
	publishDate := record.PublishDateUTC.UTC()
	now := time.Now().UTC()
	state := strings.TrimSpace(record.State)
	if record.ArchivedAt != nil && !record.ArchivedAt.IsZero() {
		state = string(qotd.OfficialPostStateArchived)
	} else {
		switch state {
		case "", string(qotd.OfficialPostStateCurrent), string(qotd.OfficialPostStatePrevious):
			state = string(qotd.StateWithinWindow(record.GraceUntil, record.ArchiveAt, now))
		case string(qotd.OfficialPostStateProvisioning):
			// preserve provisioning until the publish finishes.
		case string(qotd.OfficialPostStateArchiving),
			string(qotd.OfficialPostStateMissingDiscord),
			string(qotd.OfficialPostStateFailed),
			string(qotd.OfficialPostStateArchived):
			// preserve explicitly managed non-live states.
		default:
			state = string(qotd.StateWithinWindow(record.GraceUntil, record.ArchiveAt, now))
		}
	}
	return &qotdOfficialPostResponse{
		ID:                      record.ID,
		QuestionID:              record.QuestionID,
		PublishMode:             strings.TrimSpace(record.PublishMode),
		PublishDateUTC:          publishDate,
		State:                   strings.TrimSpace(state),
		QuestionChannelID:       strings.TrimSpace(record.ForumChannelID),
		DiscordThreadID:         strings.TrimSpace(record.DiscordThreadID),
		DiscordStarterMessageID: strings.TrimSpace(record.DiscordStarterMessageID),
		QuestionTextSnapshot:    record.QuestionTextSnapshot,
		IsPinned:                record.IsPinned,
		PublishedAt:             record.PublishedAt,
		GraceUntil:              record.GraceUntil.UTC(),
		ArchiveAt:               record.ArchiveAt.UTC(),
		ClosedAt:                record.ClosedAt,
		ArchivedAt:              record.ArchivedAt,
		PostURL:                 buildQOTDOfficialPostJumpURL(guildID, record),
	}
}

func buildQOTDSummaryResponse(guildID string, summary qotd.Summary) qotdSummaryResponse {
	return qotdSummaryResponse{
		Settings:                summary.Settings,
		Counts:                  summary.Counts,
		CurrentPublishDateUTC:   summary.CurrentPublishDateUTC.UTC(),
		PublishedForCurrentSlot: summary.PublishedForCurrentSlot,
		CurrentPost:             buildQOTDOfficialPostResponse(guildID, summary.CurrentPost),
		PreviousPost:            buildQOTDOfficialPostResponse(guildID, summary.PreviousPost),
	}
}

func buildQOTDOfficialPostJumpURL(guildID string, record *storage.QOTDOfficialPostRecord) string {
	if record == nil {
		return ""
	}
	if threadID := strings.TrimSpace(record.DiscordThreadID); threadID != "" {
		return discordqotd.BuildThreadJumpURL(guildID, threadID)
	}
	channelID := strings.TrimSpace(record.ForumChannelID)
	messageID := strings.TrimSpace(record.DiscordStarterMessageID)
	if channelID == "" || messageID == "" {
		return ""
	}
	return discordqotd.BuildMessageJumpURL(guildID, channelID, messageID)
}
