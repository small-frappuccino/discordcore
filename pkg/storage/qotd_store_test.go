//go:build integration

package storage

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"
)

func TestQOTDTablesInitialized(t *testing.T) {
	store := newTempStore(t)

	required := []string{
		"qotd_questions",
		"qotd_official_posts",
		"qotd_forum_surfaces",
		"qotd_answer_messages",
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

	legacyTables := []string{"qotd_reply_threads", "qotd_thread_archives", "qotd_message_archives"}
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

	var publishedOnceExists bool
	if err := store.db.QueryRow(
		`SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'qotd_questions'
			  AND column_name = 'published_once_at'
		)`,
	).Scan(&publishedOnceExists); err != nil {
		t.Fatalf("query qotd_questions.published_once_at existence: %v", err)
	}
	if !publishedOnceExists {
		t.Fatal("expected qotd_questions.published_once_at to exist")
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
	reserved, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", publishDate, QOTDQuestionSelectorQueue)
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

func TestReserveNextQOTDQuestionSkipsPublishedOnceQuestion(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()
	publishedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)

	first, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Already published question",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	first.PublishedOnceAt = &publishedAt
	if first, err = store.UpdateQOTDQuestion(ctx, *first); err != nil {
		t.Fatalf("UpdateQOTDQuestion(first) failed: %v", err)
	}

	second, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Still publishable question",
		Status:        "ready",
		QueuePosition: 2,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	reserved, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", publishDate, QOTDQuestionSelectorQueue)
	if err != nil {
		t.Fatalf("ReserveNextQOTDQuestion() failed: %v", err)
	}
	if reserved == nil || reserved.ID != second.ID {
		t.Fatalf("expected scheduled reservation to skip already-published question, got %+v", reserved)
	}
}

func TestReserveNextReadyQOTDQuestionSkipsPublishedOnceQuestion(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()
	publishedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)

	first, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Already published question",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(first) failed: %v", err)
	}
	first.PublishedOnceAt = &publishedAt
	if first, err = store.UpdateQOTDQuestion(ctx, *first); err != nil {
		t.Fatalf("UpdateQOTDQuestion(first) failed: %v", err)
	}

	second, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Still publishable question",
		Status:        "ready",
		QueuePosition: 2,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(second) failed: %v", err)
	}

	reserved, err := store.ReserveNextReadyQOTDQuestion(ctx, "g1", "default", QOTDQuestionSelectorQueue)
	if err != nil {
		t.Fatalf("ReserveNextReadyQOTDQuestion() failed: %v", err)
	}
	if reserved == nil || reserved.ID != second.ID {
		t.Fatalf("expected manual reservation to skip already-published question, got %+v", reserved)
	}
}

