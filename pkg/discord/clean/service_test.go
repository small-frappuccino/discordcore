package clean

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/small-frappuccino/discordcore/pkg/clean"
)

type InMemoryMetrics struct {
	mu          sync.Mutex
	Successes   int
	TotalDelete int
	Failures    int
}

func (m *InMemoryMetrics) RecordCleanAttempt() {}
func (m *InMemoryMetrics) RecordCleanSuccess(durationMs int64, deleted int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Successes++
	m.TotalDelete += deleted
}
func (m *InMemoryMetrics) RecordCleanFailure(cause string, durationMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Failures++
}
func (m *InMemoryMetrics) RecordCleanDeleteFailure(class string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Failures++
}
func (m *InMemoryMetrics) RecordCleanAuditLogFailure() {}

type mockClient struct {
	mu                 sync.Mutex
	messagesFunc       func(limit uint) ([]discord.Message, error)
	messagesBeforeFunc func(before discord.MessageID, limit uint) ([]discord.Message, error)
	deleteMessagesFunc func(messageIDs []discord.MessageID) error
	deleteMessageFunc  func(messageID discord.MessageID) error
	createMessageFunc  func(data api.SendMessageData) (*discord.Message, error)
	deleteMsgErr       error
	deletedMsgs        []discord.MessageID
	createMsgErr       error
}

func (m *mockClient) Messages(channelID discord.ChannelID, limit uint) ([]discord.Message, error) {
	if m.messagesFunc != nil {
		return m.messagesFunc(limit)
	}
	return nil, nil
}
func (m *mockClient) MessagesBefore(channelID discord.ChannelID, before discord.MessageID, limit uint) ([]discord.Message, error) {
	if m.messagesBeforeFunc != nil {
		return m.messagesBeforeFunc(before, limit)
	}
	return nil, nil
}
func (m *mockClient) DeleteMessages(channelID discord.ChannelID, messageIDs []discord.MessageID, reason api.AuditLogReason) error {
	if m.deleteMessagesFunc != nil {
		return m.deleteMessagesFunc(messageIDs)
	}
	return nil
}
func (m *mockClient) DeleteMessage(channelID discord.ChannelID, messageID discord.MessageID, reason api.AuditLogReason) error {
	if m.deleteMsgErr != nil {
		return m.deleteMsgErr
	}
	m.mu.Lock()
	m.deletedMsgs = append(m.deletedMsgs, messageID)
	m.mu.Unlock()
	if m.deleteMessageFunc != nil {
		return m.deleteMessageFunc(messageID)
	}
	return nil
}

func (m *mockClient) SendMessageComplex(channelID discord.ChannelID, data api.SendMessageData) (*discord.Message, error) {
	if m.createMessageFunc != nil {
		return m.createMessageFunc(data)
	}
	return &discord.Message{}, nil
}

func TestExecuteClean_Pagination(t *testing.T) {
	t.Parallel()
	mockClock := time.Now()

	client := &mockClient{
		messagesFunc: func(limit uint) ([]discord.Message, error) {
			msgs := make([]discord.Message, 100)
			for i := 0; i < 100; i++ {
				msgs[i] = discord.Message{ID: discord.MessageID(1000 - i), Timestamp: discord.NewTimestamp(mockClock)}
			}
			return msgs, nil
		},
		messagesBeforeFunc: func(before discord.MessageID, limit uint) ([]discord.Message, error) {
			start := int(before) - 1
			if start <= 0 {
				return nil, nil
			}
			count := int(limit)
			if start < count {
				count = start
			}
			msgs := make([]discord.Message, count)
			for i := 0; i < count; i++ {
				msgs[i] = discord.Message{ID: discord.MessageID(start - i), Timestamp: discord.NewTimestamp(mockClock)}
			}
			return msgs, nil
		},
		deleteMessagesFunc: func(messageIDs []discord.MessageID) error {
			return nil
		},
	}

	metrics := &InMemoryMetrics{}
	svc := NewService(client, metrics, slog.Default())
	svc.now = func() time.Time { return mockClock }

	filter := clean.Filter{Count: 100}
	deleted, err := svc.ExecuteClean(context.Background(), 1, filter, 0, "test")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if deleted != 100 {
		t.Errorf("expected 100 deleted, got %d", deleted)
	}
}

