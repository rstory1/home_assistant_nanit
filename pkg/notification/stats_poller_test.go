package notification

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockStatsFetcher implements StatsFetcher for testing
type MockStatsFetcher struct {
	stats *SleepStats
	err   error
	calls int
}

func (m *MockStatsFetcher) TryFetchSleepStats(babyUID string) (*SleepStats, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.stats, nil
}

func TestStatsPollerConfig_Defaults(t *testing.T) {
	config := DefaultStatsPollerConfig()

	if config.PollInterval != 5*time.Minute {
		t.Errorf("PollInterval = %v, want 5m", config.PollInterval)
	}

	if config.Jitter != 0.1 {
		t.Errorf("Jitter = %v, want 0.1", config.Jitter)
	}

	if config.MaxBackoff != 15*time.Minute {
		t.Errorf("MaxBackoff = %v, want 15m", config.MaxBackoff)
	}
}

func TestStatsPollerPoll_ReturnsStats(t *testing.T) {
	fetcher := &MockStatsFetcher{
		stats: &SleepStats{
			Valid:              true,
			TimesWokeUp:        3,
			SleepInterventions: 1,
			TotalAwakeTime:     1200,
			TotalSleepTime:     3600,
			States: []SleepState{
				{Title: SleepStateAsleep, BeginTS: 100, EndTS: 200},
			},
		},
	}

	config := DefaultStatsPollerConfig()
	poller := NewStatsPoller(config, fetcher)

	ctx := context.Background()
	stats, err := poller.Poll(ctx, "baby1")

	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}

	if stats == nil {
		t.Fatal("Poll() returned nil stats")
	}

	if stats.TimesWokeUp != 3 {
		t.Errorf("TimesWokeUp = %d, want 3", stats.TimesWokeUp)
	}

	if stats.SleepInterventions != 1 {
		t.Errorf("SleepInterventions = %d, want 1", stats.SleepInterventions)
	}
}

func TestStatsPollerPoll_ReturnsNilWhenNoStats(t *testing.T) {
	fetcher := &MockStatsFetcher{
		stats: nil, // No stats available
	}

	config := DefaultStatsPollerConfig()
	poller := NewStatsPoller(config, fetcher)

	ctx := context.Background()
	stats, err := poller.Poll(ctx, "baby1")

	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}

	if stats != nil {
		t.Error("Expected nil stats when none available")
	}
}

func TestStatsPollerPoll_ContextCancellation(t *testing.T) {
	fetcher := &MockStatsFetcher{
		stats: nil,
	}

	config := DefaultStatsPollerConfig()
	poller := NewStatsPoller(config, fetcher)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := poller.Poll(ctx, "baby1")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Poll() error = %v, want context.Canceled", err)
	}
}

func TestStatsPollerPoll_UpdatesStateTracker(t *testing.T) {
	fetcher := &MockStatsFetcher{
		stats: &SleepStats{
			Valid: true,
			States: []SleepState{
				{Title: SleepStateAsleep, BeginTS: 100, EndTS: 0}, // Ongoing (no end)
			},
		},
	}

	config := DefaultStatsPollerConfig()
	poller := NewStatsPoller(config, fetcher)

	// Set up a state tracker
	tracker := NewSleepStateTracker()
	poller.SetStateTracker(tracker)

	ctx := context.Background()
	_, err := poller.Poll(ctx, "baby1")

	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}

	// State tracker should now show asleep and in bed
	if !tracker.IsAsleep() {
		t.Error("Expected IsAsleep() to be true when current state is ASLEEP")
	}
	if !tracker.InBed() {
		t.Error("Expected InBed() to be true when current state is ASLEEP")
	}
}

func TestStatsPollerPoll_DetectsChanges(t *testing.T) {
	stats1 := &SleepStats{
		Valid:       true,
		TimesWokeUp: 1,
	}

	fetcher := &MockStatsFetcher{stats: stats1}

	config := DefaultStatsPollerConfig()
	poller := NewStatsPoller(config, fetcher)

	ctx := context.Background()

	// First poll
	result1, _ := poller.Poll(ctx, "baby1")
	if result1.TimesWokeUp != 1 {
		t.Errorf("First poll TimesWokeUp = %d, want 1", result1.TimesWokeUp)
	}

	// Check if change is detected
	changed := poller.HasChanged(result1)
	if !changed {
		t.Error("Expected HasChanged() to be true for first poll")
	}

	// Second poll with same data
	result2, _ := poller.Poll(ctx, "baby1")
	changed = poller.HasChanged(result2)
	if changed {
		t.Error("Expected HasChanged() to be false for same data")
	}

	// Third poll with different data
	stats2 := &SleepStats{
		Valid:       true,
		TimesWokeUp: 2,
	}
	fetcher.stats = stats2
	result3, _ := poller.Poll(ctx, "baby1")
	changed = poller.HasChanged(result3)
	if !changed {
		t.Error("Expected HasChanged() to be true for changed data")
	}
}

