package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestQOTDTablesInitialized(t *testing.T) {
	store := newTempStore(t)

	required := []string{
		"qotd_questions",
		"qotd_official_posts",
		"qotd_forum_surfaces",
		"qotd_answer_messages",
		"qotd_thread_archives",
		"qotd_message_archives",
		"qotd_collected_questions",
	}
	for _, tableName := range required {
		var exists bool
		if err := store.db.QueryRow(
			`SELECT EXISTS(
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = $1
			)`,
			tableName,
		).Scan(&exists); err != nil {
			t.Fatalf("query table %s existence: %v", tableName, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist", tableName)
		}
	}

	legacyTables := []string{"qotd_reply_threads"}
	for _, tableName := range legacyTables {
		var exists bool
		if err := store.db.QueryRow(
			`SELECT EXISTS(
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = $1
			)`,
			tableName,
		).Scan(&exists); err != nil {
			t.Fatalf("query legacy table %s existence: %v", tableName, err)
		}
		if exists {
			t.Fatalf("expected legacy table %s to be removed", tableName)
		}
	}

	legacyColumns := []struct {
		tableName  string
		columnName string
	}{
		{tableName: "qotd_official_posts", columnName: "response_channel_id_snapshot"},
		{tableName: "qotd_official_posts", columnName: "is_pinned"},
		{tableName: "qotd_thread_archives", columnName: "reply_thread_id"},
	}
	for _, legacyColumn := range legacyColumns {
		var exists bool
		if err := store.db.QueryRow(
			`SELECT EXISTS(
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = $1
				  AND column_name = $2
			)`,
			legacyColumn.tableName,
			legacyColumn.columnName,
		).Scan(&exists); err != nil {
			t.Fatalf("query legacy column %s.%s existence: %v", legacyColumn.tableName, legacyColumn.columnName, err)
		}
		if exists {
			t.Fatalf("expected legacy column %s.%s to be removed", legacyColumn.tableName, legacyColumn.columnName)
		}
	}
}

func TestInitResetsQOTDQuestionSequenceWhenTableEmpty(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	first, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "First question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	if first.ID != 1 {
		t.Fatalf("expected fresh isolated database to start question IDs at 1, got %d", first.ID)
	}

	if err := store.DeleteQOTDQuestion(ctx, "g1", first.ID); err != nil {
		t.Fatalf("DeleteQOTDQuestion() failed: %v", err)
	}
	if err := store.Init(); err != nil {
		t.Fatalf("Init() after emptying qotd_questions failed: %v", err)
	}

	second, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Second question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}
	if second.ID != 1 {
		t.Fatalf("expected question ID sequence to reset to 1 after Init() on an empty table, got %d", second.ID)
	}
}

func TestDeleteQOTDQuestionReindexesDisplayIDs(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	first, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "First question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	second, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Second question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}
	if first.DisplayID != 1 || second.DisplayID != 2 {
		t.Fatalf("expected sequential visible ids after create, got first=%d second=%d", first.DisplayID, second.DisplayID)
	}

	if err := store.DeleteQOTDQuestion(ctx, "g1", first.ID); err != nil {
		t.Fatalf("DeleteQOTDQuestion() failed: %v", err)
	}

	remaining, err := store.ListQOTDQuestions(ctx, "g1", "default")
	if err != nil {
		t.Fatalf("ListQOTDQuestions() failed: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected one remaining question, got %+v", remaining)
	}
	if remaining[0].ID != second.ID {
		t.Fatalf("expected second question to remain, got %+v", remaining[0])
	}
	if remaining[0].DisplayID != 1 {
		t.Fatalf("expected remaining question display id to be renumbered to 1, got %d", remaining[0].DisplayID)
	}

	third, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Third question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(third) failed: %v", err)
	}
	if third.DisplayID != 2 {
		t.Fatalf("expected next visible id to continue from 2 after reindex, got %d", third.DisplayID)
	}
}

func TestReserveNextQOTDQuestionUsesQueueOrder(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	if _, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Second question",
		Status:        "ready",
		QueuePosition: 2,
	}); err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}
	first, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "First question",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	reserved, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", publishDate)
	if err != nil {
		t.Fatalf("ReserveNextQOTDQuestion() failed: %v", err)
	}
	if reserved == nil {
		t.Fatal("expected a reserved question record")
	}
	if reserved.ID != first.ID {
		t.Fatalf("expected lowest queue position to be reserved first, got id=%d want=%d", reserved.ID, first.ID)
	}
	if reserved.Status != "reserved" {
		t.Fatalf("expected reserved status, got %q", reserved.Status)
	}
	if reserved.ScheduledForDateUTC == nil || !reserved.ScheduledForDateUTC.Equal(publishDate) {
		t.Fatalf("expected scheduled publish date %s, got %+v", publishDate.Format(time.RFC3339), reserved.ScheduledForDateUTC)
	}
}

