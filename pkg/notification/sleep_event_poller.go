package notification

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"
)

// SleepEventFetcher interface for fetching sleep events from the API
type SleepEventFetcher interface {
	TryFetchSleepEvents(babyUID string, limit int) ([]SleepEvent, error)
}

// SleepEventPollerConfig configures the sleep event poller
type SleepEventPollerConfig struct {
	// PollInterval is the base interval between polls (default 30s)
	PollInterval time.Duration

	// Jitter is the jitter factor 0-1, e.g. 0.2 = ±20% (default 0.2)
	Jitter float64

	// MaxBackoff is the maximum backoff duration (default 5m)
	MaxBackoff time.Duration

	// EventLimit is the number of events to fetch per poll (default 20)
	EventLimit int

	// DedupMaxSize is the maximum number of event UIDs to track for deduplication (default 500)
	DedupMaxSize int

	// FilterSleepEventsOnly when true, only returns sleep-related events (default false)
	FilterSleepEventsOnly bool
}

// DefaultSleepEventPollerConfig returns the default sleep event poller configuration
func DefaultSleepEventPollerConfig() SleepEventPollerConfig {
	return SleepEventPollerConfig{
		PollInterval:          30 * time.Second,
		Jitter:                0.2,
		MaxBackoff:            5 * time.Minute,
		EventLimit:            20,
		DedupMaxSize:          500,
		FilterSleepEventsOnly: false,
	}
}

// SleepEventPollerStats contains statistics about sleep event poller operation
type SleepEventPollerStats struct {
	PollCount      int64
	EventsReceived int64
	ErrorCount     int64
	StartedAt      time.Time
}

// SleepEventPoller polls for sleep events from the Nanit /events API
type SleepEventPoller struct {
	config       SleepEventPollerConfig
	fetcher      SleepEventFetcher
	stateTracker *SleepStateTracker
	seenUIDs     map[string]time.Time // UID -> time seen
	seenOrder    []string             // UIDs in order they were seen (for eviction)

	mu          sync.RWMutex
	errorCount  int
	backoff     time.Duration
	stats       SleepEventPollerStats
	lastPollAt  time.Time
}

// NewSleepEventPoller creates a new sleep event poller
func NewSleepEventPoller(config SleepEventPollerConfig, fetcher SleepEventFetcher) *SleepEventPoller {
	return &SleepEventPoller{
		config:    config,
		fetcher:   fetcher,
		seenUIDs:  make(map[string]time.Time),
		seenOrder: make([]string, 0),
		stats: SleepEventPollerStats{
			StartedAt: time.Now(),
		},
	}
}

// SetStateTracker sets the sleep state tracker to update when events are received
func (p *SleepEventPoller) SetStateTracker(tracker *SleepStateTracker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stateTracker = tracker
}

// Poll fetches new sleep events for a baby, applying deduplication
func (p *SleepEventPoller) Poll(ctx context.Context, babyUID string) ([]SleepEvent, error) {
	// Check context first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	p.stats.PollCount++
	p.lastPollAt = time.Now()
	p.mu.Unlock()

	// Fetch events
	events, err := p.fetcher.TryFetchSleepEvents(babyUID, p.config.EventLimit)
	if err != nil {
		return nil, err
	}

	// Filter, deduplicate, and process events
	var newEvents []SleepEvent
	for _, event := range events {
		// Skip already-seen events
		if p.isSeen(event.UID) {
			continue
		}

		// Optionally filter to only sleep-related events
		if p.config.FilterSleepEventsOnly && !event.IsSleepRelated() && !event.IsBedRelated() {
			continue
		}

		// Mark as seen
		p.markSeen(event.UID)

		// Update state tracker if set
		p.mu.RLock()
		tracker := p.stateTracker
		p.mu.RUnlock()
		if tracker != nil {
			tracker.ProcessEvent(event)
		}

		newEvents = append(newEvents, event)
	}

	// Update stats
	p.mu.Lock()
	p.stats.EventsReceived += int64(len(newEvents))
	p.mu.Unlock()

	return newEvents, nil
}

// isSeen checks if an event UID has been seen
func (p *SleepEventPoller) isSeen(uid string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.seenUIDs[uid]
	return ok
}

// markSeen marks an event UID as seen
func (p *SleepEventPoller) markSeen(uid string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if already seen
	if _, ok := p.seenUIDs[uid]; ok {
		return
	}

	// Add to tracking
	p.seenUIDs[uid] = time.Now()
	p.seenOrder = append(p.seenOrder, uid)

	// Evict old entries if over limit
	for len(p.seenOrder) > p.config.DedupMaxSize {
		oldUID := p.seenOrder[0]
		delete(p.seenUIDs, oldUID)
		p.seenOrder = p.seenOrder[1:]
	}
}

// NextInterval returns the next poll interval with jitter applied
func (p *SleepEventPoller) NextInterval() time.Duration {
	p.mu.RLock()
	backoff := p.backoff
	p.mu.RUnlock()

	base := p.config.PollInterval + backoff
	return p.applyJitter(base)
}

// applyJitter applies jitter to a duration
func (p *SleepEventPoller) applyJitter(d time.Duration) time.Duration {
	if p.config.Jitter <= 0 {
		return d
	}

	// Random factor between -jitter and +jitter
	jitterFactor := (rand.Float64()*2 - 1) * p.config.Jitter
	multiplier := 1.0 + jitterFactor

	return time.Duration(float64(d) * multiplier)
}

// CurrentBackoff returns the current backoff duration
func (p *SleepEventPoller) CurrentBackoff() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.backoff
}

// IncrementBackoff increases the backoff after an error
func (p *SleepEventPoller) IncrementBackoff() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.errorCount++
	p.stats.ErrorCount++

	// Exponential backoff: 2^errorCount * PollInterval
	backoff := time.Duration(math.Pow(2, float64(p.errorCount))) * p.config.PollInterval

	// Cap at MaxBackoff
	if backoff > p.config.MaxBackoff {
		backoff = p.config.MaxBackoff
	}

	p.backoff = backoff
}

// ResetBackoff resets the backoff after a successful poll
func (p *SleepEventPoller) ResetBackoff() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.errorCount = 0
	p.backoff = 0
}

// ErrorCount returns the current consecutive error count
func (p *SleepEventPoller) ErrorCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.errorCount
}

// Stats returns current poller statistics
func (p *SleepEventPoller) Stats() SleepEventPollerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// LastPollAt returns when the last poll occurred
func (p *SleepEventPoller) LastPollAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastPollAt
}

// StateTracker returns the current state tracker (if set)
func (p *SleepEventPoller) StateTracker() *SleepStateTracker {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stateTracker
}
