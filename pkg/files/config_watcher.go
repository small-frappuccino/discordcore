package files

import (
	"time"
)

// ConfigWatcher is a callback invoked when the configuration changes.
type ConfigWatcher func(cfg *BotConfig)

const debounceInterval = 500 * time.Millisecond

// AddWatcher registers a new watcher to be notified of configuration changes.
func (mgr *ConfigManager) AddWatcher(w ConfigWatcher) {
	mgr.watcherMu.Lock()
	defer mgr.watcherMu.Unlock()
	mgr.watchers = append(mgr.watchers, w)
}

// notifyWatchers triggers the registered watchers with a debounce mechanism
// to prevent GC saturation from high-frequency updates.
func (mgr *ConfigManager) notifyWatchers() {
	mgr.debounceMu.Lock()
	defer mgr.debounceMu.Unlock()

	if mgr.debounceTimer != nil {
		mgr.debounceTimer.Stop()
	}

	mgr.debounceTimer = time.AfterFunc(debounceInterval, func() {
		mgr.watcherMu.Lock()
		watchers := make([]ConfigWatcher, len(mgr.watchers))
		copy(watchers, mgr.watchers)
		mgr.watcherMu.Unlock()

		if len(watchers) == 0 {
			return
		}

		// Use the thread-safe SnapshotConfig to get the latest state
		snap := mgr.SnapshotConfig()

		for _, w := range watchers {
			w(&snap)
		}
	})
}
