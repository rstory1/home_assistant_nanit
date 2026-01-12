package notification

import (
	"sync"
	"testing"
	"time"
)

func TestSleepStateTrackerInitialState(t *testing.T) {
	tracker := NewSleepStateTracker()

	if tracker.IsAsleep() {
		t.Error("Expected IsAsleep() to be false initially")
	}
	if tracker.InBed() {
		t.Error("Expected InBed() to be false initially")
	}
	if tracker.LastEvent() != "" {
		t.Errorf("Expected LastEvent() to be empty, got %q", tracker.LastEvent())
	}
	if !tracker.LastEventTime().IsZero() {
		t.Error("Expected LastEventTime() to be zero initially")
	}
}

func TestSleepStateTrackerFellAsleep(t *testing.T) {
	tracker := NewSleepStateTracker()
	eventTime := time.Now()

	event := SleepEvent{
		Key:     SleepEventKeyFellAsleep,
		TimeRaw: float64(eventTime.Unix()),
	}

	changed := tracker.ProcessEvent(event)

	if !changed {
		t.Error("Expected ProcessEvent to return true for state change")
	}
	if !tracker.IsAsleep() {
		t.Error("Expected IsAsleep() to be true after FELL_ASLEEP")
	}
	if tracker.LastEvent() != SleepEventKeyFellAsleep {
		t.Errorf("LastEvent() = %q, want %q", tracker.LastEvent(), SleepEventKeyFellAsleep)
	}
}

func TestSleepStateTrackerWokeUp(t *testing.T) {
	tracker := NewSleepStateTracker()

	// First set asleep
	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyFellAsleep,
		TimeRaw: float64(time.Now().Unix()),
	})

	// Then wake up
	changed := tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyWokeUp,
		TimeRaw: float64(time.Now().Unix()),
	})

	if !changed {
		t.Error("Expected ProcessEvent to return true for state change")
	}
	if tracker.IsAsleep() {
		t.Error("Expected IsAsleep() to be false after WOKE_UP")
	}
	if tracker.LastEvent() != SleepEventKeyWokeUp {
		t.Errorf("LastEvent() = %q, want %q", tracker.LastEvent(), SleepEventKeyWokeUp)
	}
}

func TestSleepStateTrackerPutInBed(t *testing.T) {
	tracker := NewSleepStateTracker()

	event := SleepEvent{
		Key:     SleepEventKeyPutInBed,
		TimeRaw: float64(time.Now().Unix()),
	}

	changed := tracker.ProcessEvent(event)

	if !changed {
		t.Error("Expected ProcessEvent to return true for state change")
	}
	if !tracker.InBed() {
		t.Error("Expected InBed() to be true after PUT_IN_BED")
	}
}

func TestSleepStateTrackerRemoved(t *testing.T) {
	tracker := NewSleepStateTracker()

	// First put in bed
	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyPutInBed,
		TimeRaw: float64(time.Now().Unix()),
	})

	// Then remove
	changed := tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyRemoved,
		TimeRaw: float64(time.Now().Unix()),
	})

	if !changed {
		t.Error("Expected ProcessEvent to return true for state change")
	}
	if tracker.InBed() {
		t.Error("Expected InBed() to be false after REMOVED")
	}
}

func TestSleepStateTrackerRemovedAsleep(t *testing.T) {
	tracker := NewSleepStateTracker()

	// First put in bed and asleep
	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyPutInBed,
		TimeRaw: float64(time.Now().Unix()),
	})
	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyFellAsleep,
		TimeRaw: float64(time.Now().Unix()),
	})

	// Then remove while asleep
	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyRemovedAsleep,
		TimeRaw: float64(time.Now().Unix()),
	})

	if tracker.InBed() {
		t.Error("Expected InBed() to be false after REMOVED_ASLEEP")
	}
	// Note: IsAsleep might still be true if parent removes sleeping baby
}

func TestSleepStateTrackerNoChangeOnDuplicateEvent(t *testing.T) {
	tracker := NewSleepStateTracker()

	// First FELL_ASLEEP
	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyFellAsleep,
		TimeRaw: float64(time.Now().Unix()),
	})

	// Second FELL_ASLEEP - no change
	changed := tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyFellAsleep,
		TimeRaw: float64(time.Now().Unix()),
	})

	if changed {
		t.Error("Expected ProcessEvent to return false for duplicate state")
	}
}