func TestReorderQOTDQuestionsAllowsQueuePositionSwap(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	first, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "First question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	second, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Second question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}

	if err := store.ReorderQOTDQuestions(ctx, "g1", "default", []int64{second.ID, first.ID}); err != nil {
		t.Fatalf("ReorderQOTDQuestions() failed: %v", err)
	}

	questions, err := store.ListQOTDQuestions(ctx, "g1", "default")
	if err != nil {
		t.Fatalf("ListQOTDQuestions() failed: %v", err)
	}
	if len(questions) != 2 {
		t.Fatalf("expected two questions after reorder, got %+v", questions)
	}
	if questions[0].ID != second.ID || questions[0].QueuePosition != 1 || questions[0].DisplayID != 1 {
		t.Fatalf("expected second question to move into slot 1, got %+v", questions)
	}
	if questions[1].ID != first.ID || questions[1].QueuePosition != 2 || questions[1].DisplayID != 2 {
		t.Fatalf("expected first question to move into slot 2, got %+v", questions)
	}
}

func TestQOTDOfficialPostsAllowManualAndScheduledOnSameDate(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question one",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	second, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question two",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)

	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "current",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(scheduled) failed: %v", err)
	}

	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           second.ID,
		PublishMode:          "manual",
		PublishDateUTC:       publishDate,
		State:                "current",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: second.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(manual) failed: %v", err)
	}

	_, err = store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           second.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "current",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: second.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err == nil {
		t.Fatal("expected duplicate scheduled publish date to remain unique")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr == nil {
		t.Fatalf("expected pg error for duplicate scheduled publish date, got %T %v", err, err)
	}
	if pgErr.Code != "23505" {
		t.Fatalf("expected SQLSTATE 23505 for duplicate scheduled publish date, got %q", pgErr.Code)
	}
	if pgErr.ConstraintName != "idx_qotd_official_posts_scheduled_publish_date" {
		t.Fatalf("expected scheduled publish date constraint, got %q", pgErr.ConstraintName)
	}
}

func TestQOTDOfficialPostProgressAndPendingRecoveryLifecycle(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question one",
		Status:  "reserved",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}

	official, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		State:                "provisioning",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	progress, err := store.UpdateQOTDOfficialPostProgress(ctx, official.ID, QOTDOfficialPostRecord{
		QuestionListThreadID:    "questions-list-thread",
		DiscordThreadID:         "official-thread-1",
		DiscordStarterMessageID: "starter-message-1",
		AnswerChannelID:         "official-thread-1",
	})
	if err != nil {
		t.Fatalf("UpdateQOTDOfficialPostProgress() failed: %v", err)
	}
	if progress.QuestionListThreadID != "questions-list-thread" || progress.DiscordThreadID != "official-thread-1" || progress.DiscordStarterMessageID != "starter-message-1" {
		t.Fatalf("expected progress update to persist partial artifacts, got %+v", progress)
	}
	if progress.PublishedAt != nil {
		t.Fatalf("expected progress update to keep published_at unset until finalize, got %+v", progress)
	}

	if _, err := store.UpdateQOTDOfficialPostState(ctx, official.ID, "failed", nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState(failed) failed: %v", err)
	}

	pending, err := store.ListQOTDOfficialPostsPendingRecovery(ctx, "g1")
	if err != nil {
		t.Fatalf("ListQOTDOfficialPostsPendingRecovery() failed: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != official.ID {
		t.Fatalf("expected failed provisioning record to be listed for recovery, got %+v", pending)
	}

	finalizedAt := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	finalized, err := store.FinalizeQOTDOfficialPost(ctx, official.ID, "questions-list-thread", "list-entry-1", "official-thread-1", "starter-message-1", "official-thread-1", finalizedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if finalized.PublishedAt == nil || !finalized.PublishedAt.Equal(finalizedAt) {
		t.Fatalf("expected finalize to persist published_at, got %+v", finalized)
	}

	if _, err := store.UpdateQOTDOfficialPostState(ctx, official.ID, "current", nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState(current) failed: %v", err)
	}
	pending, err = store.ListQOTDOfficialPostsPendingRecovery(ctx, "g1")
	if err != nil {
		t.Fatalf("ListQOTDOfficialPostsPendingRecovery(after finalize) failed: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected finalized post to disappear from pending recovery list, got %+v", pending)
	}
}

func TestDeleteQOTDQuestionsByDecksPreservesOfficialPostHistory(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "deck-a",
		Body:    "Question one",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}
	official, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "deck-a",
		DeckNameSnapshot:     "Deck A",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		State:                "published",
		ChannelID:            "question-channel-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	if err := store.DeleteQOTDQuestionsByDecks(ctx, "g1", []string{"deck-a"}); err != nil {
		t.Fatalf("DeleteQOTDQuestionsByDecks() failed: %v", err)
	}

	deletedQuestion, err := store.GetQOTDQuestion(ctx, "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if deletedQuestion != nil {
		t.Fatalf("expected question to be deleted, got %+v", deletedQuestion)
	}

	questions, err := store.ListQOTDQuestions(ctx, "g1", "deck-a")
	if err != nil {
		t.Fatalf("ListQOTDQuestions() failed: %v", err)
	}
	if len(questions) != 0 {
		t.Fatalf("expected deck questions to be deleted, got %+v", questions)
	}

	preservedOfficial, err := store.GetQOTDOfficialPostByDate(ctx, "g1", official.PublishDateUTC)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if preservedOfficial == nil {
		t.Fatal("expected official post history to remain after deleting deck questions")
	}
	if preservedOfficial.QuestionID != question.ID || preservedOfficial.QuestionTextSnapshot != question.Body {
		t.Fatalf("expected official post snapshot to remain intact, got %+v", preservedOfficial)
	}
}

