package discord

import (
	"net/http"
	"testing"
)

func BenchmarkAcquireBucket(b *testing.B) {
	b.ReportAllocs()
	g := NewRESTGateway()
	key := "bench:bucket:1"

	// populate
	resp := &http.Response{
		Header: make(http.Header),
	}
	resp.Header.Set("X-RateLimit-Remaining", "1000")
	resp.Header.Set("X-RateLimit-Reset-After", "60")
	g.updateBucket(key, resp)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.acquireBucket(key)
	}
}
