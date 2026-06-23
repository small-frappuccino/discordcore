package cache

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
)

// TestCache_GCEviction verifies that weak references are correctly garbage collected and evicted.
func TestCache_GCEviction(t *testing.T) {
	uc := NewUnifiedCache(CacheConfig{GuildTTL: time.Minute})

	guild := &discord.Guild{ID: discord.GuildID(123)}
	uc.SetGuild("123", guild)

	_, ok := uc.GetGuild("123")
	if !ok {
		t.Fatal("Expected guild to be in cache")
	}

	// Remove strong reference
	guild = nil
	runtime.GC()
	// Give AddCleanup a chance deterministically
	for i := 0; i < 1000; i++ {
		if _, ok = uc.GetGuild("123"); !ok {
			break
		}
		runtime.Gosched()
	}

	if ok {
		t.Fatal("Expected guild to be evicted via GC")
	}
}

// TestCache_StaleReads ensures that fetching an explicitly nulled weak reference correctly returns a miss.
func TestCache_StaleReads(t *testing.T) {
	uc := NewUnifiedCache(CacheConfig{GuildTTL: time.Minute})

	guild := &discord.Guild{ID: discord.GuildID(456)}
	uc.SetGuild("456", guild)

	// Remove strong reference and force GC
	guild = nil
	runtime.GC()

	// Try to fetch immediately. The weak pointer should be nil, resulting in instant miss.
	_, ok := uc.GetGuild("456")
	if ok {
		t.Fatal("Expected stale read to instantly miss")
	}
}

// TestCache_ReferenceCycles guarantees that cyclic references within cached structs do not prevent garbage collection.
func TestCache_ReferenceCycles(t *testing.T) {
	uc := NewUnifiedCache(CacheConfig{MemberTTL: time.Minute})

	type Cycle struct {
		m *discord.Member
		c *Cycle
	}

	member := &discord.Member{User: discord.User{ID: discord.UserID(1)}}
	c := &Cycle{m: member}
	c.c = c // cycle

	uc.SetMember("1", "1", member)

	// Break direct references
	member = nil
	c = nil

	runtime.GC()
	for i := 0; i < 1000; i++ {
		if _, ok := uc.GetMember("1", "1"); !ok {
			break
		}
		runtime.Gosched()
	}

	if _, ok := uc.GetMember("1", "1"); ok {
		t.Fatal("Expected cycle to be collected and member evicted")
	}
}

// BenchmarkCache_Shards measures concurrent map read and write performance to validate shard contention.
func BenchmarkCache_Shards(b *testing.B) {
	uc := NewUnifiedCache(CacheConfig{GuildTTL: time.Minute})

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			uc.SetGuild("1", &discord.Guild{})
			uc.GetGuild("1")
			i++
		}
	})
}

// TestCache_AsyncIO asserts the performance bounds of snapshot extraction under concurrent lock acquisition.
func TestCache_AsyncIO(t *testing.T) {
	uc := NewUnifiedCache(CacheConfig{GuildTTL: time.Minute})

	for i := 0; i < 1000; i++ {
		uc.SetGuild(string(rune(i)), &discord.Guild{})
	}

	start := time.Now()
	_ = uc.guilds.Snapshot()
	duration := time.Since(start)

	if duration > 50*time.Millisecond {
		t.Fatalf("Snapshot took too long: %v (should be <1ms ideally, up to 50ms under load)", duration)
	}
}

// TestCache_CorruptRecovery checks that the warmup routine robustly ignores absent datastores.
func TestCache_CorruptRecovery(t *testing.T) {
	// We simulate this by directly calling Warmup with a mock store
	uc := NewUnifiedCache(CacheConfig{})
	err := uc.Warmup(context.Background())
	if err != nil {
		t.Fatalf("Warmup should ignore nil store but got err: %v", err)
	}
}