// TestReserveNextReadyQOTDQuestionRandomCoversAllReadyQuestions exercises
// random selection by reserving repeatedly until the pool is exhausted. Any
// eligible question must eventually be picked, otherwise the random selector
// is silently degenerating to a deterministic order.
func TestReserveNextReadyQOTDQuestionRandomCoversAllReadyQuestions(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	const totalQuestions = 5
	createdIDs := make(map[int64]struct{}, totalQuestions)
	for i := 0; i < totalQuestions; i++ {
		question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
			GuildID:       "g1",
			DeckID:        "default",
			Body:          fmt.Sprintf("Question %d", i+1),
			Status:        "ready",
			QueuePosition: int64(i + 1),
		})
		if err != nil {
			t.Fatalf("CreateQOTDQuestion(%d) failed: %v", i+1, err)
		}
		createdIDs[question.ID] = struct{}{}
	}

	pickedIDs := make(map[int64]struct{}, totalQuestions)
	for i := 0; i < totalQuestions; i++ {
		picked, err := store.ReserveNextReadyQOTDQuestion(ctx, "g1", "default", QOTDQuestionSelectorRandom)
		if err != nil {
			t.Fatalf("ReserveNextReadyQOTDQuestion(random, iteration %d) failed: %v", i, err)
		}
		if picked == nil {
			t.Fatalf("expected random reservation to find a ready question on iteration %d", i)
		}
		if _, known := createdIDs[picked.ID]; !known {
			t.Fatalf("random reservation returned a question id not in the created set: %d", picked.ID)
		}
		if _, dup := pickedIDs[picked.ID]; dup {
			t.Fatalf("random reservation returned the same question id twice: %d", picked.ID)
		}
		pickedIDs[picked.ID] = struct{}{}
	}

	if len(pickedIDs) != totalQuestions {
		t.Fatalf("expected random reservation to eventually cover every ready question, got %d/%d", len(pickedIDs), totalQuestions)
	}

	// With every ready question now reserved, the random selector must
	// degrade to nil (no eligible rows left) rather than ignore the WHERE
	// clause.
	exhausted, err := store.ReserveNextReadyQOTDQuestion(ctx, "g1", "default", QOTDQuestionSelectorRandom)
	if err != nil {
		t.Fatalf("ReserveNextReadyQOTDQuestion(random, exhausted) failed: %v", err)
	}
	if exhausted != nil {
		t.Fatalf("expected no question available after pool was exhausted, got %+v", exhausted)
	}
}

// TestCreateQOTDOfficialPostProvisioningAssignsMonotonicPublishOrdinal is the
// load-bearing invariant for the visible thread numbering: each provisioning
// insert must increment publish_ordinal per (guild_id, deck_id), never
// recycling a value, even across decks within the same guild.
func TestCreateQOTDOfficialPostProvisioningAssignsMonotonicPublishOrdinal(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	// The scheduled publish date unique index is per (guild_id,
	// publish_date_utc), so tests that exercise both decks within one
	// guild must hand out distinct days. We allocate from a monotonically
	// increasing counter and let each deck take whatever slot comes next.
	dayCounter := 0
	makePost := func(deckID string) *QOTDOfficialPostRecord {
		dayCounter++
		question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
			GuildID:       "g1",
			DeckID:        deckID,
			Body:          fmt.Sprintf("Question %s/%d", deckID, dayCounter),
			Status:        "ready",
			QueuePosition: int64(dayCounter),
		})
		if err != nil {
			t.Fatalf("CreateQOTDQuestion(%s/%d) failed: %v", deckID, dayCounter, err)
		}
		publishDate := time.Date(2026, 5, dayCounter, 0, 0, 0, 0, time.UTC)
		post, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
			GuildID:              "g1",
			DeckID:               deckID,
			DeckNameSnapshot:     deckID,
			QuestionID:           question.ID,
			PublishMode:          "scheduled",
			ConsumeAutomaticSlot: true,
			PublishDateUTC:       publishDate,
			State:                "provisioning",
			ChannelID:            "123456789012345678",
			QuestionTextSnapshot: question.Body,
			GraceUntil:           publishDate.Add(24 * time.Hour),
			ArchiveAt:            publishDate.Add(48 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateQOTDOfficialPostProvisioning(%s/%d) failed: %v", deckID, dayCounter, err)
		}
		return post
	}

	deckADay1 := makePost("deck-a")
	deckADay2 := makePost("deck-a")
	deckBDay1 := makePost("deck-b")
	deckADay3 := makePost("deck-a")
	deckBDay2 := makePost("deck-b")

	if deckADay1.PublishOrdinal != 1 {
		t.Fatalf("expected deck-a first publish ordinal=1, got %d", deckADay1.PublishOrdinal)
	}
	if deckADay2.PublishOrdinal != 2 {
		t.Fatalf("expected deck-a second publish ordinal=2, got %d", deckADay2.PublishOrdinal)
	}
	if deckADay3.PublishOrdinal != 3 {
		t.Fatalf("expected deck-a third publish ordinal=3, got %d", deckADay3.PublishOrdinal)
	}
	if deckBDay1.PublishOrdinal != 1 {
		t.Fatalf("expected deck-b first publish ordinal=1 (sequence is per-deck), got %d", deckBDay1.PublishOrdinal)
	}
	if deckBDay2.PublishOrdinal != 2 {
		t.Fatalf("expected deck-b second publish ordinal=2, got %d", deckBDay2.PublishOrdinal)
	}
}

