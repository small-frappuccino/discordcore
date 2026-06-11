package logging

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestStatsServiceConcurrencyAndDeadlock(t *testing.T) {
	// Bounded context timeout to catch silent deadlocks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := NewStatsService(nil, files.NewConfigManagerWithStore(&files.MemoryConfigStore{}), nil, nil, "", "", nil, nil, nil)
	s.lastRun = make(map[string]time.Time)
	s.guilds = make(map[string]*statsGuildState)

	guildID := "test-guild"
	userID := "test-user"

	startCh := make(chan struct{})
	var wg sync.WaitGroup

	numActors := 50

	// 1. applyStatsMemberUpdate (Writer)
	for i := 0; i < numActors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			select {
			case <-ctx.Done():
				return
			default:
				s.ApplyStatsMemberUpdate(guildID, userID, false, []string{"role1"})
			}
		}()
	}

	// 2. statsSnapshot (Reader)
	for i := 0; i < numActors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			select {
			case <-ctx.Done():
				return
			default:
				s.statsSnapshot(guildID)
			}
		}()
	}

	// 3. shouldRunStatsUpdate (Reader)
	for i := 0; i < numActors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			select {
			case <-ctx.Done():
				return
			default:
				s.shouldRunStatsUpdate(guildID, 5*time.Minute)
			}
		}()
	}

	// Centralized broadcast channel to force simultaneous lock acquisition attempts
	close(startCh)

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		// Success! No deadlock.
	case <-ctx.Done():
		t.Fatal("Test timed out: Deadlock detected in concurrent StatsService execution.")
	}
}
