package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func TestStreamGuildMembersContext_ProcessesPagesInOrder(t *testing.T) {

	page1 := make([]*discordgo.Member, 1000)
	for i := 0; i < 1000; i++ {
		page1[i] = testGuildMember(fmt.Sprintf("u%d", i+1))
	}
	page2 := []*discordgo.Member{testGuildMember("u1001")}
	pages := map[string][]*discordgo.Member{
		"":      page1,
		"u1000": page2,
	}

	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		after := r.URL.Query().Get("after")
		_ = json.NewEncoder(w).Encode(pages[after])
	})

	ms := &MonitoringService{
		session: session,
	}

	var got []string
	total := 0
	for members, err := range ms.StreamGuildMembersContext(context.Background(), "g1") {
		if err != nil {
			t.Fatalf("StreamGuildMembersContext returned error: %v", err)
		}
		for _, member := range members {
			got = append(got, member.User.ID)
		}
		total += len(members)
	}

	if total != 1001 {
		t.Fatalf("unexpected total count: got %d want 1001", total)
	}

	if len(got) != 1001 {
		t.Fatalf("unexpected member count: got %d want 1001", len(got))
	}
	if got[0] != "u1" || got[999] != "u1000" || got[1000] != "u1001" {
		t.Fatalf("unexpected member order")
	}
}

func TestStreamGuildMembersContext_StopsOnError(t *testing.T) {

	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
	})

	ms := &MonitoringService{
		session: session,
	}

	total := 0
	var outErr error
	for members, err := range ms.StreamGuildMembersContext(context.Background(), "g1") {
		if err != nil {
			outErr = err
			break
		}
		total += len(members)
	}

	if outErr == nil {
		t.Fatalf("expected handler error, got nil")
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
