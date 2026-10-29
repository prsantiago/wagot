package bot

import (
	"sync"
	"time"
)

// dedup remembers message IDs for a TTL. Meta delivers webhooks at least
// once, so the same message can arrive several times.
//
// It is in-memory: duplicates are not detected across restarts or between
// replicas. Use a shared store (for example Redis SET NX) if you need that.
type dedup struct {
	mu        sync.Mutex
	ttl       time.Duration
	seen      map[string]time.Time
	lastPrune time.Time
	now       func() time.Time
}

func newDedup(ttl time.Duration) *dedup {
	return &dedup{ttl: ttl, seen: make(map[string]time.Time), now: time.Now}
}

// add records id and reports whether it was new.
func (d *dedup) add(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := d.now()
	if now.Sub(d.lastPrune) > d.ttl {
		for k, t := range d.seen {
			if now.Sub(t) > d.ttl {
				delete(d.seen, k)
			}
		}
		d.lastPrune = now
	}

	if t, ok := d.seen[id]; ok && now.Sub(t) <= d.ttl {
		return false
	}
	d.seen[id] = now
	return true
}

// remove forgets id.
func (d *dedup) remove(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.seen, id)
}
