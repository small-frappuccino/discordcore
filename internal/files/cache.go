package files

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/alice-bnuy/discordcore/v2/internal/util"

	"github.com/alice-bnuy/logutil"
)

// AvatarMultiGuildCache represents the cache file containing all guilds
type AvatarMultiGuildCache struct {
	Guilds      map[string]*AvatarCache `json:"guilds"`
	LastUpdated time.Time               `json:"last_updated"`
	Version     string                  `json:"version"`
}

type AvatarCacheManager struct {
	path        string
	guilds      map[string]*AvatarCache
	mu          sync.RWMutex
	saveMu      sync.Mutex
	lastSave    time.Time
	jsonManager *util.JSONManager
}

func NewAvatarCacheManager() *AvatarCacheManager {
	// Use unified path function for application cache
	path := GetApplicationCacheFilePath()
	return &AvatarCacheManager{
		path:        path,
		guilds:      make(map[string]*AvatarCache),
		jsonManager: util.NewJSONManager(path),
	}
}

// Load loads the cache from the file.
func (m *AvatarCacheManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			m.guilds = make(map[string]*AvatarCache)
			logutil.WithField("path", m.path).Info("Cache file does not exist, initializing empty cache")
			return nil
		}
		logutil.WithFields(map[string]interface{}{
			"path":  m.path,
			"error": err,
		}).Error("Failed to read cache file")
		return fmt.Errorf(ErrReadCacheFile, err)
	}

	var multiCache AvatarMultiGuildCache
	if err := json.Unmarshal(data, &multiCache); err == nil && multiCache.Guilds != nil {
		m.guilds = multiCache.Guilds
		logutil.WithField("path", m.path).Info("Cache loaded successfully")
		return nil
	}

	var oldCache AvatarCache
	if err := json.Unmarshal(data, &oldCache); err != nil {
		logutil.WithFields(map[string]interface{}{
			"path":  m.path,
			"error": err,
		}).Error("Failed to unmarshal cache file")
		return fmt.Errorf(ErrUnmarshalCache, err)
	}

	m.guilds = make(map[string]*AvatarCache)
	if oldCache.GuildID != "" {
		m.guilds[oldCache.GuildID] = &oldCache
		logutil.WithField("guildID", oldCache.GuildID).Info("Loaded old cache format for guild")
	}

	return nil
}

// GuildCache retrieves or initializes the cache for a specific guild.
func (m *AvatarCacheManager) GuildCache(guildID string) *AvatarCache {
	m.mu.RLock()
	existing, ok := m.guilds[guildID]
	m.mu.RUnlock()
	if ok {
		logutil.WithField("guildID", guildID).Debug("Cache hit for guild")
		return existing
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cache, ok := m.guilds[guildID]; ok {
		logutil.WithField("guildID", guildID).Debug("Cache hit for guild after lock")
		return cache
	}
	cache := &AvatarCache{
		Avatars:     make(map[string]string),
		LastUpdated: time.Now(),
		GuildID:     guildID,
	}
	m.guilds[guildID] = cache
	logutil.WithField("guildID", guildID).Info("Initialized new cache for guild")
	return cache
}

// UpdateAvatar updates the avatar hash for a user in a specific guild.
func (m *AvatarCacheManager) UpdateAvatar(guildID, userID, avatarHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cache, exists := m.guilds[guildID]
	if !exists {
		cache = &AvatarCache{
			Avatars:     make(map[string]string),
			LastUpdated: time.Now(),
			GuildID:     guildID,
		}
		m.guilds[guildID] = cache
		logutil.WithField("guildID", guildID).Info("Created new cache for guild during avatar update")
	}

	cache.Avatars[userID] = avatarHash
	cache.LastUpdated = time.Now()
	logutil.WithFields(map[string]interface{}{
		"guildID":    guildID,
		"userID":     userID,
		"avatarHash": avatarHash,
	}).Info("Updated avatar in cache")
}

// AvatarHash retrieves the avatar hash for a user in a specific guild.
func (m *AvatarCacheManager) AvatarHash(guildID, userID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if cache := m.guilds[guildID]; cache != nil {
		logutil.WithFields(map[string]interface{}{
			"guildID": guildID,
			"userID":  userID,
		}).Debug("Retrieved avatar hash from cache")
		return cache.Avatars[userID]
	}
	logutil.WithFields(map[string]interface{}{
		"guildID": guildID,
		"userID":  userID,
	}).Warn("Avatar hash not found in cache")
	return ""
}

// Save saves the avatar cache to the configured file path.
func (m *AvatarCacheManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	multiCache := AvatarMultiGuildCache{
		Guilds:      m.guilds,
		LastUpdated: time.Now(),
		Version:     "2.0",
	}

	if err := m.jsonManager.Save(multiCache); err != nil {
		logutil.WithFields(map[string]interface{}{
			"path":  m.path,
			"error": err,
		}).Error("Failed to write cache file")
		return fmt.Errorf(ErrWriteAvatarCache, err)
	}

	logutil.WithField("path", m.path).Info("Cache saved successfully")
	return nil
}

// SaveThrottled performs coalesced persistence respecting the minimum interval.
func (m *AvatarCacheManager) SaveThrottled(minInterval time.Duration) error {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	if time.Since(m.lastSave) < minInterval {
		return nil
	}
	if err := m.Save(); err != nil {
		return err
	}
	m.lastSave = time.Now()
	return nil
}

func (m *AvatarCacheManager) AvatarChanged(guildID, userID, currentAvatarHash string) bool {
	return m.AvatarHash(guildID, userID) != currentAvatarHash
}

// ClearForGuild removes the cache of a specific guild
func (m *AvatarCacheManager) ClearForGuild(guildID string) error {
	m.mu.Lock()
	if _, exists := m.guilds[guildID]; !exists {
		m.mu.Unlock()
		logutil.WithField("guildID", guildID).Warn(WarnNoGuildCache)
		return nil
	}
	delete(m.guilds, guildID)
	m.mu.Unlock()
	return m.Save()
}

// GuildIDs returns a list of guilds that have cache
func (m *AvatarCacheManager) GuildIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.guilds))
	for guildID := range m.guilds {
		ids = append(ids, guildID)
	}
	return ids
}

// GetApplicationCacheFilePath returns the standardized path for application_cache.json
func GetApplicationCacheFilePath() string {
	return filepath.Join(ApplicationSupportPath, "data", "application_cache.json")
}
