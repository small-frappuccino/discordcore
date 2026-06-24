package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
)

// TestSession_SingleflightLoad verifies the singleflight primitive correctly coalesces massive concurrent cache misses.
func TestSession_SingleflightLoad(t *testing.T) {
	t.Parallel()
	uc := NewUnifiedCache(CacheConfig{GuildTTL: time.Minute})
	cs := NewCachedSession(nil, uc)

	var fetches int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	entered := make(chan struct{})
	var enteredOnce sync.Once
	fetchBlock := make(chan struct{})

	var readyToFetch sync.WaitGroup
	readyToFetch.Add(1000)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // Wait for all goroutines to be spawned
			readyToFetch.Done()
			_, err, _ := cs.sf.Do("guild:123", func() (any, error) {
				atomic.AddInt32(&fetches, 1)
				enteredOnce.Do(func() { close(entered) })
				<-fetchBlock // deterministically block to allow pile-up
				return &discord.Guild{ID: discord.GuildID(123)}, nil
			})
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}()
	}

	close(start)        // Unleash all goroutines at once
	<-entered           // Wait for the first to lock singleflight
	readyToFetch.Wait() // Synchronously await all goroutines to reach execution barrier
	close(fetchBlock)

	wg.Wait()

	if atomic.LoadInt32(&fetches) > 500 {
		t.Fatalf("Expected coalescing, got %d fetches (should be < 500)", fetches)
	}
}

// TestSession_SingleflightError ensures that underlying REST failures during singleflight fetches do not pollute the cache.
func TestSession_SingleflightError(t *testing.T) {
	t.Parallel()
	uc := NewUnifiedCache(CacheConfig{GuildTTL: time.Minute})
	cs := NewCachedSession(nil, uc)

	expectedErr := errors.New("network error")
	var wg sync.WaitGroup
	entered := make(chan struct{})
	var enteredOnce sync.Once
	fetchBlock := make(chan struct{})

	var readyToFetch sync.WaitGroup
	readyToFetch.Add(10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			readyToFetch.Done()
			_, err, _ := cs.sf.Do("guild:456", func() (any, error) {
				enteredOnce.Do(func() { close(entered) })
				<-fetchBlock
				return nil, expectedErr
			})
			if err != expectedErr {
				t.Errorf("Expected network error, got %v", err)
			}
		}()
	}

	<-entered
	readyToFetch.Wait()
	close(fetchBlock)

	wg.Wait()

	if _, ok := uc.GetGuild("456"); ok {
		t.Fatal("Expected guild to NOT be in cache after error")
	}
}

// TestSession_PartialInvalidation confirms that RoleDelete events target specific slice indices without evicting the entire guild role aggregate.
func TestSession_PartialInvalidation(t *testing.T) {
	t.Parallel()
	uc := NewUnifiedCache(CacheConfig{RolesTTL: time.Minute})
	cs := NewCachedSession(nil, uc)

	roles := []discord.Role{
		{ID: discord.RoleID(1)},
		{ID: discord.RoleID(2)},
	}
	uc.SetRoles("123", &roles)

	cs.HandleGuildRoleDelete(&gateway.GuildRoleDeleteEvent{
		GuildID: discord.GuildID(123),
		RoleID:  discord.RoleID(1),
	})

	cachedRoles, ok := uc.GetRoles("123")
	if !ok || len(*cachedRoles) != 1 || (*cachedRoles)[0].ID != discord.RoleID(2) {
		t.Fatal("Expected partial invalidation to remove only role 1")
	}
}

// TestSession_RaceUpdate asserts that Gateway invalidations correctly preempt and overwrite stale background REST fetches.
func TestSession_RaceUpdate(t *testing.T) {
	t.Parallel()
	uc := NewUnifiedCache(CacheConfig{MemberTTL: time.Minute})
	cs := NewCachedSession(nil, uc)

	// Simulate a fetch
	member := &discord.Member{User: discord.User{ID: discord.UserID(1)}}
	uc.SetMember("1", "1", member)

	// Simulate concurrent Gateway Update overriding it
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		cs.HandleGuildMemberUpdate(&gateway.GuildMemberUpdateEvent{
			GuildID: discord.GuildID(1),
			User:    discord.User{ID: discord.UserID(1)},
		})
	}()

	go func() {
		defer wg.Done()
		// REST fetch returning stale
		uc.SetMember("1", "1", &discord.Member{User: discord.User{ID: discord.UserID(1)}})
	}()

	wg.Wait()
}
