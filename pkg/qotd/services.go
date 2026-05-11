package qotd

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// QuestionCatalog is the narrow read/write surface for QOTD deck questions.
// Commands and HTTP routes that only manipulate questions (add, list,
// remove, recover, mark-published) depend on this instead of pulling in
// the whole publish lifecycle. Tests that exercise question CRUD can mock
// six methods instead of the ~20 the monolithic Service exposes.
type QuestionCatalog interface {
	// Settings returns the dashboard-shaped QOTD config for the guild.
	// Most command handlers need it to resolve "active deck" before
	// touching questions.
	Settings(guildID string) (files.QOTDConfig, error)

	// ListQuestions returns every question in a deck, in queue order.
	ListQuestions(ctx context.Context, guildID, deckID string) ([]storage.QOTDQuestionRecord, error)

	// CreateQuestion appends a new question to a deck.
	CreateQuestion(ctx context.Context, guildID, actorID string, mutation QuestionMutation) (*storage.QOTDQuestionRecord, error)

	// DeleteQuestion removes a mutable question (not reserved/used).
	DeleteQuestion(ctx context.Context, guildID string, questionID int64) error

	// RestoreUsedQuestion flips a used question back to ready so it can
	// be published again.
	RestoreUsedQuestion(ctx context.Context, guildID, deckID string, questionID int64) (*storage.QOTDQuestionRecord, error)

	// MarkQuestionPublished marks a ready question as already-used
	// without touching the official-post day state. Used by operators
	// who published the question outside the bot.
	MarkQuestionPublished(ctx context.Context, guildID, deckID string, questionID int64) (*storage.QOTDQuestionRecord, error)
}

// PublishCoordinator is the narrow surface for the publish state machine
// and queue inspection. Commands like /qotd publish and /qotd questions
// queue depend on this; the runtime loop has its own ReconcileCoordinator
// because the publish path it cares about is the scheduled one.
type PublishCoordinator interface {
	// GetAutomaticQueueState reports what the scheduler will do next for
	// a deck (status of the upcoming slot, the reserved/next-ready
	// questions, schedule visibility). Pure read.
	GetAutomaticQueueState(ctx context.Context, guildID, deckID string) (AutomaticQueueState, error)

	// PublishNowWithParams runs a manual publish, optionally consuming
	// the current automatic slot. Idempotent across crashes thanks to
	// the nonce + unique-constraint contract documented on
	// QOTDOfficialPostRecord.
	PublishNowWithParams(ctx context.Context, guildID string, session *discordgo.Session, params PublishNowParams) (*PublishResult, error)
}

// ReconcileCoordinator is the narrow surface the QOTD runtime loop drives.
// The pkg/discord/qotd package re-declares the same shape as
// GuildLifecycleService because pkg/qotd already imports pkg/discord/qotd
// for the Publisher type — importing back would create a cycle. The
// duplicated declaration is the documented escape hatch; both names
// describe the same contract and *Service satisfies both implicitly.
type ReconcileCoordinator interface {
	PublishScheduledIfDue(ctx context.Context, guildID string, session *discordgo.Session) (bool, error)
	ReconcileGuild(ctx context.Context, guildID string, session *discordgo.Session) error
}

// Compile-time guarantees that the monolithic *Service still satisfies the
// narrow surfaces. Callers can keep wiring *Service directly while new code
// and tests depend on the role they need.
var (
	_ QuestionCatalog     = (*Service)(nil)
	_ PublishCoordinator  = (*Service)(nil)
	_ ReconcileCoordinator = (*Service)(nil)
)