// TestCreateQOTDOfficialPostProvisioningOrdinalSurvivesUpdates locks down
// the contract that the publish ordinal is assigned exactly once at
// provisioning and is not mutated by subsequent state transitions
// (FinalizeQOTDOfficialPost, UpdateQOTDOfficialPostState,
// UpdateQOTDOfficialPostProgress). Resume flows depend on this stability
// because they re-derive the visible thread name from the persisted ordinal.
func TestCreateQOTDOfficialPostProvisioningOrdinalSurvivesUpdates(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()
	publishDate := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)

	question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Stable ordinal question",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}

	post, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		ConsumeAutomaticSlot: true,
		PublishDateUTC:       publishDate,
		State:                "provisioning",
		ChannelID:            "123456789012345678",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           publishDate.Add(24 * time.Hour),
		ArchiveAt:            publishDate.Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	originalOrdinal := post.PublishOrdinal
	if originalOrdinal != 1 {
		t.Fatalf("expected first provisioning to receive ordinal=1, got %d", originalOrdinal)
	}

	// Each lifecycle write below has historically refreshed every
	// returnable column. We assert ordinal stays put through all of them.
	progressed, err := store.UpdateQOTDOfficialPostProgress(ctx, post.ID, QOTDOfficialPostRecord{
		QuestionListThreadID:       "qlist-thread",
		QuestionListEntryMessageID: "qlist-entry",
		DiscordThreadID:            "thread-1",
		DiscordStarterMessageID:    "starter-1",
		AnswerChannelID:            "thread-1",
	})
	if err != nil {
		t.Fatalf("UpdateQOTDOfficialPostProgress() failed: %v", err)
	}
	if progressed.PublishOrdinal != originalOrdinal {
		t.Fatalf("UpdateQOTDOfficialPostProgress mutated publish_ordinal: %d -> %d", originalOrdinal, progressed.PublishOrdinal)
	}

	finalized, err := store.FinalizeQOTDOfficialPost(ctx, FinalizeQOTDOfficialPostParams{
		ID:                         post.ID,
		QuestionListThreadID:       "qlist-thread",
		QuestionListEntryMessageID: "qlist-entry",
		DiscordThreadID:            "thread-1",
		StarterMessageID:           "starter-1",
		AnswerChannelID:            "thread-1",
		PublishedAt:                publishDate.Add(13 * time.Hour),
	})
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if finalized.PublishOrdinal != originalOrdinal {
		t.Fatalf("FinalizeQOTDOfficialPost mutated publish_ordinal: %d -> %d", originalOrdinal, finalized.PublishOrdinal)
	}

	stateUpdated, err := store.UpdateQOTDOfficialPostState(ctx, post.ID, "current", nil, nil)
	if err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState() failed: %v", err)
	}
	if stateUpdated.PublishOrdinal != originalOrdinal {
		t.Fatalf("UpdateQOTDOfficialPostState mutated publish_ordinal: %d -> %d", originalOrdinal, stateUpdated.PublishOrdinal)
	}

	reread, err := store.GetQOTDOfficialPostByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByID() failed: %v", err)
	}
	if reread == nil || reread.PublishOrdinal != originalOrdinal {
		t.Fatalf("GetQOTDOfficialPostByID returned different ordinal: want %d, got %+v", originalOrdinal, reread)
	}
}

