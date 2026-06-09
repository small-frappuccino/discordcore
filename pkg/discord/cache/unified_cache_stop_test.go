package cache_test

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"testing"
)

func TestUnifiedCacheStopIdempotency(t *testing.T) {
	uc := cache.NewUnifiedCache(cache.DefaultCacheConfig())

	// Black-box test: ensure Stop does not block and is idempotent
	uc.Stop()
	uc.Stop() // Should not panic or block
}