func TestCreateQOTDCollectedQuestionsDeduplicatesBySourceMessage(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 13, 16, 0, 0, 0, time.UTC)

	created, err := store.CreateQOTDCollectedQuestions(ctx, []QOTDCollectedQuestionRecord{
		{
			GuildID:                  "g1",
			SourceChannelID:          "channel-1",
			SourceMessageID:          "message-1",
			SourceAuthorID:           "bot-1",
			SourceAuthorNameSnapshot: "QOTD Bot",
			SourceCreatedAt:          now,
			EmbedTitle:               "Question Of The Day",
			QuestionText:             "What shipped this week?",
		},
		{
			GuildID:                  "g1",
			SourceChannelID:          "channel-1",
			SourceMessageID:          "message-1",
			SourceAuthorID:           "bot-1",
			SourceAuthorNameSnapshot: "QOTD Bot",
			SourceCreatedAt:          now,
			EmbedTitle:               "Question Of The Day",
			QuestionText:             "What shipped this week?",
		},
		{
			GuildID:                  "g1",
			SourceChannelID:          "channel-1",
			SourceMessageID:          "message-2",
			SourceAuthorID:           "bot-2",
			SourceAuthorNameSnapshot: "Other QOTD Bot",
			SourceCreatedAt:          now.Add(time.Minute),
			EmbedTitle:               "question!!",
			QuestionText:             "What are you trying next?",
		},
	})
	if err != nil {
		t.Fatalf("CreateQOTDCollectedQuestions() failed: %v", err)
	}
	if created != 2 {
		t.Fatalf("expected 2 unique collected questions, got %d", created)
	}

	total, err := store.CountQOTDCollectedQuestions(ctx, "g1")
	if err != nil {
		t.Fatalf("CountQOTDCollectedQuestions() failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total collected questions to be 2, got %d", total)
	}

	recent, err := store.ListRecentQOTDCollectedQuestions(ctx, "g1", 10)
	if err != nil {
		t.Fatalf("ListRecentQOTDCollectedQuestions() failed: %v", err)
	}
	if len(recent) != 2 || recent[0].SourceMessageID != "message-2" {
		t.Fatalf("expected most recent collected question first, got %+v", recent)
	}

	exported, err := store.ListAllQOTDCollectedQuestions(ctx, "g1")
	if err != nil {
		t.Fatalf("ListAllQOTDCollectedQuestions() failed: %v", err)
	}
	if len(exported) != 2 || exported[0].SourceMessageID != "message-1" || exported[1].SourceMessageID != "message-2" {
		t.Fatalf("expected export ordering by source_created_at asc, got %+v", exported)
	}
}
