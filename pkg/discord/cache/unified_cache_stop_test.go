package cache

import "testing"

func TestUnifiedCacheStopKeepsClosedStopChannel(t *testing.T) {
	t.Parallel()

	uc := NewUnifiedCache(DefaultCacheConfig())
	stopChan := uc.stopCleanup
	if stopChan == nil {
		t.Fatal("expected stop channel to be initialized")
	}

	uc.Stop()

	if uc.stopCleanup != stopChan {
		t.Fatal("expected Stop to keep the closed stop channel installed")
	}

	select {
	case <-uc.stopCleanup:
	default:
		t.Fatal("expected stop channel to be closed after Stop")
	}
}
