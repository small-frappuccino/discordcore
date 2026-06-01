package storage

import (
	"time"
)

// QOTDQuestionSelector controls how the next question is picked when
// reserving for a publish. It is decoupled from QOTDOfficialPostRecord's
// PublishOrdinal so the visible thread numbering stays monotonic regardless
// of which strategy ran.
type QOTDQuestionSelector string

const (
	// QOTDQuestionSelectorQueue picks the head of the queue (queue_position
	// ASC, id ASC). This is the historical default.
	QOTDQuestionSelectorQueue QOTDQuestionSelector = "queue"
	// QOTDQuestionSelectorRandom picks a uniformly-random eligible question.
	QOTDQuestionSelectorRandom QOTDQuestionSelector = "random"
)

func (s QOTDQuestionSelector) normalized() QOTDQuestionSelector {
	switch s {
	case QOTDQuestionSelectorRandom:
		return QOTDQuestionSelectorRandom
	default:
		return QOTDQuestionSelectorQueue
	}
}

// orderByClause returns the SQL ORDER BY fragment that implements the
// strategy. It is composed into the reserve queries and is intentionally
// bind-parameter-free so it can be inlined safely.
func (s QOTDQuestionSelector) orderByClause() string {
	if s.normalized() == QOTDQuestionSelectorRandom {
		return "ORDER BY RANDOM()"
	}
	return "ORDER BY queue_position ASC, id ASC"
}

// QOTDQuestionRecord is a stored QOTD question. ID is the global primary key;
// DisplayID is the stable per-guild identifier shown to users. Status holds a
// qotd.QuestionStatus value, and the *time.Time fields are nil until the
// corresponding lifecycle event (scheduled, used, first published) occurs.
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

// QOTDOfficialPostRecord is the durable state of one official QOTD post and the
// source of truth the reconcile loop drives toward Discord. State holds a
// qotd.OfficialPostState value. Two of its fields anchor publish idempotency
// (see the PublishOrdinal and Nonce field comments); changing publish paths
// must preserve both.
type QOTDOfficialPostRecord struct {
	ID               int64
	GuildID          string
	DeckID           string
	DeckNameSnapshot string
	QuestionID       int64
	// PublishOrdinal is the per-(guild,deck) publication sequence number
	// assigned at provisioning. The Discord thread title renders this
	// ("Question #001") so the visible numbering is decoupled from the
	// question's queue position and remains monotonic regardless of which
	// selection strategy (queue order vs random) chose the question.
	PublishOrdinal             int64
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

// QOTDSurfaceRecord maps a guild deck to its Discord surface: the channel and
// the question-list thread used to render the deck's published questions.
type QOTDSurfaceRecord struct {
	ID                   int64
	GuildID              string
	DeckID               string
	ChannelID            string
	QuestionListThreadID string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// QOTDAnswerMessageRecord is a stored answer posted against an official post.
// State holds a qotd.AnswerRecordState value; ClosedAt and ArchivedAt are nil
// until the answer surface closes or is archived.
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
