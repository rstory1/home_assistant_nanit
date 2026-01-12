package notification

import (
	"sync"
)

// Deduplicator tracks seen message IDs to prevent reprocessing
type Deduplicator struct {
	seen    map[int]int64 // message ID -> order (for eviction)
	order   int64         // monotonically increasing counter
	maxSize int
	mu      sync.RWMutex
}

// NewDeduplicator creates a new Deduplicator with the given maximum size
// When maxSize is exceeded, the oldest entries are evicted
// If maxSize is 0, no eviction occurs (unlimited)
func NewDeduplicator(maxSize int) *Deduplicator {
	return &Deduplicator{
		seen:    make(map[int]int64),
		maxSize: maxSize,
	}
}

// IsSeen returns true if the message ID was already processed
func (d *Deduplicator) IsSeen(messageID int) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	_, exists := d.seen[messageID]
	return exists
}

// MarkSeen marks a message ID as processed
func (d *Deduplicator) MarkSeen(messageID int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// If already seen, just update the order
	if _, exists := d.seen[messageID]; exists {
		return
	}

	// Add new entry
	d.order++
	d.seen[messageID] = d.order

	// Evict old entries if needed
	d.evictIfNeeded()
}

// evictIfNeeded removes old entries if we've exceeded maxSize
// Must be called with lock held
func (d *Deduplicator) evictIfNeeded() {
	// No eviction if maxSize is 0 (unlimited) or we're under the limit
	if d.maxSize <= 0 || len(d.seen) <= d.maxSize {
		return
	}

	// Find and remove the oldest entry (O(n) instead of O(n log n) sort)
	// Since we typically only need to evict one entry at a time,
	// finding the minimum is more efficient than sorting
	for len(d.seen) > d.maxSize {
		var oldestID int
		var oldestOrder int64 = -1

		for id, order := range d.seen {
			if oldestOrder == -1 || order < oldestOrder {
				oldestID = id
				oldestOrder = order
			}
		}

		if oldestOrder != -1 {
			delete(d.seen, oldestID)
		} else {
			// Safety: shouldn't happen, but avoid infinite loop
			break
		}
	}
}

// Size returns the number of tracked message IDs
func (d *Deduplicator) Size() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.seen)
}

// Clear removes all tracked message IDs
func (d *Deduplicator) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.seen = make(map[int]int64)
	d.order = 0
}
