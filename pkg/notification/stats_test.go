package notification

import (
	"encoding/json"
	"testing"
)

func TestSleepStatsParsing(t *testing.T) {
	jsonData := `{
		"exists": true,
		"latest": {
			"valid": true,
			"sleep_interventions": 1,
			"total_awake_time": 1195,
			"times_woke_up": 3,
			"date": "2026-01-09T00:00:00.000Z",
			"bed_start_time": 1767966052,
			"num_events": 15,
			"longest_sleep": 2312,
			"times_out_of_crib": 3,
			"ongoing": true,
			"total_sleep_time": 3266,
			"sleep_end_time": 1767972597,
			"sleep_start_time": 1767966055,
			"sleep_onset": 3,
			"total_present_time": 4461,
			"kind": "night",
			"last_wake_up": 1767972597,
			"sleep_sessions": 3,
			"parent_interventions": 10,
			"soothing_events": 0,
			"states": [
				{"title": "ASLEEP", "begin_ts": 1767952800, "end_ts": 1767957553},
				{"title": "AWAKE", "begin_ts": 1767957553, "end_ts": 1767957608}
			]
		}
	}`

	var resp SleepStatsResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal SleepStatsResponse: %v", err)
	}

	if !resp.Exists {
		t.Error("Expected Exists to be true")
	}

	stats := resp.Latest
	if !stats.Valid {
		t.Error("Expected Valid to be true")
	}
	if stats.SleepInterventions != 1 {
		t.Errorf("SleepInterventions = %d, want 1", stats.SleepInterventions)
	}
	if stats.TotalAwakeTime != 1195 {
		t.Errorf("TotalAwakeTime = %d, want 1195", stats.TotalAwakeTime)
	}
	if stats.TimesWokeUp != 3 {
		t.Errorf("TimesWokeUp = %d, want 3", stats.TimesWokeUp)
	}
	if stats.LongestSleep != 2312 {
		t.Errorf("LongestSleep = %d, want 2312", stats.LongestSleep)
	}
	if stats.TimesOutOfCrib != 3 {
		t.Errorf("TimesOutOfCrib = %d, want 3", stats.TimesOutOfCrib)
	}
	if !stats.Ongoing {
		t.Error("Expected Ongoing to be true")
	}
	if stats.TotalSleepTime != 3266 {
		t.Errorf("TotalSleepTime = %d, want 3266", stats.TotalSleepTime)
	}
	if stats.Kind != "night" {
		t.Errorf("Kind = %q, want %q", stats.Kind, "night")
	}
	if stats.SleepSessions != 3 {
		t.Errorf("SleepSessions = %d, want 3", stats.SleepSessions)
	}
	if stats.ParentInterventions != 10 {
		t.Errorf("ParentInterventions = %d, want 10", stats.ParentInterventions)
	}
	if len(stats.States) != 2 {
		t.Errorf("Expected 2 states, got %d", len(stats.States))
	}
}

