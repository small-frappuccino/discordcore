package automod

import (
	"context"
	"testing"
	"time"
)

func TestAutomodFallbackShouldDedup_EmptyKeyNeverDedups(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	if as.fallbackShouldDedupAt("", now) {
		t.Fatal("empty key must not dedup")
	}
	if as.fallbackShouldDedupAt("", now.Add(1*time.Second)) {
		t.Fatal("empty key must remain non-deduping on repeated calls")
	}
}

func TestAutomodFallbackShouldDedup_SecondCallWithinTTLReturnsTrue(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	key := "automod:g1:r1:u1:content:abcd"

	if as.fallbackShouldDedupAt(key, now) {
		t.Fatal("first call must return false (not yet seen)")
	}
	if !as.fallbackShouldDedupAt(key, now.Add(5*time.Second)) {
		t.Fatal("second call within TTL must return true")
	}
}

func TestAutomodFallbackShouldDedup_CallAfterTTLReturnsFalse(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	key := "automod:g1:r1:u1:content:abcd"

	if as.fallbackShouldDedupAt(key, now) {
		t.Fatal("first call must return false")
	}
	if as.fallbackShouldDedupAt(key, now.Add(automodFallbackDedupTTL+time.Second)) {
		t.Fatal("call after TTL must return false")
	}
}

func TestAutomodFallbackShouldDedup_DistinctKeysDoNotInterfere(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)

	if as.fallbackShouldDedupAt("key-a", now) {
		t.Fatal("first key-a call must return false")
	}
	if as.fallbackShouldDedupAt("key-b", now.Add(time.Second)) {
		t.Fatal("distinct key-b must not collide with key-a")
	}
	if !as.fallbackShouldDedupAt("key-a", now.Add(2*time.Second)) {
		t.Fatal("repeat of key-a must still dedup")
	}
}

func TestAutomodFallbackShouldDedup_LazyCleanupBoundsMap(t *testing.T) {
	t.Parallel()

	as := NewAutomodService(nil, nil, "")
	as.Start(context.Background())
	defer as.Stop(context.Background())

	base := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)

	// Seed past expiry: at base+0, all are expired by base+TTL+1s.
	for i := 0; i < automodFallbackDedupCleanupThreshold+5; i++ {
		key := "stale-" + time.Duration(i).String()
		as.fallbackShouldDedupAt(key, base)
	}

	// Trigger cleanup with a new key well past TTL.
	as.fallbackShouldDedupAt("fresh", base.Add(automodFallbackDedupTTL+time.Second))
}
