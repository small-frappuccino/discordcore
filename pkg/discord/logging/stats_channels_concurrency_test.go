package logging

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
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

func TestStatsServiceConcurrency_Maps(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := NewStatsService(nil, files.NewConfigManagerWithStore(&files.MemoryConfigStore{}), nil, nil, "", "", nil, nil, nil)
	// Seed with empty config to satisfy basic guards
	_, _ = s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "test-guild",
				Stats: files.StatsConfig{
					Enabled: true,
					Channels: []files.StatsChannelConfig{
						{ChannelID: "test-channel"},
					},
				},
			},
		}
		return nil
	})
	s.lastRun = make(map[string]time.Time)
	s.guilds = make(map[string]*statsGuildState)

	guildID := "test-guild"
	channelID := "test-channel"
	userID := "test-user"

	startCh := make(chan struct{})
	var wg sync.WaitGroup
	numActors := 50

	// 1. Published Channels Maps
	for i := 0; i < numActors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			s.recordStatsPublishedChannel(guildID, channelID, statsPublishedChannel{count: 1, name: "foo", label: "bar"})
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			s.statsPublishedChannels(guildID)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			s.statsPublishedChannel(guildID, channelID)
		}()
	}

	// 2. Guild State Map Pointers
	for i := 0; i < numActors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			s.pruneStatsGuildState(map[string]struct{}{guildID: {}})
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			s.mu.Lock()
			s.ensureStatsGuildStateLocked(guildID)
			s.mu.Unlock()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			state := newStatsGuildState("", nil)
			s.replaceStatsGuildState(guildID, state)
		}()
	}

	// 3. Member Arithmetic
	memberAdd := &discordgo.Member{
		GuildID: guildID,
		User:    &discordgo.User{ID: userID, Bot: false},
		Roles:   []string{"role1"},
	}

	for i := 0; i < numActors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			s.applyStatsMemberAdd(memberAdd)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			s.applyStatsMemberRemove(guildID, userID)
		}()
	}

	close(startCh)

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		// Success! No deadlock or panic.
	case <-ctx.Done():
		t.Fatal("Test timed out: Deadlock detected in concurrent StatsService maps execution.")
	}
}
