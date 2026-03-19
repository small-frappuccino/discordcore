package logging

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func testGuildMember(id string) *discordgo.Member {
	return &discordgo.Member{
		User: &discordgo.User{ID: id},
	}
}

func TestPaginateGuildMembersContext_ProcessesPagesInOrder(t *testing.T) {
	t.Parallel()

	pages := map[string][]*discordgo.Member{
		"":   {testGuildMember("u1"), testGuildMember("u2")},
		"u2": {testGuildMember("u3")},
	}

	var got []string
	total, err := paginateGuildMembersContext(
		context.Background(),
		"g1",
		2,
		func(ctx context.Context, guildID, after string, limit int) ([]*discordgo.Member, error) {
			if guildID != "g1" {
				t.Fatalf("unexpected guild id: %s", guildID)
			}
			if limit != 2 {
				t.Fatalf("unexpected page size: %d", limit)
			}
			return pages[after], nil
		},
		func(members []*discordgo.Member) error {
			for _, member := range members {
				got = append(got, member.User.ID)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("paginateGuildMembersContext returned error: %v", err)
	}
	if total != 3 {
		t.Fatalf("unexpected total count: got %d want 3", total)
	}

	want := []string{"u1", "u2", "u3"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected member order: got %v want %v", got, want)
	}
}

func TestPaginateGuildMembersContext_StopsOnHandlerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("stop")
	total, err := paginateGuildMembersContext(
		context.Background(),
		"g1",
		2,
		func(ctx context.Context, guildID, after string, limit int) ([]*discordgo.Member, error) {
			return []*discordgo.Member{testGuildMember("u1"), testGuildMember("u2")}, nil
		},
		func(members []*discordgo.Member) error {
			return wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected handler error, got %v", err)
	}
	if total != 0 {
		t.Fatalf("expected zero committed total on handler failure, got %d", total)
	}
}

func TestRunGuildTasksWithLimit_RespectsMaxConcurrency(t *testing.T) {
	t.Parallel()

	guildIDs := []string{"g1", "g2", "g3", "g4"}
	release := make(chan struct{})
	started := make(chan string, len(guildIDs))

	var active atomic.Int32
	var peak atomic.Int32

	done := make(chan error, 1)
	go func() {
		done <- runGuildTasksWithLimit(context.Background(), guildIDs, 2, func(ctx context.Context, guildID string) error {
			cur := active.Add(1)
			for {
				prev := peak.Load()
				if cur <= prev || peak.CompareAndSwap(prev, cur) {
					break
				}
			}
			started <- guildID
			<-release
			active.Add(-1)
			return nil
		})
	}()

	for range 2 {
		select {
		case <-started:
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("expected two tasks to start under the concurrency limit")
		}
	}

	select {
	case guildID := <-started:
		t.Fatalf("task %s started before a concurrency slot was released", guildID)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	if err := <-done; err != nil {
		t.Fatalf("runGuildTasksWithLimit returned error: %v", err)
	}
	if got := peak.Load(); got > 2 {
		t.Fatalf("observed concurrency %d exceeds limit 2", got)
	}
}
