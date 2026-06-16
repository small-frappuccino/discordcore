package monitoring

import (
	"sync"
)

type presenceSnapshot struct {
	Status       string
	ClientStatus clientStatus
}

// presenceWatcher records the last presence snapshot seen for each watched
// user and reports whether a newly observed snapshot differs from it. Access
// is serialized by mu. The zero value is ready to use: the snapshot map is
// created lazily on first write.
type presenceWatcher struct {
	mu        sync.Mutex
	snapshots map[string]presenceSnapshot
}

// observe compares snap against the snapshot previously stored for userID.
// When an identical snapshot is already stored it reports changed=false and
// leaves the store untouched. Otherwise it stores snap and returns the prior
// snapshot via prev/hasPrev so callers can render the transition.
func (w *presenceWatcher) observe(userID string, snap presenceSnapshot) (prev presenceSnapshot, hasPrev, changed bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	prev, hasPrev = w.snapshots[userID]
	if hasPrev && presenceSnapshotEqual(prev, snap) {
		return prev, true, false
	}
	if w.snapshots == nil {
		w.snapshots = make(map[string]presenceSnapshot)
	}
	w.snapshots[userID] = snap
	return prev, hasPrev, true
}
