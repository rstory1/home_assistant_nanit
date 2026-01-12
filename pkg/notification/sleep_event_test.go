package notification

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSleepEventParsing(t *testing.T) {
	jsonData := `{
		"key": "FELL_ASLEEP",
		"internal_key": "FELL_ASLEEP",
		"title": "Baby fell asleep",
		"time": 1767969084.0,
		"begin_ts": 1767969074.0,
		"end_ts": 1767969094.0,
		"baby_uid": "testbaby1",
		"camera_uid": "TESTCAM12345XX",
		"uid": "abc123",
		"confidence": 1.0,
		"source": "sm"
	}`

	var event SleepEvent
	err := json.Unmarshal([]byte(jsonData), &event)
	if err != nil {
		t.Fatalf("Failed to unmarshal SleepEvent: %v", err)
	}

	if event.Key != "FELL_ASLEEP" {
		t.Errorf("Key = %q, want %q", event.Key, "FELL_ASLEEP")
	}
	if event.InternalKey != "FELL_ASLEEP" {
		t.Errorf("InternalKey = %q, want %q", event.InternalKey, "FELL_ASLEEP")
	}
	if event.Title != "Baby fell asleep" {
		t.Errorf("Title = %q, want %q", event.Title, "Baby fell asleep")
	}
	if event.BabyUID != "testbaby1" {
		t.Errorf("BabyUID = %q, want %q", event.BabyUID, "testbaby1")
	}
	if event.CameraUID != "TESTCAM12345XX" {
		t.Errorf("CameraUID = %q, want %q", event.CameraUID, "TESTCAM12345XX")
	}
	if event.UID != "abc123" {
		t.Errorf("UID = %q, want %q", event.UID, "abc123")
	}
	if event.Confidence != 1.0 {
		t.Errorf("Confidence = %f, want %f", event.Confidence, 1.0)
	}
	if event.Source != "sm" {
		t.Errorf("Source = %q, want %q", event.Source, "sm")
	}

	// Check time parsing
	expectedTime := time.Unix(1767969084, 0).UTC()
	if !event.Time().Equal(expectedTime) {
		t.Errorf("Time() = %v, want %v", event.Time(), expectedTime)
	}
}

func TestSleepEventParsingAllTypes(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		internalKey string
	}{
		{"fell asleep", "FELL_ASLEEP", "FELL_ASLEEP"},
		{"woke up", "WOKE_UP", "WOKE_UP"},
		{"put in bed", "PUT_IN_BED", "PUT_IN_BED"},
		{"put to sleep", "PUT_TO_SLEEP", "PUT_TO_SLEEP"},
		{"removed", "REMOVED", "REMOVED"},
		{"removed asleep", "REMOVED_ASLEEP", "REMOVED_ASLEEP"},
		{"visit", "VISIT", "VISIT"},
		{"visit fell asleep", "VISIT", "VISIT_FELL_ASLEEP"},
		{"visit woke up", "VISIT", "VISIT_WOKE_UP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData := `{"key": "` + tt.key + `", "internal_key": "` + tt.internalKey + `", "uid": "test123", "time": 1767969084.0, "baby_uid": "baby1"}`
			var event SleepEvent
			err := json.Unmarshal([]byte(jsonData), &event)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}
			if event.Key != tt.key {
				t.Errorf("Key = %q, want %q", event.Key, tt.key)
			}
			if event.InternalKey != tt.internalKey {
				t.Errorf("InternalKey = %q, want %q", event.InternalKey, tt.internalKey)
			}
		})
	}
}

func TestSleepEventIsSleepRelated(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"FELL_ASLEEP", true},
		{"WOKE_UP", true},
		{"PUT_IN_BED", false},
		{"REMOVED", false},
		{"VISIT", false},
		{"MOTION", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			event := SleepEvent{Key: tt.key}
			if got := event.IsSleepRelated(); got != tt.expected {
				t.Errorf("IsSleepRelated() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSleepEventIsBedRelated(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"PUT_IN_BED", true},
		{"PUT_TO_SLEEP", true},
		{"REMOVED", true},
		{"REMOVED_ASLEEP", true},
		{"FELL_ASLEEP", false},
		{"WOKE_UP", false},
		{"VISIT", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			event := SleepEvent{Key: tt.key}
			if got := event.IsBedRelated(); got != tt.expected {
				t.Errorf("IsBedRelated() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSleepEventsResponseParsing(t *testing.T) {
	jsonData := `{
		"events": [
			{"key": "FELL_ASLEEP", "internal_key": "FELL_ASLEEP", "uid": "e1", "time": 1767969084.0, "baby_uid": "baby1"},
			{"key": "WOKE_UP", "internal_key": "WOKE_UP", "uid": "e2", "time": 1767969184.0, "baby_uid": "baby1"}
		]
	}`

	var resp SleepEventsResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal SleepEventsResponse: %v", err)
	}

	if len(resp.Events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(resp.Events))
	}

	if resp.Events[0].Key != "FELL_ASLEEP" {
		t.Errorf("First event Key = %q, want %q", resp.Events[0].Key, "FELL_ASLEEP")
	}
	if resp.Events[1].Key != "WOKE_UP" {
		t.Errorf("Second event Key = %q, want %q", resp.Events[1].Key, "WOKE_UP")
	}
}

func TestSleepEventKeyConstants(t *testing.T) {
	// Verify constant values match expected API values
	if SleepEventKeyFellAsleep != "FELL_ASLEEP" {
		t.Errorf("SleepEventKeyFellAsleep = %q, want %q", SleepEventKeyFellAsleep, "FELL_ASLEEP")
	}
	if SleepEventKeyWokeUp != "WOKE_UP" {
		t.Errorf("SleepEventKeyWokeUp = %q, want %q", SleepEventKeyWokeUp, "WOKE_UP")
	}
	if SleepEventKeyPutInBed != "PUT_IN_BED" {
		t.Errorf("SleepEventKeyPutInBed = %q, want %q", SleepEventKeyPutInBed, "PUT_IN_BED")
	}
	if SleepEventKeyPutToSleep != "PUT_TO_SLEEP" {
		t.Errorf("SleepEventKeyPutToSleep = %q, want %q", SleepEventKeyPutToSleep, "PUT_TO_SLEEP")
	}
	if SleepEventKeyRemoved != "REMOVED" {
		t.Errorf("SleepEventKeyRemoved = %q, want %q", SleepEventKeyRemoved, "REMOVED")
	}
	if SleepEventKeyRemovedAsleep != "REMOVED_ASLEEP" {
		t.Errorf("SleepEventKeyRemovedAsleep = %q, want %q", SleepEventKeyRemovedAsleep, "REMOVED_ASLEEP")
	}
	if SleepEventKeyVisit != "VISIT" {
		t.Errorf("SleepEventKeyVisit = %q, want %q", SleepEventKeyVisit, "VISIT")
	}
}
