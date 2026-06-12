package monitoring

import (
	"context"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

// RolesCacheService encapsulates role-related caching primitives and their lifecycles.
type RolesCacheService struct {
	rolesCache rolesCacheStore
	roleAudit  roleUpdateAuditStore

	configManager *files.ConfigManager
	stopCh        chan struct{}
}

// NewRolesCacheService news roles cache service.
func NewRolesCacheService(configManager *files.ConfigManager) *RolesCacheService {
	return &RolesCacheService{
		configManager: configManager,
		stopCh:        make(chan struct{}),
	}
}

// Start starts.
func (s *RolesCacheService) Start(ctx context.Context) error {
	go s.rolesCacheCleanupLoop(ctx)
	return nil
}

// Stop stops.
func (s *RolesCacheService) Stop(ctx context.Context) error {
	close(s.stopCh)
	return nil
}

func (s *RolesCacheService) rolesCacheCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.rolesCache.evictExpired()
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// CacheRolesSet caches roles set.
func (s *RolesCacheService) CacheRolesSet(guildID, userID string, roles []string) {
	ttl := s.rolesCache.ttl
	if s.configManager != nil {
		if gcfg := s.configManager.GuildConfig(guildID); gcfg != nil {
			if d := gcfg.RolesCacheTTLDuration(); d > 0 {
				ttl = d
			}
		}
	}
	s.rolesCache.set(guildID, userID, roles, ttl)
}

// CacheRolesGet caches roles get.
func (s *RolesCacheService) CacheRolesGet(guildID, userID string) ([]string, bool) {
	return s.rolesCache.get(guildID, userID)
}

// CacheRolesSize caches roles size.
func (s *RolesCacheService) CacheRolesSize() int {
	return s.rolesCache.size()
}

// AuditCachedEntries audits cached entries.
func (s *RolesCacheService) AuditCachedEntries(guildID string, now time.Time) ([]*discordgo.AuditLogEntry, bool) {
	return s.roleAudit.cachedEntries(guildID, now)
}

// AuditStoreEntries audits store entries.
func (s *RolesCacheService) AuditStoreEntries(guildID string, now time.Time, entries []*discordgo.AuditLogEntry) {
	s.roleAudit.storeEntries(guildID, now, entries)
}

// AuditShouldDebounce audits should debounce.
func (s *RolesCacheService) AuditShouldDebounce(guildID, userID string, now time.Time) bool {
	return s.roleAudit.shouldDebounce(guildID, userID, now)
}

// AuditSizes audits sizes.
func (s *RolesCacheService) AuditSizes() (cacheSize, debounceSize int) {
	return s.roleAudit.sizes()
}
