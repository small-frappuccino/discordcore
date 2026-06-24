package core

import (
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
)

type MockClient struct {
	calls int32
}

func (m *MockClient) BulkOverwriteCommands(appID discord.AppID, commands []api.CreateCommandData) ([]discord.Command, error) {
	atomic.AddInt32(&m.calls, 1)
	return nil, nil
}

func TestRegistry_SyncMock(t *testing.T) {
	t.Parallel()
	r := NewCommandRegistry()
	r.Register(&Command{Name: "test", Description: "test cmd"})
	r.Seal()

	mock := &MockClient{}
	err := r.Sync(mock, discord.AppID(1))
	if err != nil {
		t.Fatalf("sync err: %v", err)
	}

	if mock.calls != 1 {
		t.Fatalf("expected 1 call to BulkOverwriteCommands, got %d", mock.calls)
	}
}

func TestRegistry_ParallelReads(t *testing.T) {
	t.Parallel()
	r := NewCommandRegistry()
	r.Register(&Command{Name: "test1"})
	r.Register(&Command{Name: "test2"})
	r.Seal()

	for i := 0; i < 1000; i++ {
		t.Run("parallel_read_"+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			count := 0
			for cmd := range r.All() {
				if cmd.Name == "test1" || cmd.Name == "test2" {
					count++
				}
			}
			if count != 2 {
				t.Errorf("expected 2 commands, got %d", count)
			}
		})
	}
}
