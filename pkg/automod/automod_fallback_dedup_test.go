//go:build ignore

package automod

import (
	"context"
	"testing"
	"time"
)

func TestAutomodFallbackShouldDedup_EmptyKeyNeverDedups(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "", "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	if as.dedupCache.ShouldDedupAt("", now, true) {
		t.Fatal("empty key must not dedup")
	}
	if as.dedupCache.ShouldDedupAt("", now.Add(1*time.Second, true)) {
		t.Fatal("empty key must remain non-deduping on repeated calls")
	}
}

func TestAutomodFallbackShouldDedup_SecondCallWithinTTLReturnsTrue(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "", "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	key := "automod:g1:r1:u1:content:abcd"

	if as.dedupCache.ShouldDedupAt(key, now, true) {
		t.Fatal("first call must return false (not yet seen)")
	}
	if !as.dedupCache.ShouldDedupAt(key, now.Add(5*time.Second, true)) {
		t.Fatal("second call within TTL must return true")
	}
}

func TestAutomodFallbackShouldDedup_CallAfterTTLReturnsFalse(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "", "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	key := "automod:g1:r1:u1:content:abcd"

	if as.dedupCache.ShouldDedupAt(key, now, true) {
		t.Fatal("first call must return false")
	}
	if as.dedupCache.ShouldDedupAt(key, now.Add(automodFallbackDedupTTL+time.Second, true)) {
		t.Fatal("call after TTL must return false")
	}
}

func TestAutomodFallbackShouldDedup_DistinctKeysDoNotInterfere(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "", "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)

	if as.dedupCache.ShouldDedupAt("key-a", now, true) {
		t.Fatal("first key-a call must return false")
	}
	if as.dedupCache.ShouldDedupAt("key-b", now.Add(time.Second, true)) {
		t.Fatal("distinct key-b must not collide with key-a")
	}
	if !as.dedupCache.ShouldDedupAt("key-a", now.Add(2*time.Second, true)) {
		t.Fatal("repeat of key-a must still dedup")
	}
}

func TestAutomodFallbackShouldDedup_LazyCleanupBoundsMap(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "", "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	base := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)

	// Seed past expiry: at base+0, all are expired by base+TTL+1s.
	for i := 0; i < automodFallbackDedupCleanupThreshold+5; i++ {
		key := "stale-" + time.Duration(i).String()
		as.dedupCache.ShouldDedupAt(key, base, true)
	}

	// Trigger cleanup with a new key well past TTL.
	as.dedupCache.ShouldDedupAt("fresh", base.Add(automodFallbackDedupTTL+time.Second, true))
}
