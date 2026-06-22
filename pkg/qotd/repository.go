package qotd

import (
	"context"
	"iter"
	"time"
)

// Repository encapsulates the persistent storage logic for the QOTD domain.
// It manages complex relational operations (like atomic cross-table syncs and row-level locks)
// while providing a simple facade to domain services.
type Repository interface {
	// Question Management
	CreateQOTDQuestion(ctx context.Context, rec QuestionRecord) (*QuestionRecord, error)
	UpdateQOTDQuestion(ctx context.Context, rec QuestionRecord) (*QuestionRecord, error)
	DeleteQOTDQuestion(ctx context.Context, guildID string, questionID int64) error
	DeleteQOTDQuestionsByDecks(ctx context.Context, guildID string, deckIDs []string) error
	ListQOTDQuestions(ctx context.Context, guildID, deckID string) (iter.Seq2[QuestionRecord, error], error)
	GetQOTDQuestion(ctx context.Context, guildID string, questionID int64) (*QuestionRecord, error)
	ReorderQOTDQuestions(ctx context.Context, guildID, deckID string, orderedIDs []int64) error

	// Selection and Scheduling
	ReserveNextQOTDQuestion(ctx context.Context, guildID, deckID string, publishDateUTC time.Time, selector QuestionSelector) (*QuestionRecord, error)
	ReserveNextReadyQOTDQuestion(ctx context.Context, guildID, deckID string, selector QuestionSelector) (*QuestionRecord, error)
	ReclaimOrphanReservedQOTDQuestions(ctx context.Context, guildID string, todayUTC time.Time) iter.Seq2[int64, error]

	// Official Posts
	CreateQOTDOfficialPostProvisioning(ctx context.Context, rec OfficialPostRecord) (*OfficialPostRecord, error)
	FinalizeQOTDOfficialPost(ctx context.Context, params FinalizeOfficialPostParams) (*OfficialPostRecord, error)
	GetQOTDOfficialPostByID(ctx context.Context, id int64) (*OfficialPostRecord, error)
	GetQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (*OfficialPostRecord, error)
	ListQOTDOfficialPostsByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (iter.Seq[OfficialPostRecord], error)
	GetAutomaticSlotQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (*OfficialPostRecord, error)
	GetScheduledQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (*OfficialPostRecord, error)
	DeleteQOTDOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (int, error)
	DeleteQOTDOfficialPostByID(ctx context.Context, id int64) error
	DeleteQOTDUnpublishedOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (int, error)
	UpdateQOTDOfficialPostProgress(ctx context.Context, id int64, progress OfficialPostRecord) (*OfficialPostRecord, error)
	ListQOTDOfficialPostsPendingRecovery(ctx context.Context, guildID string) iter.Seq2[OfficialPostRecord, error]
	GetCurrentAndPreviousQOTDPosts(ctx context.Context, guildID string, now time.Time) iter.Seq2[OfficialPostRecord, error]
	ListQOTDOfficialPostsNeedingArchive(ctx context.Context, now time.Time) iter.Seq2[OfficialPostRecord, error]
	UpdateQOTDOfficialPostState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (*OfficialPostRecord, error)

	// Surfaces
	GetQOTDSurfaceByDeck(ctx context.Context, guildID, deckID string) (*SurfaceRecord, error)
	UpsertQOTDSurface(ctx context.Context, rec SurfaceRecord) (*SurfaceRecord, error)
	DeleteQOTDSurfaceByDeck(ctx context.Context, guildID, deckID string) error

	// Answer Messages
	CreateQOTDAnswerMessage(ctx context.Context, rec AnswerMessageRecord) (*AnswerMessageRecord, error)
	FinalizeQOTDAnswerMessage(ctx context.Context, id int64, discordMessageID string) (*AnswerMessageRecord, error)
	GetQOTDAnswerMessageByOfficialPostAndUser(ctx context.Context, officialPostID int64, userID string) (*AnswerMessageRecord, error)
	ListQOTDAnswerMessagesByOfficialPost(ctx context.Context, officialPostID int64) (iter.Seq2[AnswerMessageRecord, error], error)
	UpdateQOTDAnswerMessageState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (*AnswerMessageRecord, error)
}
