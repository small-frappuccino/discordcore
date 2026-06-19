package cleanup

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/small-frappuccino/discordgo"
)

// TestBulkDeleteAgeRejectionFallsBackToSingleDeletes covers the 14-day race:
// when Discord rejects a bulk-delete chunk because at least one message
// crossed the 14-day boundary mid-flight, the cleanup package retries the
// chunk one message at a time so the rest of the chunk is not marked as
// failed.
func TestBulkDeleteAgeRejectionFallsBackToSingleDeletes(t *testing.T) {
	const channelID = "c1"
	chunkIDs := []string{"m1", "m2", "m3"}

	var (
		mu                 sync.Mutex
		bulkCalls          int
		singleDeletedIDs   []string
		bulkChunkErrorSeen bool
		singleErrorIDs     []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/bulk-delete"):
			mu.Lock()
			bulkCalls++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"code":    discordgo.ErrCodeMessageProvidedTooOldForBulkDelete,
				"message": "You can only bulk delete messages that are under 14 days old.",
			})
		case req.Method == http.MethodDelete && strings.Contains(req.URL.Path, "/messages/"):
			parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
			id := parts[len(parts)-1]
			mu.Lock()
			singleDeletedIDs = append(singleDeletedIDs, id)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	withDiscordEndpoints(t, server.URL)
	session := newTestSession(t)

	deleted, failed := DeleteMessages(session, channelID, chunkIDs, DeleteOptions{
		Mode: DeleteModeBulkPreferred,
		OnDeleteError: func(_ string, _ error, _ FailureClass) {
			mu.Lock()
			singleErrorIDs = append(singleErrorIDs, "called")
			mu.Unlock()
		},
		OnChunkError: func(_ []string, _ error, _ FailureClass) {
			mu.Lock()
			bulkChunkErrorSeen = true
			mu.Unlock()
		},
	})

	if deleted != 3 || failed != 0 {
		t.Fatalf("expected age-fallback to recover the chunk, got deleted=%d failed=%d", deleted, failed)
	}
	if bulkCalls != 1 {
		t.Fatalf("expected one bulk call before fallback, got %d", bulkCalls)
	}
	if len(singleDeletedIDs) != 3 {
		t.Fatalf("expected 3 single deletes after age fallback, got %v", singleDeletedIDs)
	}
	if bulkChunkErrorSeen {
		t.Fatal("OnChunkError must not fire on bulk-age fallback")
	}
	if len(singleErrorIDs) != 0 {
		t.Fatalf("OnDeleteError must not fire when single-delete succeeds, got %v", singleErrorIDs)
	}
}

// TestBulkDeleteForbiddenFiresChunkErrorOnce verifies that a non-age chunk
// failure (e.g. the bot lost permissions mid-flight) reports exactly one
// chunk-level error instead of one per-message log line, and counts the
// chunk size as failed.
func TestBulkDeleteForbiddenFiresChunkErrorOnce(t *testing.T) {
	const channelID = "c1"
	chunkIDs := []string{"m1", "m2", "m3", "m4"}

	var (
		mu              sync.Mutex
		chunkErrorCount int
		gotClass        FailureClass
		gotChunk        []string
		perMessageCalls int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/bulk-delete"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{
				"code":    discordgo.ErrCodeMissingPermissions,
				"message": "Missing Permissions",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	withDiscordEndpoints(t, server.URL)
	session := newTestSession(t)

	deleted, failed := DeleteMessages(session, channelID, chunkIDs, DeleteOptions{
		Mode: DeleteModeBulkPreferred,
		OnDeleteError: func(_ string, _ error, _ FailureClass) {
			mu.Lock()
			perMessageCalls++
			mu.Unlock()
		},
		OnChunkError: func(ids []string, _ error, class FailureClass) {
			mu.Lock()
			chunkErrorCount++
			gotClass = class
			gotChunk = append([]string(nil), ids...)
			mu.Unlock()
		},
	})

	if deleted != 0 || failed != len(chunkIDs) {
		t.Fatalf("expected all chunk messages to count as failed, got deleted=%d failed=%d", deleted, failed)
	}
	if chunkErrorCount != 1 {
		t.Fatalf("expected exactly one chunk-level error report, got %d", chunkErrorCount)
	}
	if perMessageCalls != 0 {
		t.Fatalf("expected zero per-message error reports for chunk failure, got %d", perMessageCalls)
	}
	if gotClass != FailureClassForbidden {
		t.Fatalf("expected FailureClassForbidden, got %d", gotClass)
	}
	if len(gotChunk) != len(chunkIDs) {
		t.Fatalf("expected chunk callback to carry full chunk, got %v", gotChunk)
	}
}

// TestSingleDeleteMissingMessageCountsAsDeleted verifies that a 404 from
// single-delete is treated as deletion success (the message is gone, the
// cleanup goal is satisfied) and does not bubble as a failure.
func TestSingleDeleteMissingMessageCountsAsDeleted(t *testing.T) {
	const channelID = "c1"
	ids := []string{"m1", "m2"}

	var (
		mu        sync.Mutex
		callOrder []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
		id := parts[len(parts)-1]
		mu.Lock()
		callOrder = append(callOrder, id)
		mu.Unlock()
		if id == "m2" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{
				"code":    discordgo.ErrCodeUnknownMessage,
				"message": "Unknown Message",
			})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	withDiscordEndpoints(t, server.URL)
	session := newTestSession(t)

	var errCalls int
	deleted, failed := DeleteMessages(session, channelID, ids, DeleteOptions{
		Mode: DeleteModeSingleOnly,
		OnDeleteError: func(_ string, _ error, _ FailureClass) {
			errCalls++
		},
	})

	if deleted != 2 || failed != 0 {
		t.Fatalf("expected missing-message to count as deleted, got deleted=%d failed=%d", deleted, failed)
	}
	if errCalls != 0 {
		t.Fatalf("OnDeleteError must not fire for missing-message, got %d", errCalls)
	}
	if len(callOrder) != 2 {
		t.Fatalf("expected both ids to be attempted, got %v", callOrder)
	}
}

// TestClassifyDeleteErrorWrapsRetainBranchClassification ensures the
// classifier still categorizes wrapped errors correctly when used through
// real call paths.
func TestClassifyDeleteErrorWrapsRetainBranchClassification(t *testing.T) {
	wrapped := errors.New("baseline")
	if got := ClassifyDeleteError(wrapped); got != FailureClassTransient {
		t.Fatalf("plain error should classify as transient, got %d", got)
	}
}

func withDiscordEndpoints(t *testing.T, baseURL string) {
	t.Helper()
	oldAPI := discordgo.EndpointAPI
	oldChannels := discordgo.EndpointChannels
	discordgo.EndpointAPI = baseURL + "/"
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointChannels = oldChannels
	})
}

func newTestSession(t *testing.T) *discordgo.Session {
	t.Helper()
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("discordgo.New: %v", err)
	}
	return session
}
