//go:build integration

package qotd

import (
	"context"
	"testing"
	"time"
)

// TestServiceReplaceCurrentPublish tests the happy path and validates the states
// when a user requests to replace the current QOTD.
func TestServiceReplaceCurrentPublish(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)
	fixedNow := time.Date(2026, 6, 12, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	// Question 1 (will be the first published)
	q1, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "First Question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(q1) failed: %v", err)
	}

	// Question 2 (will replace the first)
	q2, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Replacement Question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(q2) failed: %v", err)
	}

	// Publish Q1
	firstResult, err := service.PublishNow(context.Background(), "g1")
	if err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}

	// Replace it
	replaceResult, err := service.ReplaceCurrentPublish(context.Background(), "g1")
	if err != nil {
		t.Fatalf("ReplaceCurrentPublish() failed: %v", err)
	}

	if replaceResult.Question.ID != q2.ID {
		t.Fatalf("Expected replacement to use q2, got %d", replaceResult.Question.ID)
	}

	// Check that q1 was completely purged from DB
	_, err = store.GetQOTDQuestion(context.Background(), "g1", q1.ID)
	if err == nil {
		t.Fatalf("Expected old question to be deleted, but it was found")
	}

	// Check that the old official post was purged
	_, err = store.GetQOTDOfficialPostByID(context.Background(), firstResult.OfficialPost.ID)
	if err == nil {
		t.Fatalf("Expected old official post to be deleted, but it was found")
	}

	// Check that the new post inherited the ConsumeAutomaticSlot if it applied
	if !replaceResult.OfficialPost.ConsumeAutomaticSlot {
		t.Fatalf("Expected replacement post to consume the automatic slot just like the original manual publish")
	}
}

// TestServiceReplaceCurrentPublishFailsIfNoQuestions ensures we don't accidentally
// delete the active thread if there are no available replacements.
func TestServiceReplaceCurrentPublishFailsIfNoQuestions(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)
	fixedNow := time.Date(2026, 6, 12, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	q1, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "First Question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(q1) failed: %v", err)
	}

	// Publish Q1 (exhausting the ready queue)
	firstResult, err := service.PublishNow(context.Background(), "g1")
	if err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}

	// Try replacing it when there are no more questions available
	_, err = service.ReplaceCurrentPublish(context.Background(), "g1")
	if err != ErrNoQuestionsAvailable {
		t.Fatalf("Expected ErrNoQuestionsAvailable, got %v", err)
	}

	// Ensure Q1 was NOT deleted
	_, err = store.GetQOTDQuestion(context.Background(), "g1", q1.ID)
	if err != nil {
		t.Fatalf("Expected old question to NOT be deleted when replacement fails early")
	}

	_, err = store.GetQOTDOfficialPostByID(context.Background(), firstResult.OfficialPost.ID)
	if err != nil {
		t.Fatalf("Expected old official post to NOT be deleted when replacement fails early")
	}
}
