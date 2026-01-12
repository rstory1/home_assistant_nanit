package notification

import (
	"sync"
	"testing"
)

func TestDeduplicatorNewMessagesNotSeen(t *testing.T) {
	d := NewDeduplicator(100)

	if d.IsSeen(1) {
		t.Error("New message 1 should not be seen")
	}
	if d.IsSeen(2) {
		t.Error("New message 2 should not be seen")
	}
	if d.IsSeen(3) {
		t.Error("New message 3 should not be seen")
	}
}

func TestDeduplicatorMarkSeen(t *testing.T) {
	d := NewDeduplicator(100)

	// Initially not seen
	if d.IsSeen(42) {
		t.Error("Message should not be seen before marking")
	}

	// Mark as seen
	d.MarkSeen(42)

	// Now should be seen
	if !d.IsSeen(42) {
		t.Error("Message should be seen after marking")
	}
}

func TestDeduplicatorMultipleMarks(t *testing.T) {
	d := NewDeduplicator(100)

	ids := []int{1, 2, 3, 4, 5}

	// Mark all
	for _, id := range ids {
		d.MarkSeen(id)
	}

	// Verify all seen
	for _, id := range ids {
		if !d.IsSeen(id) {
			t.Errorf("Message %d should be seen", id)
		}
	}

	// Verify unknown not seen
	if d.IsSeen(999) {
		t.Error("Message 999 should not be seen")
	}
}

func TestDeduplicatorEviction(t *testing.T) {
	maxSize := 10
	d := NewDeduplicator(maxSize)

	// Add more than maxSize messages
	for i := 1; i <= 20; i++ {
		d.MarkSeen(i)
	}

	// Older messages should be evicted (1-10)
	// Newer messages should remain (11-20)
	for i := 1; i <= 10; i++ {
		if d.IsSeen(i) {
			t.Errorf("Old message %d should have been evicted", i)
		}
	}

	for i := 11; i <= 20; i++ {
		if !d.IsSeen(i) {
			t.Errorf("New message %d should still be seen", i)
		}
	}
}

func TestDeduplicatorEvictionKeepsNewest(t *testing.T) {
	maxSize := 5
	d := NewDeduplicator(maxSize)

	// Add messages in order
	for i := 1; i <= 10; i++ {
		d.MarkSeen(i)
	}

	// Should keep the newest 5 (6-10)
	count := 0
	for i := 1; i <= 10; i++ {
		if d.IsSeen(i) {
			count++
		}
	}

	if count > maxSize {
		t.Errorf("Should have at most %d messages, got %d", maxSize, count)
	}
}

func TestDeduplicatorConcurrentAccess(t *testing.T) {
	d := NewDeduplicator(1000)

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Spawn goroutines that mark and check messages
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numOperations; i++ {
				msgID := goroutineID*numOperations + i
				d.MarkSeen(msgID)
				d.IsSeen(msgID)
			}
		}(g)
	}

	wg.Wait()

	// No panic = success for concurrent access
}

func TestDeduplicatorSize(t *testing.T) {
	d := NewDeduplicator(100)

	if d.Size() != 0 {
		t.Errorf("Initial size should be 0, got %d", d.Size())
	}

	d.MarkSeen(1)
	d.MarkSeen(2)
	d.MarkSeen(3)

	if d.Size() != 3 {
		t.Errorf("Size should be 3, got %d", d.Size())
	}

	// Marking same ID again shouldn't increase size
	d.MarkSeen(1)
	if d.Size() != 3 {
		t.Errorf("Size should still be 3, got %d", d.Size())
	}
}

func TestDeduplicatorClear(t *testing.T) {
	d := NewDeduplicator(100)

	d.MarkSeen(1)
	d.MarkSeen(2)
	d.MarkSeen(3)

	if d.Size() != 3 {
		t.Errorf("Size should be 3, got %d", d.Size())
	}

	d.Clear()

	if d.Size() != 0 {
		t.Errorf("Size should be 0 after clear, got %d", d.Size())
	}

	if d.IsSeen(1) {
		t.Error("Message 1 should not be seen after clear")
	}
}

func TestDeduplicatorZeroMaxSize(t *testing.T) {
	// Edge case: maxSize of 0 should still work (no eviction)
	d := NewDeduplicator(0)

	d.MarkSeen(1)
	d.MarkSeen(2)

	// With maxSize 0, we treat it as unlimited
	if !d.IsSeen(1) {
		t.Error("Message 1 should be seen")
	}
	if !d.IsSeen(2) {
		t.Error("Message 2 should be seen")
	}
}
