package storage

import (
	"encoding/json"
	"time"
)

type QOTDQuestionRecord struct {
	ID                  int64
	DisplayID           int64
	GuildID             string
	DeckID              string
	Body                string
	Status              string
	QueuePosition       int64
	CreatedBy           string
	ScheduledForDateUTC *time.Time
	UsedAt              *time.Time
	PublishedOnceAt     *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type QOTDOfficialPostRecord struct {
	ID                         int64
	GuildID                    string
	DeckID                     string
	DeckNameSnapshot           string
	QuestionID                 int64
	PublishMode                string
	ConsumeAutomaticSlot       bool
	PublishDateUTC             time.Time
	State                      string
	ChannelID                  string
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	DiscordThreadID            string
	DiscordStarterMessageID    string
	AnswerChannelID            string
	QuestionTextSnapshot       string
	// Nonce is sent to Discord with enforce_nonce=true so that retried
	// publishes after a crash (record persisted, Discord call accepted, but
	// the message ID never made it back to our DB) deduplicate server-side
	// instead of producing a second QOTD post in the channel. Empty for
	// legacy records created before the column existed; the publisher falls
	// back to the non-idempotent send path in that case.
	Nonce            string
	PublishedAt      *time.Time
	GraceUntil       time.Time
	ArchiveAt        time.Time
	ClosedAt         *time.Time
	ArchivedAt       *time.Time
	LastReconciledAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type QOTDSurfaceRecord struct {
	ID                   int64
	GuildID              string
	DeckID               string
	ChannelID            string
	QuestionListThreadID string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type QOTDAnswerMessageRecord struct {
	ID                      int64
	GuildID                 string
	OfficialPostID          int64
	UserID                  string
	State                   string
	AnswerChannelID         string
	DiscordMessageID        string
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

type QOTDCollectedQuestionRecord struct {
	ID                       int64
	GuildID                  string
	SourceChannelID          string
	SourceMessageID          string
	SourceAuthorID           string
	SourceAuthorNameSnapshot string
	SourceCreatedAt          time.Time
	EmbedTitle               string
	QuestionText             string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
