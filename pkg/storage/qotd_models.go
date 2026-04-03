package storage

import (
	"encoding/json"
	"time"
)

type QOTDQuestionRecord struct {
	ID                  int64
	GuildID             string
	Body                string
	Status              string
	QueuePosition       int64
	CreatedBy           string
	ScheduledForDateUTC *time.Time
	UsedAt              *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type QOTDOfficialPostRecord struct {
	ID                      int64
	GuildID                 string
	QuestionID              int64
	PublishDateUTC          time.Time
	State                   string
	ForumChannelID          string
	DiscordThreadID         string
	DiscordStarterMessageID string
	QuestionTextSnapshot    string
	IsPinned                bool
	PublishedAt             *time.Time
	GraceUntil              time.Time
	ArchiveAt               time.Time
	ClosedAt                *time.Time
	ArchivedAt              *time.Time
	LastReconciledAt        *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type QOTDReplyThreadRecord struct {
	ID                      int64
	GuildID                 string
	OfficialPostID          int64
	UserID                  string
	State                   string
	ForumChannelID          string
	DiscordThreadID         string
	DiscordStarterMessageID string
	CreatedViaInteractionID string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	ClosedAt                *time.Time
	ArchivedAt              *time.Time
}

type QOTDThreadArchiveRecord struct {
	ID              int64
	GuildID         string
	OfficialPostID  int64
	ReplyThreadID   *int64
	SourceKind      string
	DiscordThreadID string
	ArchivedAt      time.Time
	CreatedAt       time.Time
}

type QOTDMessageArchiveRecord struct {
	ID                 int64
	ThreadArchiveID    int64
	DiscordMessageID   string
	AuthorID           string
	AuthorNameSnapshot string
	AuthorIsBot        bool
	Content            string
	EmbedsJSON         json.RawMessage
	AttachmentsJSON    json.RawMessage
	CreatedAt          time.Time
}