// TestCreateQOTDOfficialPostProvisioningOrdinalSharedAcrossPublishModes
// guards the invariant that scheduled and manual publishes draw from the
// same per-deck sequence. Otherwise the visible thread numbering would
// reset whenever an admin used /qotd-publish (manual) instead of the
// scheduler.
func TestCreateQOTDOfficialPostProvisioningOrdinalSharedAcrossPublishModes(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	makePost := func(mode string, day int) *QOTDOfficialPostRecord {
		question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
			GuildID:       "g1",
			DeckID:        "default",
			Body:          fmt.Sprintf("Question %s/%d", mode, day),
			Status:        "ready",
			QueuePosition: int64(day),
		})
		if err != nil {
			t.Fatalf("CreateQOTDQuestion(%s/%d) failed: %v", mode, day, err)
		}
		publishDate := time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC)
		post, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
			GuildID:              "g1",
			DeckID:               "default",
			DeckNameSnapshot:     "Default",
			QuestionID:           question.ID,
			PublishMode:          mode,
			ConsumeAutomaticSlot: mode == "scheduled",
			PublishDateUTC:       publishDate,
			State:                "provisioning",
			ChannelID:            "123456789012345678",
			QuestionTextSnapshot: question.Body,
			GraceUntil:           publishDate.Add(24 * time.Hour),
			ArchiveAt:            publishDate.Add(48 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateQOTDOfficialPostProvisioning(%s/%d) failed: %v", mode, day, err)
		}
		return post
	}

	scheduled := makePost("scheduled", 1)
	manual := makePost("manual", 2)
	scheduledNext := makePost("scheduled", 3)

	if scheduled.PublishOrdinal != 1 {
		t.Fatalf("expected first scheduled publish ordinal=1, got %d", scheduled.PublishOrdinal)
	}
	if manual.PublishOrdinal != 2 {
		t.Fatalf("expected manual publish to take the next ordinal=2 (shared sequence), got %d", manual.PublishOrdinal)
	}
	if scheduledNext.PublishOrdinal != 3 {
		t.Fatalf("expected scheduled publish after manual to take ordinal=3, got %d", scheduledNext.PublishOrdinal)
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

func TestGetQOTDOfficialPostByDatePrefersPublishedPostAcrossModes(t *testing.T) {
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
	publishedAt := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)

	manual, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
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
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(manual) failed: %v", err)
	}
	manual, err = store.FinalizeQOTDOfficialPost(ctx, FinalizeQOTDOfficialPostParams{
		ID:                         manual.ID,
		QuestionListThreadID:       "questions-list-thread",
		QuestionListEntryMessageID: "questions-list-entry-manual",
		DiscordThreadID:            "manual-thread",
		StarterMessageID:           "manual-message",
		AnswerChannelID:            "manual-thread",
		PublishedAt:                publishedAt,
	})
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost(manual) failed: %v", err)
	}

	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "provisioning",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(scheduled) failed: %v", err)
	}

	record, err := store.GetQOTDOfficialPostByDate(ctx, "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if record == nil || record.ID != manual.ID {
		t.Fatalf("expected published manual post to win the day lookup, got %+v", record)
	}
	if record.PublishMode != "manual" || record.PublishedAt == nil {
		t.Fatalf("expected published manual post from the same day, got %+v", record)
	}
}

// TestGetQOTDOfficialPostByDateRoundTrip writes a provisioned official post and
// reads it back by (guild, publish date), exercising the scheduled-publish
// lookup path end-to-end against Postgres. It is the behavioral counterpart to
// the static placeholder guard in query_placeholders_test.go: this query
// previously used "?" binds and failed under the pgx driver with SQLSTATE 42601
// ("syntax error at or near \"AND\"").
func TestGetQOTDOfficialPostByDateRoundTrip(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Round-trip question",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}

	publishDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	created, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "provisioning",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	got, err := store.GetQOTDOfficialPostByDate(ctx, "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetQOTDOfficialPostByDate() returned no record for the inserted publish date")
	}
	if got.ID != created.ID {
		t.Fatalf("GetQOTDOfficialPostByDate() returned id %d, want %d", got.ID, created.ID)
	}
	if got.GuildID != "g1" || got.DeckID != "default" || !got.PublishDateUTC.Equal(created.PublishDateUTC) {
		t.Fatalf("GetQOTDOfficialPostByDate() returned mismatched record: %+v", got)
	}

	// A date with no official post must miss cleanly (nil, nil) rather than error.
	missing, err := store.GetQOTDOfficialPostByDate(ctx, "g1", publishDate.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(unused date) failed: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected no record for an unused date, got %+v", missing)
	}
}

