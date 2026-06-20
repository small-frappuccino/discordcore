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

func TestSession_SingleflightLoad(t *testing.T) {
	uc := NewUnifiedCache(CacheConfig{GuildTTL: time.Minute})
	cs := NewCachedSession(nil, uc)

	var fetches int32
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err, _ := cs.sf.Do("guild:123", func() (any, error) {
				atomic.AddInt32(&fetches, 1)
				time.Sleep(10 * time.Millisecond) // simulate fetch
				return &discord.Guild{ID: discord.GuildID(123)}, nil
			})
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt32(&fetches) != 1 {
		t.Fatalf("Expected exactly 1 fetch, got %d", fetches)
	}
}

func TestSession_SingleflightError(t *testing.T) {
	uc := NewUnifiedCache(CacheConfig{GuildTTL: time.Minute})
	cs := NewCachedSession(nil, uc)

	expectedErr := errors.New("network error")
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err, _ := cs.sf.Do("guild:456", func() (any, error) {
				time.Sleep(10 * time.Millisecond)
				return nil, expectedErr
			})
			if err != expectedErr {
				t.Errorf("Expected network error, got %v", err)
			}
		}()
	}

	wg.Wait()

	if _, ok := uc.GetGuild("456"); ok {
		t.Fatal("Expected guild to NOT be in cache after error")
	}
}

func TestSession_PartialInvalidation(t *testing.T) {
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

func TestSession_RaceUpdate(t *testing.T) {
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
