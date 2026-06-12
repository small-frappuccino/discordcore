//go:build integration

package qotd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// TestServicePublishNowConflict tests the failure mode where a swarm deployment causes multiple
// pods to evaluate the same PublishNow request or where a scheduled slot is provisioned by another
// pod precisely after the actor lock is acquired but before the provision finishes.
func TestServicePublishNowConflict(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)
	fixedNow := time.Date(2026, 6, 12, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	q1, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question for Pod A",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	// Another question to be selected by the concurrent pod
	q2, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Question for Pod B",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	publishDate := NormalizePublishDateUTC(fixedNow)

	// We simulate a race condition where the current pod attempts to publish, but it hits a Postgres constraint
	// because Pod B just created the QOTDOfficialPostRecord for the exact same PublishDateUTC.
	// We do this by calling resolvePublishNowConflict directly as if `provisionManualOfficialPost` caught the unique constraint error.

	// Pod B provisions its post:
	provisionedByB, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           q2.ID,
		PublishMode:          string(PublishModeManual),
		ConsumeAutomaticSlot: true,
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateProvisioning),
		ChannelID:            integrationQuestionChannelID,
		QuestionTextSnapshot: q2.Body,
		Nonce:                "nonce-from-pod-B",
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning by Pod B failed: %v", err)
	}

	// Pod A hits the conflict and tries to resolve it
	// Pod A will first release its reserved question (q1)
	service.releaseManualReservedQuestion(context.Background(), "g1", *q1)

	// Now Pod A resolves the conflict:
	result, err := service.resolvePublishNowConflict(context.Background(), "g1", publishDate, errors.New("simulated unique constraint violation"))
	if err != nil {
		// Wait, resolvePublishNowConflict returns ErrPublishInProgress if the other pod is still provisioning.
		if !errors.Is(err, ErrPublishInProgress) {
			t.Fatalf("Expected ErrPublishInProgress since Pod B is still provisioning, got %v", err)
		}
	} else if result != nil {
		t.Fatalf("Expected conflict resolver to not return a result when provisioning, got %v", result)
	}

	// Now Pod B finishes:
	_, err = store.UpdateQOTDOfficialPostState(context.Background(), provisionedByB.ID, string(OfficialPostStateCurrent), nil, nil)
	if err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState failed: %v", err)
	}

	// Pod A hits the conflict again (e.g. user clicked twice rapidly):
	result, err = service.resolvePublishNowConflict(context.Background(), "g1", publishDate, errors.New("simulated unique constraint violation"))
	if err != nil {
		if !errors.Is(err, ErrAlreadyPublished) {
			t.Fatalf("Expected ErrAlreadyPublished since Pod B is done, got %v", err)
		}
	} else if result != nil {
		t.Fatalf("Expected conflict resolver to not return a result when finished, got %v", result)
	}
}

// TestServicePublishNowDuplicateThread tests the failure mode where Discord returns 500
// or times out after creating the thread, so our code retries but Discord yields an error that
// the thread already exists, or we just want to ensure publishResultFromOfficialPost reconstructs cleanly.
func TestServicePublishResultFromOfficialPost(t *testing.T) {
	service, _, _ := newIntegrationTestQOTDService(t)

	q1, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Reconstructed",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	post := storage.QOTDOfficialPostRecord{
		GuildID:                    "g1",
		QuestionID:                 q1.ID,
		DiscordThreadID:            "thread-123",
		DiscordStarterMessageID:    "msg-123",
		QuestionListThreadID:       "list-123",
		QuestionListEntryMessageID: "entry-123",
		PublishedAt:                timePtr(time.Now()),
	}

	res, err := service.publishResultFromOfficialPost(context.Background(), post)
	if err != nil {
		t.Fatalf("publishResultFromOfficialPost failed: %v", err)
	}
	if res.Question.ID != q1.ID {
		t.Fatalf("Expected question ID %d, got %d", q1.ID, res.Question.ID)
	}
	if res.OfficialPost.DiscordThreadID != "thread-123" {
		t.Fatalf("Expected thread ID thread-123, got %s", res.OfficialPost.DiscordThreadID)
	}
}
