package files

import (
	"path/filepath"

	dcache "github.com/alice-bnuy/discordcore/v2/internal/cache"
)

// Deprecated: use internal/cache.AvatarCacheManager instead.
// NewAvatarCacheManager returns the default manager from the unified cache package.
func NewAvatarCacheManager() *dcache.AvatarCacheManager {
	return dcache.NewDefaultAvatarCacheManager()
}

// GetApplicationCacheFilePath returns the standardized path for application_cache.json
func GetApplicationCacheFilePath() string {
	return filepath.Join(ApplicationSupportPath, "data", "application_cache.json")
}