func TestExecuteClean_Degradation_50034(t *testing.T) {
	t.Parallel()
	mockClock := time.Now()

	client := &mockClient{
		messagesFunc: func(limit uint) ([]discord.Message, error) {
			msgs := make([]discord.Message, 10)
			for i := 0; i < 10; i++ {
				msgs[i] = discord.Message{ID: discord.MessageID(100 - i), Timestamp: discord.NewTimestamp(mockClock)}
			}
			return msgs, nil
		},
		deleteMessagesFunc: func(messageIDs []discord.MessageID) error {
			return &httputil.HTTPError{Code: 50034, Message: "You can only bulk delete messages that are under 14 days old."}
		},
		deleteMessageFunc: func(messageID discord.MessageID) error {
			return nil // fallback works
		},
	}

	metrics := &InMemoryMetrics{}
	svc := NewService(client, metrics, slog.Default())
	svc.now = func() time.Time { return mockClock }

	filter := clean.Filter{Count: 10}
	deleted, err := svc.ExecuteClean(context.Background(), 1, filter, 0, "test")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if deleted != 10 {
		t.Errorf("expected 10 deleted through fallback, got %d", deleted)
	}
}

func TestExecuteClean_Concurrency_Race(t *testing.T) {
	t.Parallel()
	mockClock := time.Now().Add(-20 * 24 * time.Hour) // force single deletes

	client := &mockClient{
		messagesFunc: func(limit uint) ([]discord.Message, error) {
			msgs := make([]discord.Message, 100)
			for i := 0; i < 100; i++ {
				msgs[i] = discord.Message{ID: discord.MessageID(1000 - i), Timestamp: discord.NewTimestamp(mockClock)}
			}
			return msgs, nil
		},
		deleteMessageFunc: func(messageID discord.MessageID) error {
			for i := 0; i < 1000; i++ {
				runtime.Gosched() // simulate IO delay by yielding CPU deterministically
			}
			return nil
		},
	}

	metrics := &InMemoryMetrics{}
	svc := NewService(client, metrics, slog.Default())
	svc.now = func() time.Time { return mockClock }

	filter := clean.Filter{Count: 100}

	t.Run("concurrency race test", func(t *testing.T) {
		deleted, err := svc.ExecuteClean(context.Background(), 1, filter, 0, "test")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if deleted != 100 {
			t.Errorf("expected 100, got %d", deleted)
		}

		metrics.mu.Lock()
		defer metrics.mu.Unlock()
		if metrics.TotalDelete != 100 {
			t.Errorf("expected 100 recorded metrics total delete, got %d", metrics.TotalDelete)
		}
	})
}

func TestExecuteClean_AuditDispatch(t *testing.T) {
	t.Parallel()
	mockClock := time.Now()
	var auditLogged atomic.Bool

	client := &mockClient{
		messagesFunc: func(limit uint) ([]discord.Message, error) {
			return []discord.Message{{ID: 1, Timestamp: discord.NewTimestamp(mockClock)}}, nil
		},
		deleteMessagesFunc: func(messageIDs []discord.MessageID) error { return nil },
		createMessageFunc: func(data api.SendMessageData) (*discord.Message, error) {
			if len(data.Embeds) > 0 {
				auditLogged.Store(true)
				if data.Embeds[0].Title != "Clean Command Executed" {
					t.Errorf("unexpected embed title: %s", data.Embeds[0].Title)
				}
			}
			return nil, errors.New("audit failure") // ensure it doesn't break execution
		},
	}

	metrics := &InMemoryMetrics{}
	svc := NewService(client, metrics, slog.Default())
	svc.now = func() time.Time { return mockClock }

	deleted, err := svc.ExecuteClean(context.Background(), 1, clean.Filter{Count: 1}, 2, "tester")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	svc.Close() // Gracefully waits for audit log dispatch
	if !auditLogged.Load() {
		t.Errorf("audit log was not dispatched")
	}
}