func TestStatsPollerBackoff_Exponential(t *testing.T) {
	config := DefaultStatsPollerConfig()
	config.PollInterval = 5 * time.Minute
	config.MaxBackoff = 30 * time.Minute
	config.Jitter = 0 // Disable jitter for deterministic test

	poller := NewStatsPoller(config, nil)

	// Initial backoff should be 0
	if poller.CurrentBackoff() != 0 {
		t.Errorf("Initial backoff = %v, want 0", poller.CurrentBackoff())
	}

	// After 1 error: 2^1 * 5m = 10m
	poller.IncrementBackoff()
	expected := 10 * time.Minute
	if poller.CurrentBackoff() != expected {
		t.Errorf("Backoff after 1 error = %v, want %v", poller.CurrentBackoff(), expected)
	}
}

func TestStatsPollerBackoff_CappedAtMax(t *testing.T) {
	config := DefaultStatsPollerConfig()
	config.PollInterval = 5 * time.Minute
	config.MaxBackoff = 15 * time.Minute
	config.Jitter = 0

	poller := NewStatsPoller(config, nil)

	// Increment many times
	for i := 0; i < 10; i++ {
		poller.IncrementBackoff()
	}

	// Should be capped at MaxBackoff
	if poller.CurrentBackoff() > config.MaxBackoff {
		t.Errorf("Backoff = %v, should be capped at %v", poller.CurrentBackoff(), config.MaxBackoff)
	}
}

func TestStatsPollerBackoff_ResetOnSuccess(t *testing.T) {
	config := DefaultStatsPollerConfig()
	config.Jitter = 0

	poller := NewStatsPoller(config, nil)

	// Build up some backoff
	poller.IncrementBackoff()
	poller.IncrementBackoff()

	if poller.CurrentBackoff() == 0 {
		t.Error("Backoff should be non-zero after errors")
	}

	// Reset
	poller.ResetBackoff()

	if poller.CurrentBackoff() != 0 {
		t.Errorf("Backoff = %v, want 0 after reset", poller.CurrentBackoff())
	}
}

func TestStatsPollerStats(t *testing.T) {
	fetcher := &MockStatsFetcher{
		stats: &SleepStats{Valid: true, TimesWokeUp: 1},
	}

	config := DefaultStatsPollerConfig()
	poller := NewStatsPoller(config, fetcher)

	ctx := context.Background()

	// Initial stats
	stats := poller.Stats()
	if stats.PollCount != 0 {
		t.Errorf("Initial PollCount = %d, want 0", stats.PollCount)
	}

	// After a poll
	poller.Poll(ctx, "baby1")
	stats = poller.Stats()

	if stats.PollCount != 1 {
		t.Errorf("PollCount = %d, want 1", stats.PollCount)
	}
}

func TestStatsPollerNextInterval_WithJitter(t *testing.T) {
	config := DefaultStatsPollerConfig()
	config.Jitter = 0.1 // ±10%
	config.PollInterval = 5 * time.Minute

	poller := NewStatsPoller(config, nil)

	// Run multiple times to check jitter range
	minExpected := time.Duration(float64(5*time.Minute) * 0.9) // 4.5 min
	maxExpected := time.Duration(float64(5*time.Minute) * 1.1) // 5.5 min

	for i := 0; i < 100; i++ {
		interval := poller.NextInterval()
		if interval < minExpected || interval > maxExpected {
			t.Errorf("NextInterval() = %v, want between %v and %v", interval, minExpected, maxExpected)
		}
	}
}

func TestStatsPoller_FetcherReturnsError(t *testing.T) {
	fetcher := &MockStatsFetcher{
		err: errors.New("network error"),
	}

	config := DefaultStatsPollerConfig()
	poller := NewStatsPoller(config, fetcher)

	ctx := context.Background()
	stats, err := poller.Poll(ctx, "baby1")

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if stats != nil {
		t.Errorf("Expected nil stats on error, got %v", stats)
	}
}
