// Package notification provides types and utilities for handling Nanit camera notifications
package notification

import (
	"sync"
	"time"
)

// SleepStateSnapshot represents a point-in-time snapshot of sleep state
type SleepStateSnapshot struct {
	IsAsleep      bool
	InBed         bool
	LastEvent     string
	LastEventTime time.Time
}

// SleepStateTracker tracks the baby's sleep and bed state based on events
// Thread-safe for concurrent access
type SleepStateTracker struct {
	mu            sync.RWMutex
	isAsleep      bool
	inBed         bool
	lastEvent     string
	lastEventTime time.Time
}

// NewSleepStateTracker creates a new sleep state tracker with default values
func NewSleepStateTracker() *SleepStateTracker {
	return &SleepStateTracker{}
}

// ProcessEvent processes a sleep event and updates the internal state
// Returns true if the sleep or bed state changed
func (t *SleepStateTracker) ProcessEvent(event SleepEvent) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	prevAsleep := t.isAsleep
	prevInBed := t.inBed

	// Update last event info
	t.lastEvent = event.Key
	t.lastEventTime = event.Time()

	// Update sleep state based on event key
	switch event.Key {
	case SleepEventKeyFellAsleep:
		t.isAsleep = true
		// Falling asleep implies in bed
		t.inBed = true
	case SleepEventKeyWokeUp:
		t.isAsleep = false
		// Waking up still means in bed
		t.inBed = true
	case SleepEventKeyPutInBed, SleepEventKeyPutToSleep:
		t.inBed = true
	case SleepEventKeyRemoved, SleepEventKeyRemovedAsleep:
		t.inBed = false
		// Note: REMOVED_ASLEEP doesn't necessarily mean awake
	}

	// Return true if state changed
	return t.isAsleep != prevAsleep || t.inBed != prevInBed
}

// UpdateFromStats updates the state from sleep statistics API response
// This is useful for initial sync or recovery
// Returns true if the state changed
func (t *SleepStateTracker) UpdateFromStats(stats *SleepStats) bool {
	if stats == nil || len(stats.States) == 0 {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	prevAsleep := t.isAsleep
	prevInBed := t.inBed

	// Get current state from stats
	currentState := stats.CurrentState()

	switch currentState {
	case SleepStateAsleep:
		t.isAsleep = true
		t.inBed = true
	case SleepStateAwake:
		t.isAsleep = false
		t.inBed = true
	case SleepStateParentIntervention:
		// Parent intervention - still in bed, sleep state unclear
		t.inBed = true
	case SleepStateAbsent:
		t.isAsleep = false
		t.inBed = false
	}

	// Update last event time from stats
	lastState := stats.States[len(stats.States)-1]
	t.lastEventTime = time.Unix(int64(lastState.BeginTS), 0).UTC()

	return t.isAsleep != prevAsleep || t.inBed != prevInBed
}

// IsAsleep returns true if the baby is currently asleep
func (t *SleepStateTracker) IsAsleep() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.isAsleep
}

// InBed returns true if the baby is currently in the crib/bed
func (t *SleepStateTracker) InBed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.inBed
}

// LastEvent returns the key of the last processed event
func (t *SleepStateTracker) LastEvent() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastEvent
}

// LastEventTime returns the time of the last processed event
func (t *SleepStateTracker) LastEventTime() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastEventTime
}

// GetState returns a snapshot of the current state
func (t *SleepStateTracker) GetState() SleepStateSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return SleepStateSnapshot{
		IsAsleep:      t.isAsleep,
		InBed:         t.inBed,
		LastEvent:     t.lastEvent,
		LastEventTime: t.lastEventTime,
	}
}

// SetState sets the internal state directly (useful for restoring from persistence)
func (t *SleepStateTracker) SetState(isAsleep, inBed bool, lastEvent string, lastEventTime time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.isAsleep = isAsleep
	t.inBed = inBed
	t.lastEvent = lastEvent
	t.lastEventTime = lastEventTime
}