func TestSleepStatsCurrentState(t *testing.T) {
	tests := []struct {
		name     string
		states   []SleepState
		expected string
	}{
		{
			name:     "empty states",
			states:   []SleepState{},
			expected: "UNKNOWN",
		},
		{
			name: "last state is ASLEEP",
			states: []SleepState{
				{Title: "AWAKE", BeginTS: 100, EndTS: 200},
				{Title: "ASLEEP", BeginTS: 200, EndTS: 300},
			},
			expected: "ASLEEP",
		},
		{
			name: "last state is AWAKE",
			states: []SleepState{
				{Title: "ASLEEP", BeginTS: 100, EndTS: 200},
				{Title: "AWAKE", BeginTS: 200, EndTS: 300},
			},
			expected: "AWAKE",
		},
		{
			name: "last state is ABSENT",
			states: []SleepState{
				{Title: "ASLEEP", BeginTS: 100, EndTS: 200},
				{Title: "ABSENT", BeginTS: 200, EndTS: 300},
			},
			expected: "ABSENT",
		},
		{
			name: "last state is PARENT_INTERVENTION",
			states: []SleepState{
				{Title: "ASLEEP", BeginTS: 100, EndTS: 200},
				{Title: "PARENT_INTERVENTION", BeginTS: 200, EndTS: 300},
			},
			expected: "PARENT_INTERVENTION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := SleepStats{States: tt.states}
			if got := stats.CurrentState(); got != tt.expected {
				t.Errorf("CurrentState() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSleepStatsIsAsleep(t *testing.T) {
	tests := []struct {
		name     string
		states   []SleepState
		expected bool
	}{
		{
			name:     "empty states - not asleep",
			states:   []SleepState{},
			expected: false,
		},
		{
			name: "last state ASLEEP",
			states: []SleepState{
				{Title: "ASLEEP", BeginTS: 200, EndTS: 300},
			},
			expected: true,
		},
		{
			name: "last state AWAKE",
			states: []SleepState{
				{Title: "AWAKE", BeginTS: 200, EndTS: 300},
			},
			expected: false,
		},
		{
			name: "last state ABSENT",
			states: []SleepState{
				{Title: "ABSENT", BeginTS: 200, EndTS: 300},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := SleepStats{States: tt.states}
			if got := stats.IsAsleep(); got != tt.expected {
				t.Errorf("IsAsleep() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSleepStatsIsInBed(t *testing.T) {
	tests := []struct {
		name     string
		states   []SleepState
		expected bool
	}{
		{
			name:     "empty states - not in bed",
			states:   []SleepState{},
			expected: false,
		},
		{
			name: "last state ASLEEP - in bed",
			states: []SleepState{
				{Title: "ASLEEP", BeginTS: 200, EndTS: 300},
			},
			expected: true,
		},
		{
			name: "last state AWAKE - in bed",
			states: []SleepState{
				{Title: "AWAKE", BeginTS: 200, EndTS: 300},
			},
			expected: true,
		},
		{
			name: "last state PARENT_INTERVENTION - in bed",
			states: []SleepState{
				{Title: "PARENT_INTERVENTION", BeginTS: 200, EndTS: 300},
			},
			expected: true,
		},
		{
			name: "last state ABSENT - not in bed",
			states: []SleepState{
				{Title: "ABSENT", BeginTS: 200, EndTS: 300},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := SleepStats{States: tt.states}
			if got := stats.IsInBed(); got != tt.expected {
				t.Errorf("IsInBed() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSleepStatsTotalAwakeTimeMinutes(t *testing.T) {
	stats := SleepStats{TotalAwakeTime: 1195} // seconds
	expected := 19                            // 1195 / 60 = 19.9, truncated to 19

	if got := stats.TotalAwakeTimeMinutes(); got != expected {
		t.Errorf("TotalAwakeTimeMinutes() = %d, want %d", got, expected)
	}
}

func TestSleepStatsTotalSleepTimeMinutes(t *testing.T) {
	stats := SleepStats{TotalSleepTime: 3266} // seconds
	expected := 54                            // 3266 / 60 = 54.4, truncated to 54

	if got := stats.TotalSleepTimeMinutes(); got != expected {
		t.Errorf("TotalSleepTimeMinutes() = %d, want %d", got, expected)
	}
}

func TestSleepStateConstants(t *testing.T) {
	if SleepStateAsleep != "ASLEEP" {
		t.Errorf("SleepStateAsleep = %q, want %q", SleepStateAsleep, "ASLEEP")
	}
	if SleepStateAwake != "AWAKE" {
		t.Errorf("SleepStateAwake = %q, want %q", SleepStateAwake, "AWAKE")
	}
	if SleepStateAbsent != "ABSENT" {
		t.Errorf("SleepStateAbsent = %q, want %q", SleepStateAbsent, "ABSENT")
	}
	if SleepStateParentIntervention != "PARENT_INTERVENTION" {
		t.Errorf("SleepStateParentIntervention = %q, want %q", SleepStateParentIntervention, "PARENT_INTERVENTION")
	}
}

func TestSleepStatsNoData(t *testing.T) {
	jsonData := `{"exists": false}`

	var resp SleepStatsResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if resp.Exists {
		t.Error("Expected Exists to be false")
	}
}
