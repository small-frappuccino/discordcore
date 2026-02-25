package logging

import (
	"testing"
	"time"
)

func TestMonitoringService_CacheRolesSetClearsEntryOnEmptySnapshot(t *testing.T) {
	ms := &MonitoringService{
		rolesCache: make(map[string]cachedRoles),
		rolesTTL:   time.Minute,
	}

	ms.cacheRolesSet("g1", "u1", []string{"r1", "r2"})
	if got, ok := ms.cacheRolesGet("g1", "u1"); !ok || !sameStringSet(got, []string{"r1", "r2"}) {
		t.Fatalf("expected cached roles before clear, got=%v ok=%v", got, ok)
	}

	ms.cacheRolesSet("g1", "u1", []string{})
	if got, ok := ms.cacheRolesGet("g1", "u1"); ok {
		t.Fatalf("expected cache entry to be removed on empty snapshot, got=%v", got)
	}
}
