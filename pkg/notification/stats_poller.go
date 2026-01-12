package notification

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"
)

// StatsFetcher interface for fetching sleep stats from the API
type StatsFetcher interface {
	TryFetchSleepStats(babyUID string) (*SleepStats, error)
}

// StatsPollerConfig configures the stats poller
type StatsPollerConfig struct {
	// PollInterval is the base interval between polls (default 5m)
	PollInterval time.Duration

	// Jitter is the jitter factor 0-1, e.g. 0.1 = ±10% (default 0.1)
	Jitter float64

	// MaxBackoff is the maximum backoff duration (default 15m)
	MaxBackoff time.Duration
}

// DefaultStatsPollerConfig returns the default stats poller configuration
func DefaultStatsPollerConfig() StatsPollerConfig {
	return StatsPollerConfig{
		PollInterval: 5 * time.Minute,
		Jitter:       0.1,
		MaxBackoff:   15 * time.Minute,
	}
}

// StatsPollerStats contains statistics about stats poller operation
type StatsPollerStats struct {
	PollCount  int64
	ErrorCount int64
	StartedAt  time.Time
}

// StatsPoller polls for sleep statistics from the Nanit /stats/latest API
type StatsPoller struct {
	config       StatsPollerConfig
	fetcher      StatsFetcher
	stateTracker *SleepStateTracker

	mu           sync.RWMutex
	errorCount   int
	backoff      time.Duration
	stats        StatsPollerStats
	lastPollAt   time.Time
	lastStats    *SleepStats // Last fetched stats for change detection
}

// NewStatsPoller creates a new stats poller
func NewStatsPoller(config StatsPollerConfig, fetcher StatsFetcher) *StatsPoller {
	return &StatsPoller{
		config:  config,
		fetcher: fetcher,
		stats: StatsPollerStats{
			StartedAt: time.Now(),
		},
	}
}

// SetStateTracker sets the sleep state tracker to update when stats are received
func (p *StatsPoller) SetStateTracker(tracker *SleepStateTracker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stateTracker = tracker
}

// Poll fetches sleep stats for a baby
func (p *StatsPoller) Poll(ctx context.Context, babyUID string) (*SleepStats, error) {
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

	// Fetch stats
	stats, err := p.fetcher.TryFetchSleepStats(babyUID)
	if err != nil {
		return nil, err
	}

	if stats == nil {
		return nil, nil
	}

	// Update state tracker if set
	p.mu.RLock()
	tracker := p.stateTracker
	p.mu.RUnlock()
	if tracker != nil {
		tracker.UpdateFromStats(stats)
	}

	return stats, nil
}

// HasChanged checks if the provided stats differ from the last fetched stats
// This is useful to only publish when values have actually changed
func (p *StatsPoller) HasChanged(current *SleepStats) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.lastStats == nil {
		// First poll - consider changed
		p.lastStats = current
		return true
	}

	if current == nil {
		// No current stats - check if we had stats before
		hadStats := p.lastStats != nil
		p.lastStats = nil
		return hadStats
	}

	// Compare relevant fields
	changed := p.lastStats.TimesWokeUp != current.TimesWokeUp ||
		p.lastStats.SleepInterventions != current.SleepInterventions ||
		p.lastStats.TotalAwakeTime != current.TotalAwakeTime ||
		p.lastStats.TotalSleepTime != current.TotalSleepTime ||
		p.lastStats.CurrentState() != current.CurrentState()

	if changed {
		p.lastStats = current
	}

	return changed
}

// LastStats returns the last fetched stats
func (p *StatsPoller) LastStats() *SleepStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastStats
}

// NextInterval returns the next poll interval with jitter applied
func (p *StatsPoller) NextInterval() time.Duration {
	p.mu.RLock()
	backoff := p.backoff
	p.mu.RUnlock()

	base := p.config.PollInterval + backoff
	return p.applyJitter(base)
}

// applyJitter applies jitter to a duration
func (p *StatsPoller) applyJitter(d time.Duration) time.Duration {
	if p.config.Jitter <= 0 {
		return d
	}

	// Random factor between -jitter and +jitter
	jitterFactor := (rand.Float64()*2 - 1) * p.config.Jitter
	multiplier := 1.0 + jitterFactor

	return time.Duration(float64(d) * multiplier)
}

// CurrentBackoff returns the current backoff duration
func (p *StatsPoller) CurrentBackoff() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.backoff
}

// IncrementBackoff increases the backoff after an error
func (p *StatsPoller) IncrementBackoff() {
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
func (p *StatsPoller) ResetBackoff() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.errorCount = 0
	p.backoff = 0
}

// ErrorCount returns the current consecutive error count
func (p *StatsPoller) ErrorCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.errorCount
}

// Stats returns current poller statistics
func (p *StatsPoller) Stats() StatsPollerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// LastPollAt returns when the last poll occurred
func (p *StatsPoller) LastPollAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastPollAt
}

// StateTracker returns the current state tracker (if set)
func (p *StatsPoller) StateTracker() *SleepStateTracker {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stateTracker
}