func TestGetScheduledQOTDOfficialPostByDateIgnoresManualPost(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	scheduledQuestion, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Scheduled question",
		Status:  "reserved",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(scheduled) failed: %v", err)
	}
	manualQuestion, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Manual question",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(manual) failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)
	publishedAt := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)

	manual, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           manualQuestion.ID,
		PublishMode:          "manual",
		PublishDateUTC:       publishDate,
		State:                "current",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: manualQuestion.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(manual) failed: %v", err)
	}
	if _, err := store.FinalizeQOTDOfficialPost(ctx, FinalizeQOTDOfficialPostParams{
		ID:                         manual.ID,
		QuestionListThreadID:       "questions-list-thread",
		QuestionListEntryMessageID: "questions-list-entry-manual",
		DiscordThreadID:            "manual-thread",
		StarterMessageID:           "manual-message",
		AnswerChannelID:            "manual-thread",
		PublishedAt:                publishedAt,
	}); err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost(manual) failed: %v", err)
	}

	scheduled, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           scheduledQuestion.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "provisioning",
		ChannelID:            "forum-1",
		QuestionTextSnapshot: scheduledQuestion.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(scheduled) failed: %v", err)
	}

	record, err := store.GetScheduledQOTDOfficialPostByDate(ctx, "g1", publishDate)
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate() failed: %v", err)
	}
	if record == nil || record.ID != scheduled.ID {
		t.Fatalf("expected scheduled lookup to ignore the manual post, got %+v", record)
	}
	if record.PublishMode != "scheduled" || record.QuestionID != scheduledQuestion.ID {
		t.Fatalf("expected scheduled lookup to return the scheduled slot record, got %+v", record)
	}
	if record.PublishedAt != nil {
		t.Fatalf("expected scheduled lookup to return the scheduled provisioning record before finalize, got %+v", record)
	}
}

func TestReclaimOrphanReservedQOTDQuestionsReleasesPastReservationsWithoutPosts(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	pastDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	todayUTC := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	orphan, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Stuck after crash",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(orphan) failed: %v", err)
	}
	if _, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", pastDate, QOTDQuestionSelectorQueue); err != nil {
		t.Fatalf("ReserveNextQOTDQuestion(orphan) failed: %v", err)
	}

	freed, err := store.ReclaimOrphanReservedQOTDQuestions(ctx, "g1", todayUTC)
	if err != nil {
		t.Fatalf("ReclaimOrphanReservedQOTDQuestions() failed: %v", err)
	}
	if len(freed) != 1 || freed[0] != orphan.ID {
		t.Fatalf("expected orphan reservation to be released, got %+v", freed)
	}

	restored, err := store.GetQOTDQuestion(ctx, "g1", orphan.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(orphan) failed: %v", err)
	}
	if restored == nil || restored.Status != "ready" {
		t.Fatalf("expected orphan to be restored to ready, got %+v", restored)
	}
	if restored.ScheduledForDateUTC != nil {
		t.Fatalf("expected scheduled_for_date_utc to be cleared on the orphan, got %+v", restored.ScheduledForDateUTC)
	}
}

