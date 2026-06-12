//go:build ignore

package monitoring

import (
	"testing"
)

func TestMonitoringService_CacheRolesSetClearsEntryOnEmptySnapshot(t *testing.T) {
	ms := &MonitoringService{
		rolesCacheService: NewRolesCacheService(nil)}

	ms.rolesCacheService.CacheRolesSet("g1", "u1", []string{"r1", "r2"})
	if got, ok := ms.rolesCacheService.CacheRolesGet("g1", "u1"); !ok || !sameStringSet(got, []string{"r1", "r2"}) {
		t.Fatalf("expected cached roles before clear, got=%v ok=%v", got, ok)
	}

	ms.rolesCacheService.CacheRolesSet("g1", "u1", []string{})
	if got, ok := ms.rolesCacheService.CacheRolesGet("g1", "u1"); ok {
		t.Fatalf("expected cache entry to be removed on empty snapshot, got=%v", got)
	}
}
