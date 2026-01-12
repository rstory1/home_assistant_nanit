// Package notification provides types and utilities for handling Nanit camera notifications
package notification

// Sleep state constants from the stats API
const (
	SleepStateAsleep             = "ASLEEP"
	SleepStateAwake              = "AWAKE"
	SleepStateAbsent             = "ABSENT"
	SleepStateParentIntervention = "PARENT_INTERVENTION"
	SleepStateUnknown            = "UNKNOWN"
)

// SleepState represents a single state transition in the sleep state machine
type SleepState struct {
	Title   string  `json:"title"`
	BeginTS float64 `json:"begin_ts"`
	EndTS   float64 `json:"end_ts"`
	UID     string  `json:"uid,omitempty"`
	BabyUID string  `json:"baby_uid,omitempty"`
	Time    float64 `json:"time,omitempty"`
}

// SleepStats represents the sleep statistics from /babies/{uid}/stats/latest
type SleepStats struct {
	Valid               bool         `json:"valid"`
	SleepInterventions  int          `json:"sleep_interventions"`
	TotalAwakeTime      int          `json:"total_awake_time"`
	TimesWokeUp         int          `json:"times_woke_up"`
	Date                string       `json:"date"`
	BedStartTime        int64        `json:"bed_start_time"`
	NumEvents           int          `json:"num_events"`
	LongestSleep        int          `json:"longest_sleep"`
	TimesOutOfCrib      int          `json:"times_out_of_crib"`
	Ongoing             bool         `json:"ongoing"`
	TotalSleepTime      int          `json:"total_sleep_time"`
	SleepEndTime        int64        `json:"sleep_end_time"`
	SleepStartTime      int64        `json:"sleep_start_time"`
	SleepOnset          int          `json:"sleep_onset"`
	TotalPresentTime    int          `json:"total_present_time"`
	Kind                string       `json:"kind"` // "night" or "nap"
	LastWakeUp          int64        `json:"last_wake_up"`
	SleepSessions       int          `json:"sleep_sessions"`
	ParentInterventions int          `json:"parent_interventions"`
	SoothingEvents      int          `json:"soothing_events"`
	States              []SleepState `json:"states"`
}

// SleepStatsResponse represents the API response from /babies/{uid}/stats/latest
type SleepStatsResponse struct {
	Exists bool       `json:"exists"`
	Latest SleepStats `json:"latest"`
}

// CurrentState returns the current sleep state (last state in the states array)
func (s *SleepStats) CurrentState() string {
	if len(s.States) == 0 {
		return SleepStateUnknown
	}
	return s.States[len(s.States)-1].Title
}

// IsAsleep returns true if the current state is ASLEEP
func (s *SleepStats) IsAsleep() bool {
	return s.CurrentState() == SleepStateAsleep
}

// IsInBed returns true if the baby is currently in bed (not ABSENT)
func (s *SleepStats) IsInBed() bool {
	state := s.CurrentState()
	return state != SleepStateUnknown && state != SleepStateAbsent
}

// TotalAwakeTimeMinutes returns the total awake time in minutes
func (s *SleepStats) TotalAwakeTimeMinutes() int {
	return s.TotalAwakeTime / 60
}

// TotalSleepTimeMinutes returns the total sleep time in minutes
func (s *SleepStats) TotalSleepTimeMinutes() int {
	return s.TotalSleepTime / 60
}

// LongestSleepMinutes returns the longest sleep duration in minutes
func (s *SleepStats) LongestSleepMinutes() int {
	return s.LongestSleep / 60
}