func TestReclaimOrphanReservedQOTDQuestionsKeepsTodayReservation(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	todayUTC := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Active reservation",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}
	if _, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", todayUTC, QOTDQuestionSelectorQueue); err != nil {
		t.Fatalf("ReserveNextQOTDQuestion() failed: %v", err)
	}

	freed, err := store.ReclaimOrphanReservedQOTDQuestions(ctx, "g1", todayUTC)
	if err != nil {
		t.Fatalf("ReclaimOrphanReservedQOTDQuestions() failed: %v", err)
	}
	if len(freed) != 0 {
		t.Fatalf("expected today's reservation to stay in place (publish may be in flight), got freed=%+v", freed)
	}

	stored, err := store.GetQOTDQuestion(ctx, "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if stored == nil || stored.Status != "reserved" {
		t.Fatalf("expected today's reservation to remain reserved, got %+v", stored)
	}
}

func TestReclaimOrphanReservedQOTDQuestionsLeavesQuestionsWithLinkedPosts(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	pastDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	todayUTC := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID:       "g1",
		DeckID:        "default",
		Body:          "Linked to a real post",
		Status:        "ready",
		QueuePosition: 1,
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}
	reserved, err := store.ReserveNextQOTDQuestion(ctx, "g1", "default", pastDate, QOTDQuestionSelectorQueue)
	if err != nil || reserved == nil {
		t.Fatalf("ReserveNextQOTDQuestion() failed: %v", err)
	}
	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       pastDate,
		State:                "provisioning",
		ChannelID:            "channel-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           pastDate.Add(24 * time.Hour),
		ArchiveAt:            pastDate.Add(48 * time.Hour),
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	freed, err := store.ReclaimOrphanReservedQOTDQuestions(ctx, "g1", todayUTC)
	if err != nil {
		t.Fatalf("ReclaimOrphanReservedQOTDQuestions() failed: %v", err)
	}
	if len(freed) != 0 {
		t.Fatalf("expected reservations linked to a publish record to be left alone, got %+v", freed)
	}

	stored, err := store.GetQOTDQuestion(ctx, "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if stored == nil || stored.Status != "reserved" {
		t.Fatalf("expected reservation linked to a post to stay reserved, got %+v", stored)
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
	finalized, err := store.FinalizeQOTDOfficialPost(ctx, FinalizeQOTDOfficialPostParams{
		ID:                         official.ID,
		QuestionListThreadID:       "questions-list-thread",
		QuestionListEntryMessageID: "list-entry-1",
		DiscordThreadID:            "official-thread-1",
		StarterMessageID:           "starter-message-1",
		AnswerChannelID:            "official-thread-1",
		PublishedAt:                finalizedAt,
	})
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

func TestDeleteQOTDOfficialPostsByDeckRemovesOnlyMatchingDeck(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	deckAQuestion, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "deck-a",
		Body:    "Deck A question",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(deck-a) failed: %v", err)
	}
	deckBQuestion, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "deck-b",
		Body:    "Deck B question",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(deck-b) failed: %v", err)
	}

	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "deck-a",
		DeckNameSnapshot:     "Deck A",
		QuestionID:           deckAQuestion.ID,
		PublishMode:          "manual",
		PublishDateUTC:       time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		State:                "current",
		ChannelID:            "question-channel-a",
		QuestionTextSnapshot: deckAQuestion.Body,
		GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(deck-a) failed: %v", err)
	}
	deckBOfficial, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "deck-b",
		DeckNameSnapshot:     "Deck B",
		QuestionID:           deckBQuestion.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC),
		State:                "provisioning",
		ChannelID:            "question-channel-b",
		QuestionTextSnapshot: deckBQuestion.Body,
		GraceUntil:           time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 6, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(deck-b) failed: %v", err)
	}

	deleted, err := store.DeleteQOTDOfficialPostsByDeck(ctx, "g1", "deck-a")
	if err != nil {
		t.Fatalf("DeleteQOTDOfficialPostsByDeck() failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected one deck-a official post to be deleted, got %d", deleted)
	}

	deletedRecord, err := store.GetQOTDOfficialPostByDate(ctx, "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(deck-a) failed: %v", err)
	}
	if deletedRecord != nil {
		t.Fatalf("expected deck-a official post to be removed, got %+v", deletedRecord)
	}

	preservedRecord, err := store.GetQOTDOfficialPostByDate(ctx, "g1", time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(deck-b) failed: %v", err)
	}
	if preservedRecord == nil || preservedRecord.ID != deckBOfficial.ID {
		t.Fatalf("expected deck-b official post to remain, got %+v", preservedRecord)
	}
	if preservedRecord.DeckID != "deck-b" {
		t.Fatalf("expected only deck-a official posts to be deleted, got %+v", preservedRecord)
	}
}

