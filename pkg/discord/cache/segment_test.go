package cache

import (
	"sync"
	"testing"
	"time"
)

func TestSegmentGetSetAndExpire(t *testing.T) {
	seg := newSegment[int](time.Minute, 0)
	if _, ok := seg.Get("missing"); ok {
		t.Fatalf("expected miss on empty segment")
	}

	seg.Set("a", 1)
	if v, ok := seg.Get("a"); !ok || v != 1 {
		t.Fatalf("expected hit for key a, got %v %v", v, ok)
	}

	// Force expiration without waiting by setting explicit expiration in the past.
	seg.SetWithExpiration("a", 2, time.Now().Add(-time.Second))
	if v, ok := seg.Get("a"); ok || v != 0 {
		t.Fatalf("expected expired entry to be treated as miss, got %v %v", v, ok)
	}
}

func TestSegmentLRUEviction(t *testing.T) {
	seg := newSegment[int](0, 2)
	seg.Set("a", 1)
	seg.Set("b", 2)
	// Access a to make b LRU
	if _, ok := seg.Get("a"); !ok {
		t.Fatalf("expected a to be present")
	}
	seg.Set("c", 3) // should evict b

	if _, ok := seg.Get("b"); ok {
		t.Fatalf("expected b to be evicted")
	}
	if v, ok := seg.Get("a"); !ok || v != 1 {
		t.Fatalf("expected a to remain after eviction, got %v %v", v, ok)
	}
	if v, ok := seg.Get("c"); !ok || v != 3 {
		t.Fatalf("expected c to be present, got %v %v", v, ok)
	}
}

func TestSegmentConcurrentAccess(t *testing.T) {
	seg := newSegment[int](0, 0)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%10))
			seg.Set(key, i)
			seg.Get(key)
		}(i)
	}
	wg.Wait()
	// Ensure map isn't empty and no panics occurred under race conditions
	if _, ok := seg.Get("a"); !ok {
		t.Fatalf("expected at least one entry after concurrent access")
	}
}
