// Package notification provides types and utilities for handling Nanit camera notifications
package notification

import (
	"time"
)

// Sleep event key constants matching Nanit API
const (
	SleepEventKeyFellAsleep    = "FELL_ASLEEP"
	SleepEventKeyWokeUp        = "WOKE_UP"
	SleepEventKeyPutInBed      = "PUT_IN_BED"
	SleepEventKeyPutToSleep    = "PUT_TO_SLEEP"
	SleepEventKeyRemoved       = "REMOVED"
	SleepEventKeyRemovedAsleep = "REMOVED_ASLEEP"
	SleepEventKeyVisit         = "VISIT"
)

// SleepEvent represents an event from the Nanit /events API endpoint
type SleepEvent struct {
	Key         string  `json:"key"`
	InternalKey string  `json:"internal_key"`
	Title       string  `json:"title,omitempty"`
	TimeRaw     float64 `json:"time"`
	BeginTS     float64 `json:"begin_ts,omitempty"`
	EndTS       float64 `json:"end_ts,omitempty"`
	BabyUID     string  `json:"baby_uid"`
	CameraUID   string  `json:"camera_uid,omitempty"`
	UID         string  `json:"uid"`
	Confidence  float64 `json:"confidence,omitempty"`
	Source      string  `json:"source,omitempty"`
}

// Time returns the event time as a time.Time in UTC
func (e SleepEvent) Time() time.Time {
	return time.Unix(int64(e.TimeRaw), 0).UTC()
}

// BeginTime returns the event begin time as a time.Time in UTC
func (e SleepEvent) BeginTime() time.Time {
	if e.BeginTS == 0 {
		return e.Time()
	}
	return time.Unix(int64(e.BeginTS), 0).UTC()
}

// EndTime returns the event end time as a time.Time in UTC
func (e SleepEvent) EndTime() time.Time {
	if e.EndTS == 0 {
		return e.Time()
	}
	return time.Unix(int64(e.EndTS), 0).UTC()
}

// IsSleepRelated returns true if this event affects the is_asleep state
func (e SleepEvent) IsSleepRelated() bool {
	return e.Key == SleepEventKeyFellAsleep || e.Key == SleepEventKeyWokeUp
}

// IsBedRelated returns true if this event affects the in_bed state
func (e SleepEvent) IsBedRelated() bool {
	switch e.Key {
	case SleepEventKeyPutInBed, SleepEventKeyPutToSleep, SleepEventKeyRemoved, SleepEventKeyRemovedAsleep:
		return true
	default:
		return false
	}
}

// NotificationType returns the corresponding NotificationType for this sleep event
func (e SleepEvent) NotificationType() NotificationType {
	switch e.Key {
	case SleepEventKeyFellAsleep:
		return NotificationFellAsleep
	case SleepEventKeyWokeUp:
		return NotificationWokeUp
	case SleepEventKeyPutInBed:
		return NotificationPutInBed
	case SleepEventKeyPutToSleep:
		return NotificationPutToSleep
	case SleepEventKeyRemoved:
		return NotificationRemoved
	case SleepEventKeyRemovedAsleep:
		return NotificationRemovedAsleep
	case SleepEventKeyVisit:
		return NotificationVisit
	default:
		// Return the key as a notification type for unknown events
		return NotificationType(e.Key)
	}
}

// SleepEventsResponse represents the API response from /babies/{uid}/events
type SleepEventsResponse struct {
	Events []SleepEvent `json:"events"`
}

// FilterSleepEvents filters events by a predicate function
func FilterSleepEvents(events []SleepEvent, predicate func(SleepEvent) bool) []SleepEvent {
	var result []SleepEvent
	for _, e := range events {
		if predicate(e) {
			result = append(result, e)
		}
	}
	return result
}