func TestSleepStateTrackerVisitNoStateChange(t *testing.T) {
	tracker := NewSleepStateTracker()

	// Set initial state
	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyFellAsleep,
		TimeRaw: float64(time.Now().Unix()),
	})

	wasAsleep := tracker.IsAsleep()

	// VISIT should not change sleep state
	changed := tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyVisit,
		TimeRaw: float64(time.Now().Unix()),
	})

	// VISIT updates last event but doesn't change sleep/bed state
	if tracker.IsAsleep() != wasAsleep {
		t.Error("VISIT should not change IsAsleep state")
	}
	if tracker.LastEvent() != SleepEventKeyVisit {
		t.Errorf("LastEvent() = %q, want %q", tracker.LastEvent(), SleepEventKeyVisit)
	}
	// changed should be false since sleep/bed state didn't change
	if changed {
		t.Error("Expected ProcessEvent to return false for VISIT (no state change)")
	}
}

func TestSleepStateTrackerConcurrentAccess(t *testing.T) {
	tracker := NewSleepStateTracker()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writes
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tracker.ProcessEvent(SleepEvent{
				Key:     SleepEventKeyFellAsleep,
				TimeRaw: float64(time.Now().Unix()),
			})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tracker.ProcessEvent(SleepEvent{
				Key:     SleepEventKeyWokeUp,
				TimeRaw: float64(time.Now().Unix()),
			})
		}
	}()

	// Concurrent reads
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = tracker.IsAsleep()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = tracker.InBed()
		}
	}()

	wg.Wait()
	// No panic = success
}

func TestSleepStateTrackerUpdateFromStats(t *testing.T) {
	tracker := NewSleepStateTracker()

	// Update from stats where baby is asleep and in bed
	stats := &SleepStats{
		States: []SleepState{
			{Title: SleepStateAwake, BeginTS: 100, EndTS: 200},
			{Title: SleepStateAsleep, BeginTS: 200, EndTS: 300},
		},
	}

	changed := tracker.UpdateFromStats(stats)

	if !changed {
		t.Error("Expected UpdateFromStats to return true for state change")
	}
	if !tracker.IsAsleep() {
		t.Error("Expected IsAsleep() to be true from stats")
	}
	if !tracker.InBed() {
		t.Error("Expected InBed() to be true from stats (ASLEEP implies in bed)")
	}
}

func TestSleepStateTrackerUpdateFromStatsAbsent(t *testing.T) {
	tracker := NewSleepStateTracker()

	// First set in bed
	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyPutInBed,
		TimeRaw: float64(time.Now().Unix()),
	})

	// Update from stats where baby is absent
	stats := &SleepStats{
		States: []SleepState{
			{Title: SleepStateAsleep, BeginTS: 100, EndTS: 200},
			{Title: SleepStateAbsent, BeginTS: 200, EndTS: 300},
		},
	}

	tracker.UpdateFromStats(stats)

	if tracker.IsAsleep() {
		t.Error("Expected IsAsleep() to be false when ABSENT")
	}
	if tracker.InBed() {
		t.Error("Expected InBed() to be false when ABSENT")
	}
}

func TestSleepStateTrackerGetState(t *testing.T) {
	tracker := NewSleepStateTracker()

	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyPutInBed,
		TimeRaw: float64(time.Now().Unix()),
	})
	tracker.ProcessEvent(SleepEvent{
		Key:     SleepEventKeyFellAsleep,
		TimeRaw: float64(time.Now().Unix()),
	})

	state := tracker.GetState()

	if !state.IsAsleep {
		t.Error("Expected state.IsAsleep to be true")
	}
	if !state.InBed {
		t.Error("Expected state.InBed to be true")
	}
	if state.LastEvent != SleepEventKeyFellAsleep {
		t.Errorf("state.LastEvent = %q, want %q", state.LastEvent, SleepEventKeyFellAsleep)
	}
}
