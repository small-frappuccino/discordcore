package storage

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestQOTDTablesInitialized(t *testing.T) {
	store := newTempStore(t)

	required := []string{
		"qotd_questions",
		"qotd_official_posts",
		"qotd_reply_threads",
		"qotd_thread_archives",
		"qotd_message_archives",
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
		GuildID:            "g1",
		DeckID:             "default",
		DeckNameSnapshot:   "Default",
		QuestionID:         question.ID,
		PublishMode:        "scheduled",
		PublishDateUTC:     publishDate,
		State:              "current",
		ForumChannelID:     "forum-1",
		ResponseChannelID:  "responses-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:         graceUntil,
		ArchiveAt:          archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(scheduled) failed: %v", err)
	}

	if _, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:            "g1",
		DeckID:             "default",
		DeckNameSnapshot:   "Default",
		QuestionID:         second.ID,
		PublishMode:        "manual",
		PublishDateUTC:     publishDate,
		State:              "current",
		ForumChannelID:     "forum-1",
		ResponseChannelID:  "responses-1",
		QuestionTextSnapshot: second.Body,
		GraceUntil:         graceUntil,
		ArchiveAt:          archiveAt,
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(manual) failed: %v", err)
	}

	_, err = store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:            "g1",
		DeckID:             "default",
		DeckNameSnapshot:   "Default",
		QuestionID:         second.ID,
		PublishMode:        "scheduled",
		PublishDateUTC:     publishDate,
		State:              "current",
		ForumChannelID:     "forum-1",
		ResponseChannelID:  "responses-1",
		QuestionTextSnapshot: second.Body,
		GraceUntil:         graceUntil,
		ArchiveAt:          archiveAt,
	})
	if err == nil {
		t.Fatal("expected duplicate scheduled publish date to remain unique")
	}
	if !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "unique") {
		t.Fatalf("expected unique-constraint error for duplicate scheduled publish date, got %v", err)
	}
}

func TestQOTDReplyThreadProvisioningNonceLifecycle(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: "g1",
		DeckID:  "default",
		Body:    "Question one",
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion() failed: %v", err)
	}
	official, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:            "g1",
		DeckID:             "default",
		DeckNameSnapshot:   "Default",
		QuestionID:         question.ID,
		PublishMode:        "scheduled",
		PublishDateUTC:     time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		State:              "current",
		ForumChannelID:     "forum-1",
		ResponseChannelID:  "responses-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:         time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:          time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	replyThread, err := store.CreateQOTDReplyThreadProvisioning(ctx, QOTDReplyThreadRecord{
		GuildID:                 "g1",
		OfficialPostID:          official.ID,
		UserID:                  "user-1",
		State:                   "provisioning",
		ForumChannelID:          "forum-1",
		CreatedViaInteractionID: "interaction-1",
		ProvisioningNonce:       "nonce-1",
	})
	if err != nil {
		t.Fatalf("CreateQOTDReplyThreadProvisioning() failed: %v", err)
	}
	if replyThread.ProvisioningNonce != "nonce-1" {
		t.Fatalf("expected provisioning nonce to persist on create, got %+v", replyThread)
	}

	prepared, err := store.PrepareQOTDReplyThreadProvisioning(ctx, replyThread.ID, "forum-2", "interaction-2", "nonce-2")
	if err != nil {
		t.Fatalf("PrepareQOTDReplyThreadProvisioning() failed: %v", err)
	}
	if prepared.ProvisioningNonce != "nonce-2" || prepared.ForumChannelID != "forum-2" || prepared.CreatedViaInteractionID != "interaction-2" {
		t.Fatalf("expected provisioning refresh to replace nonce and metadata, got %+v", prepared)
	}

	pending, err := store.ListQOTDReplyThreadsPendingRecovery(ctx, "g1")
	if err != nil {
		t.Fatalf("ListQOTDReplyThreadsPendingRecovery() failed: %v", err)
	}
	if len(pending) != 1 || pending[0].ProvisioningNonce != "nonce-2" {
		t.Fatalf("expected pending recovery list to include updated nonce, got %+v", pending)
	}
}
