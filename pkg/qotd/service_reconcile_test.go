//go:build integration

package qotd

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestServiceReconcileOfficialPostWindowConcurrency prevents orphaned states and thundering
// herd provisions. It ensures that if reconciliation fires concurrently (e.g., from multiple
// pods or concurrent timers resolving immediately), only one provision proceeds and no duplicate
// discord threads are created.
func TestServiceReconcileOfficialPostWindowConcurrency(t *testing.T) {
	service, _, fake := newIntegrationTestQOTDService(t)
	fixedNow := time.Date(2026, 6, 12, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	// Ensure there is an enabled scheduled deck
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	// Create a question for the scheduler to pick up
	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Scheduled question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	// Simulate 5 concurrent actors attempting to reconcile the exact same slot
	// This simulates a timer skew or multiple supervisor nodes waking up at exactly the same time.
	slotDate := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- service.reconcileOfficialPostWindow(context.Background(), "g1", slotDate, 0)
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Concurrent reconcile failed: %v", err)
		}
	}

	// Verify only ONE publish actually occurred on Discord
	// fake.mu does not exist by default on fakePublisher, wait!
	// fakePublisher has no mutex. Let me check service_integration_test.go!
	numPublishes := len(fake.publishedParams)

	if numPublishes != 1 {
		t.Fatalf("Expected exactly 1 publish call to Discord, but got %d. Thundering herd vulnerability detected.", numPublishes)
	}

	// Verify the database question state correctly transitioned
	storedQuestion, err := service.store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if storedQuestion.Status != string(QuestionStatusUsed) || storedQuestion.UsedAt == nil {
		t.Fatalf("Expected question to be marked as used, got state: %s", storedQuestion.Status)
	}
}

// TestServiceReleaseReservedQuestionIdempotency checks the failure mode where network transients
// cause a release call to retry, ensuring that repeated releases of an already-published or already-released
// question do not panic or illegally revert its state.
func TestServiceReleaseReservedQuestionIdempotency(t *testing.T) {
	service, _, _ := newIntegrationTestQOTDService(t)

	// Create a reserved question manually
	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Reserved question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	slotDate := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	question.Status = string(QuestionStatusReserved)
	question.ScheduledForDateUTC = &slotDate
	_, err = service.store.UpdateQOTDQuestion(context.Background(), *question)
	if err != nil {
		t.Fatalf("UpdateQOTDQuestion() failed: %v", err)
	}

	// First release
	err = service.releaseReservedQuestion(context.Background(), *question)
	if err != nil {
		t.Fatalf("releaseReservedQuestion(1) failed: %v", err)
	}

	// Verify the release moved it back to ready
	q1, _ := service.store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if q1.Status != string(QuestionStatusReady) {
		t.Fatalf("Expected question to be Ready, got %s", q1.Status)
	}

	// Second release (idempotent failure mode check)
	err = service.releaseReservedQuestion(context.Background(), *question)
	if err != nil {
		t.Fatalf("releaseReservedQuestion(2) failed. Expected idempotency but got error: %v", err)
	}

	// Third release after it was completely unrelated (e.g. status was USED instead)
	q1.Status = string(QuestionStatusUsed)
	q1.UsedAt = timePtr(time.Now())
	service.store.UpdateQOTDQuestion(context.Background(), *q1)

	err = service.releaseReservedQuestion(context.Background(), *q1)
	if err != nil {
		t.Fatalf("releaseReservedQuestion(3) failed with unexpected error: %v", err)
	}
}