func TestListQOTDOfficialPostsByDateReturnsAllMatchingRecords(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	questionA, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question A",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(questionA) failed: %v", err)
	}
	questionB, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question B",
		Status:  "reserved",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(questionB) failed: %v", err)
	}

	publishDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 5, 8, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 5, 9, 12, 43, 0, 0, time.UTC)

	first, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           questionA.ID,
		PublishMode:          "manual",
		PublishDateUTC:       publishDate,
		State:                "current",
		ChannelID:            "question-channel",
		QuestionTextSnapshot: questionA.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(first) failed: %v", err)
	}
	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           questionB.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDate,
		State:                "failed",
		ChannelID:            "question-channel",
		QuestionTextSnapshot: questionB.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(second) failed: %v", err)
	}

	postsIter, err := store.ListQOTDOfficialPostsByDate(ctx, "g1", publishDate)
	if err != nil {
		t.Fatalf("ListQOTDOfficialPostsByDate() failed: %v", err)
	}
	posts := slices.Collect(postsIter)
	if len(posts) != 2 {
		t.Fatalf("expected two official posts for the same date, got %+v", posts)
	}
	if posts[0].ID == 0 || posts[1].ID == 0 {
		t.Fatalf("expected persisted official post ids, got %+v", posts)
	}
	if posts[0].ID == posts[1].ID {
		t.Fatalf("expected distinct official post records, got %+v", posts)
	}
	if posts[0].ID != first.ID && posts[1].ID != first.ID {
		t.Fatalf("expected first post to be present in list, got %+v", posts)
	}
}

func TestDeleteQOTDOfficialPostByIDRemovesOnlyTargetRecord(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	questionA, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question A",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(questionA) failed: %v", err)
	}
	questionB, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question B",
		Status:  "used",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(questionB) failed: %v", err)
	}

	publishDateA := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	publishDateB := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 5, 9, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 5, 10, 12, 43, 0, 0, time.UTC)

	target, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           questionA.ID,
		PublishMode:          "scheduled",
		PublishDateUTC:       publishDateA,
		State:                "abandoned",
		ChannelID:            "question-channel",
		QuestionTextSnapshot: questionA.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(target) failed: %v", err)
	}
	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           questionB.ID,
		PublishMode:          "manual",
		PublishDateUTC:       publishDateB,
		State:                "current",
		ChannelID:            "question-channel",
		QuestionTextSnapshot: questionB.Body,
		GraceUntil:           graceUntil,
		ArchiveAt:            archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(other) failed: %v", err)
	}

	if err := store.DeleteQOTDOfficialPostByID(ctx, target.ID); err != nil {
		t.Fatalf("DeleteQOTDOfficialPostByID() failed: %v", err)
	}

	deleted, err := store.GetQOTDOfficialPostByID(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByID(target) failed: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected target official post to be deleted, got %+v", deleted)
	}

	preserved, err := store.GetQOTDOfficialPostByDate(ctx, "g1", publishDateB)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(other) failed: %v", err)
	}
	if preserved == nil || preserved.QuestionID != questionB.ID {
		t.Fatalf("expected non-target official post to remain, got %+v", preserved)
	}
}
