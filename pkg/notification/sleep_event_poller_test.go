package notification

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockSleepEventFetcher implements SleepEventFetcher for testing
type MockSleepEventFetcher struct {
	events []SleepEvent
	err    error
	calls  int
}

func (m *MockSleepEventFetcher) TryFetchSleepEvents(babyUID string, limit int) ([]SleepEvent, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.events, nil
}

func TestSleepEventPollerConfig_Defaults(t *testing.T) {
	config := DefaultSleepEventPollerConfig()

	if config.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want 30s", config.PollInterval)
	}

	if config.Jitter != 0.2 {
		t.Errorf("Jitter = %v, want 0.2", config.Jitter)
	}

	if config.MaxBackoff != 5*time.Minute {
		t.Errorf("MaxBackoff = %v, want 5m", config.MaxBackoff)
	}

	if config.EventLimit != 20 {
		t.Errorf("EventLimit = %d, want 20", config.EventLimit)
	}

	if config.DedupMaxSize != 500 {
		t.Errorf("DedupMaxSize = %d, want 500", config.DedupMaxSize)
	}
}

func TestSleepEventPollerPoll_ReturnsNewEvents(t *testing.T) {
	now := time.Now()
	fetcher := &MockSleepEventFetcher{
		events: []SleepEvent{
			{Key: "FELL_ASLEEP", UID: "e1", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
			{Key: "WOKE_UP", UID: "e2", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
		},
	}

	config := DefaultSleepEventPollerConfig()
	poller := NewSleepEventPoller(config, fetcher)

	ctx := context.Background()
	events, err := poller.Poll(ctx, "baby1")

	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Poll() returned %d events, want 2", len(events))
	}
}

func TestSleepEventPollerPoll_DeduplicatesByUID(t *testing.T) {
	now := time.Now()
	fetcher := &MockSleepEventFetcher{
		events: []SleepEvent{
			{Key: "FELL_ASLEEP", UID: "e1", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
			{Key: "WOKE_UP", UID: "e2", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
		},
	}

	config := DefaultSleepEventPollerConfig()
	poller := NewSleepEventPoller(config, fetcher)

	ctx := context.Background()

	// First poll should return both events
	events1, _ := poller.Poll(ctx, "baby1")
	if len(events1) != 2 {
		t.Errorf("First poll returned %d events, want 2", len(events1))
	}

	// Second poll with same events should return 0 (deduplicated by UID)
	events2, _ := poller.Poll(ctx, "baby1")
	if len(events2) != 0 {
		t.Errorf("Second poll returned %d events, want 0 (deduplicated)", len(events2))
	}
}

func TestSleepEventPollerPoll_NewEventsAfterDedup(t *testing.T) {
	now := time.Now()
	fetcher := &MockSleepEventFetcher{
		events: []SleepEvent{
			{Key: "FELL_ASLEEP", UID: "e1", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
		},
	}

	config := DefaultSleepEventPollerConfig()
	poller := NewSleepEventPoller(config, fetcher)

	ctx := context.Background()

	// First poll
	events1, _ := poller.Poll(ctx, "baby1")
	if len(events1) != 1 {
		t.Errorf("First poll returned %d events, want 1", len(events1))
	}

	// Add a new event
	fetcher.events = []SleepEvent{
		{Key: "FELL_ASLEEP", UID: "e1", TimeRaw: float64(now.Unix()), BabyUID: "baby1"}, // Old
		{Key: "WOKE_UP", UID: "e2", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},     // New
	}

	// Second poll should only return the new event
	events2, _ := poller.Poll(ctx, "baby1")
	if len(events2) != 1 {
		t.Errorf("Second poll returned %d events, want 1", len(events2))
	}

	if events2[0].UID != "e2" {
		t.Errorf("Second poll returned event UID %s, want e2", events2[0].UID)
	}
}

func TestSleepEventPollerPoll_ContextCancellation(t *testing.T) {
	fetcher := &MockSleepEventFetcher{
		events: nil,
	}

	config := DefaultSleepEventPollerConfig()
	poller := NewSleepEventPoller(config, fetcher)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := poller.Poll(ctx, "baby1")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Poll() error = %v, want context.Canceled", err)
	}
}

func TestSleepEventPollerPoll_UpdatesStateTracker(t *testing.T) {
	now := time.Now()
	fetcher := &MockSleepEventFetcher{
		events: []SleepEvent{
			{Key: SleepEventKeyFellAsleep, UID: "e1", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
		},
	}

	config := DefaultSleepEventPollerConfig()
	poller := NewSleepEventPoller(config, fetcher)

	// Set up a state tracker
	tracker := NewSleepStateTracker()
	poller.SetStateTracker(tracker)

	ctx := context.Background()
	_, err := poller.Poll(ctx, "baby1")

	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}

	// State tracker should now show asleep
	if !tracker.IsAsleep() {
		t.Error("Expected IsAsleep() to be true after FELL_ASLEEP event")
	}
	if !tracker.InBed() {
		t.Error("Expected InBed() to be true after FELL_ASLEEP event")
	}
}

func TestSleepEventPollerPoll_FiltersOnlySleepEvents(t *testing.T) {
	now := time.Now()
	fetcher := &MockSleepEventFetcher{
		events: []SleepEvent{
			{Key: SleepEventKeyFellAsleep, UID: "e1", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
			{Key: "SOME_OTHER_EVENT", UID: "e2", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
			{Key: SleepEventKeyWokeUp, UID: "e3", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
		},
	}

	config := DefaultSleepEventPollerConfig()
	config.FilterSleepEventsOnly = true
	poller := NewSleepEventPoller(config, fetcher)

	ctx := context.Background()
	events, _ := poller.Poll(ctx, "baby1")

	// Should only return the sleep-related events
	if len(events) != 2 {
		t.Errorf("Poll() returned %d events, want 2 (filtered)", len(events))
	}
}

func TestSleepEventPollerBackoff_Exponential(t *testing.T) {
	config := DefaultSleepEventPollerConfig()
	config.PollInterval = 30 * time.Second
	config.MaxBackoff = 5 * time.Minute
	config.Jitter = 0 // Disable jitter for deterministic test

	poller := NewSleepEventPoller(config, nil)

	// Initial backoff should be 0
	if poller.CurrentBackoff() != 0 {
		t.Errorf("Initial backoff = %v, want 0", poller.CurrentBackoff())
	}

	// After 1 error: 2^1 * 30s = 60s
	poller.IncrementBackoff()
	expected := 60 * time.Second
	if poller.CurrentBackoff() != expected {
		t.Errorf("Backoff after 1 error = %v, want %v", poller.CurrentBackoff(), expected)
	}

	// After 2 errors: 2^2 * 30s = 120s
	poller.IncrementBackoff()
	expected = 120 * time.Second
	if poller.CurrentBackoff() != expected {
		t.Errorf("Backoff after 2 errors = %v, want %v", poller.CurrentBackoff(), expected)
	}
}

func TestSleepEventPollerBackoff_CappedAtMax(t *testing.T) {
	config := DefaultSleepEventPollerConfig()
	config.PollInterval = 30 * time.Second
	config.MaxBackoff = 60 * time.Second // Cap at 1 minute
	config.Jitter = 0

	poller := NewSleepEventPoller(config, nil)

	// Increment many times
	for i := 0; i < 10; i++ {
		poller.IncrementBackoff()
	}

	// Should be capped at MaxBackoff
	if poller.CurrentBackoff() > config.MaxBackoff {
		t.Errorf("Backoff = %v, should be capped at %v", poller.CurrentBackoff(), config.MaxBackoff)
	}
}

func TestSleepEventPollerBackoff_ResetOnSuccess(t *testing.T) {
	config := DefaultSleepEventPollerConfig()
	config.Jitter = 0

	poller := NewSleepEventPoller(config, nil)

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

func TestSleepEventPollerStats(t *testing.T) {
	now := time.Now()
	fetcher := &MockSleepEventFetcher{
		events: []SleepEvent{
			{Key: "FELL_ASLEEP", UID: "e1", TimeRaw: float64(now.Unix()), BabyUID: "baby1"},
		},
	}

	config := DefaultSleepEventPollerConfig()
	poller := NewSleepEventPoller(config, fetcher)

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

	if stats.EventsReceived != 1 {
		t.Errorf("EventsReceived = %d, want 1", stats.EventsReceived)
	}
}

func TestSleepEventPollerNextInterval_WithJitter(t *testing.T) {
	config := DefaultSleepEventPollerConfig()
	config.Jitter = 0.2 // ±20%
	config.PollInterval = 30 * time.Second

	poller := NewSleepEventPoller(config, nil)

	// Run multiple times to check jitter range
	minExpected := 24 * time.Second // 30 * 0.8
	maxExpected := 36 * time.Second // 30 * 1.2

	for i := 0; i < 100; i++ {
		interval := poller.NextInterval()
		if interval < minExpected || interval > maxExpected {
			t.Errorf("NextInterval() = %v, want between %v and %v", interval, minExpected, maxExpected)
		}
	}
}

func TestSleepEventPoller_FetcherReturnsError(t *testing.T) {
	fetcher := &MockSleepEventFetcher{
		err: errors.New("network error"),
	}

	config := DefaultSleepEventPollerConfig()
	poller := NewSleepEventPoller(config, fetcher)

	ctx := context.Background()
	events, err := poller.Poll(ctx, "baby1")

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if events != nil {
		t.Errorf("Expected nil events on error, got %v", events)
	}
}
