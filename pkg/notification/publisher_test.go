package notification

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// MockMQTTClient implements MQTTClient for testing
type MockMQTTClient struct {
	mu        sync.Mutex
	published map[string]string // topic -> payload
}

func NewMockMQTTClient() *MockMQTTClient {
	return &MockMQTTClient{
		published: make(map[string]string),
	}
}

func (m *MockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var payloadStr string
	switch v := payload.(type) {
	case string:
		payloadStr = v
	case []byte:
		payloadStr = string(v)
	default:
		b, _ := json.Marshal(v)
		payloadStr = string(b)
	}

	m.published[topic] = payloadStr
	return nil
}

func (m *MockMQTTClient) GetPublished(topic string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.published[topic]
	return v, ok
}

func (m *MockMQTTClient) AllPublished() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]string)
	for k, v := range m.published {
		result[k] = v
	}
	return result
}

func TestPublisherPublishEvent_Motion(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "nanit")

	event := Event{
		ID:        1,
		Type:      NotificationMotion,
		BabyUID:   "baby123",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		EventUID:  "event-abc",
	}

	err := publisher.PublishEvent(event)
	if err != nil {
		t.Fatalf("PublishEvent() error = %v", err)
	}

	// Check event topic
	eventTopic := "nanit/babies/baby123/events/motion"
	payload, ok := client.GetPublished(eventTopic)
	if !ok {
		t.Errorf("Event not published to %s", eventTopic)
	}

	// Verify payload contains expected fields
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &eventData); err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}

	if eventData["event_uid"] != "event-abc" {
		t.Errorf("event_uid = %v, want event-abc", eventData["event_uid"])
	}

	// Check state topic (motion uses timestamp)
	stateTopic := "nanit/babies/baby123/motion_timestamp"
	statePayload, ok := client.GetPublished(stateTopic)
	if !ok {
		t.Errorf("State not published to %s", stateTopic)
	}

	// Should be ISO 8601 format for Home Assistant device_class: timestamp
	expectedTS := "2024-01-15T10:30:00Z"
	if statePayload != expectedTS {
		t.Errorf("motion_timestamp = %s, want %s", statePayload, expectedTS)
	}
}

func TestPublisherPublishEvent_CameraOffline(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "nanit")

	event := Event{
		ID:        3,
		Type:      NotificationCameraOffline,
		BabyUID:   "baby789",
		Timestamp: time.Now(),
	}

	err := publisher.PublishEvent(event)
	if err != nil {
		t.Fatalf("PublishEvent() error = %v", err)
	}

	// Check state topic (camera_offline sets camera_online to false)
	stateTopic := "nanit/babies/baby789/camera_online"
	statePayload, ok := client.GetPublished(stateTopic)
	if !ok {
		t.Errorf("State not published to %s", stateTopic)
	}

	if statePayload != "false" {
		t.Errorf("camera_online = %s, want false", statePayload)
	}
}

func TestPublisherPublishEvent_CameraOnline(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "nanit")

	event := Event{
		ID:        4,
		Type:      NotificationCameraOnline,
		BabyUID:   "baby789",
		Timestamp: time.Now(),
	}

	err := publisher.PublishEvent(event)
	if err != nil {
		t.Fatalf("PublishEvent() error = %v", err)
	}

	// Check state topic
	stateTopic := "nanit/babies/baby789/camera_online"
	statePayload, ok := client.GetPublished(stateTopic)
	if !ok {
		t.Errorf("State not published to %s", stateTopic)
	}

	if statePayload != "true" {
		t.Errorf("camera_online = %s, want true", statePayload)
	}
}

func TestPublisherPublishEvent_AllTypes(t *testing.T) {
	types := []struct {
		notifType   NotificationType
		eventTopic  string
		stateTopic  string
		stateValue  string
	}{
		{NotificationMotion, "events/motion", "motion_timestamp", ""},
		{NotificationSound, "events/sound", "sound_timestamp", ""},
		{NotificationStanding, "events/standing", "is_standing", "true"},
		{NotificationLeftBed, "events/left_bed", "left_bed", "true"},
		{NotificationAlertZone, "events/alert_zone", "alert_zone_triggered", "true"},
		{NotificationTemperature, "events/temperature_alert", "temperature_alert", "true"},
		{NotificationHumidity, "events/humidity_alert", "humidity_alert", "true"},
		{NotificationBreathing, "events/breathing_alert", "breathing_alert", "true"},
		{NotificationCameraOffline, "events/camera_offline", "camera_online", "false"},
		{NotificationCameraOnline, "events/camera_online", "camera_online", "true"},
		{NotificationLowBattery, "events/low_battery", "low_battery", "true"},
	}

	for _, tt := range types {
		t.Run(string(tt.notifType), func(t *testing.T) {
			client := NewMockMQTTClient()
			publisher := NewPublisher(client, "nanit")

			event := Event{
				ID:        1,
				Type:      tt.notifType,
				BabyUID:   "baby",
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			}

			err := publisher.PublishEvent(event)
			if err != nil {
				t.Fatalf("PublishEvent() error = %v", err)
			}

			// Check event topic
			eventTopic := "nanit/babies/baby/" + tt.eventTopic
			_, ok := client.GetPublished(eventTopic)
			if !ok {
				t.Errorf("Event not published to %s", eventTopic)
			}

			// Check state topic (if applicable)
			if tt.stateTopic != "" {
				stateTopic := "nanit/babies/baby/" + tt.stateTopic
				statePayload, ok := client.GetPublished(stateTopic)
				if !ok {
					t.Errorf("State not published to %s", stateTopic)
				}

				// For boolean states, verify the value
				if tt.stateValue != "" && statePayload != tt.stateValue {
					t.Errorf("State value = %s, want %s", statePayload, tt.stateValue)
				}
			}
		})
	}
}

func TestPublisherTopicPrefix(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "home/nanit")

	event := Event{
		ID:        1,
		Type:      NotificationMotion,
		BabyUID:   "baby",
		Timestamp: time.Now(),
	}

	publisher.PublishEvent(event)

	// Should use custom prefix
	eventTopic := "home/nanit/babies/baby/events/motion"
	_, ok := client.GetPublished(eventTopic)
	if !ok {
		t.Errorf("Event not published with custom prefix to %s", eventTopic)
	}
}

func TestPublisherEventPayloadFormat(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "nanit")

	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	event := Event{
		ID:        42,
		Type:      NotificationMotion,
		BabyUID:   "baby",
		Timestamp: timestamp,
		EventUID:  "event-xyz",
	}

	publisher.PublishEvent(event)

	eventTopic := "nanit/babies/baby/events/motion"
	payload, _ := client.GetPublished(eventTopic)

	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &eventData); err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}

	// Check all expected fields
	if eventData["id"] != float64(42) {
		t.Errorf("id = %v, want 42", eventData["id"])
	}

	if eventData["type"] != "MOTION" {
		t.Errorf("type = %v, want MOTION", eventData["type"])
	}

	if eventData["baby_uid"] != "baby" {
		t.Errorf("baby_uid = %v, want baby", eventData["baby_uid"])
	}

	if eventData["event_uid"] != "event-xyz" {
		t.Errorf("event_uid = %v, want event-xyz", eventData["event_uid"])
	}

	if eventData["timestamp"] != "2024-01-15T10:30:00Z" {
		t.Errorf("timestamp = %v, want 2024-01-15T10:30:00Z", eventData["timestamp"])
	}

	if eventData["unix_timestamp"] != float64(1705314600) {
		t.Errorf("unix_timestamp = %v, want 1705314600", eventData["unix_timestamp"])
	}
}

func TestPublisherPublishSleepState(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "nanit")

	state := SleepStateSnapshot{
		IsAsleep:      true,
		InBed:         true,
		LastEvent:     "FELL_ASLEEP",
		LastEventTime: time.Date(2024, 1, 15, 22, 30, 0, 0, time.UTC),
	}

	err := publisher.PublishSleepState("baby123", state)
	if err != nil {
		t.Fatalf("PublishSleepState() error = %v", err)
	}

	// Check is_asleep topic
	isAsleepTopic := "nanit/babies/baby123/is_asleep"
	payload, ok := client.GetPublished(isAsleepTopic)
	if !ok {
		t.Errorf("State not published to %s", isAsleepTopic)
	}
	if payload != "true" {
		t.Errorf("is_asleep = %s, want true", payload)
	}

	// Check in_bed topic
	inBedTopic := "nanit/babies/baby123/in_bed"
	payload, ok = client.GetPublished(inBedTopic)
	if !ok {
		t.Errorf("State not published to %s", inBedTopic)
	}
	if payload != "true" {
		t.Errorf("in_bed = %s, want true", payload)
	}

	// Check last_sleep_event topic
	lastEventTopic := "nanit/babies/baby123/last_sleep_event"
	payload, ok = client.GetPublished(lastEventTopic)
	if !ok {
		t.Errorf("State not published to %s", lastEventTopic)
	}
	if payload != "FELL_ASLEEP" {
		t.Errorf("last_sleep_event = %s, want FELL_ASLEEP", payload)
	}
}

func TestPublisherPublishSleepState_Awake(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "nanit")

	state := SleepStateSnapshot{
		IsAsleep:      false,
		InBed:         true,
		LastEvent:     "WOKE_UP",
		LastEventTime: time.Now(),
	}

	err := publisher.PublishSleepState("baby123", state)
	if err != nil {
		t.Fatalf("PublishSleepState() error = %v", err)
	}

	// Check is_asleep topic - should be false
	isAsleepTopic := "nanit/babies/baby123/is_asleep"
	payload, ok := client.GetPublished(isAsleepTopic)
	if !ok {
		t.Errorf("State not published to %s", isAsleepTopic)
	}
	if payload != "false" {
		t.Errorf("is_asleep = %s, want false", payload)
	}
}

func TestPublisherPublishSleepStats(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "nanit")

	stats := &SleepStats{
		Valid:              true,
		TimesWokeUp:        3,
		SleepInterventions: 1,
		TotalAwakeTime:     1200, // seconds
		TotalSleepTime:     3600, // seconds
	}

	err := publisher.PublishSleepStats("baby456", stats)
	if err != nil {
		t.Fatalf("PublishSleepStats() error = %v", err)
	}

	// Check times_woke_up
	topic := "nanit/babies/baby456/times_woke_up"
	payload, ok := client.GetPublished(topic)
	if !ok {
		t.Errorf("Stats not published to %s", topic)
	}
	if payload != "3" {
		t.Errorf("times_woke_up = %s, want 3", payload)
	}

	// Check sleep_interventions
	topic = "nanit/babies/baby456/sleep_interventions"
	payload, ok = client.GetPublished(topic)
	if !ok {
		t.Errorf("Stats not published to %s", topic)
	}
	if payload != "1" {
		t.Errorf("sleep_interventions = %s, want 1", payload)
	}

	// Check awake_time_today (should be in minutes)
	topic = "nanit/babies/baby456/awake_time_today"
	payload, ok = client.GetPublished(topic)
	if !ok {
		t.Errorf("Stats not published to %s", topic)
	}
	if payload != "20" { // 1200 seconds = 20 minutes
		t.Errorf("awake_time_today = %s, want 20", payload)
	}

	// Check sleep_time_today (should be in minutes)
	topic = "nanit/babies/baby456/sleep_time_today"
	payload, ok = client.GetPublished(topic)
	if !ok {
		t.Errorf("Stats not published to %s", topic)
	}
	if payload != "60" { // 3600 seconds = 60 minutes
		t.Errorf("sleep_time_today = %s, want 60", payload)
	}
}

func TestPublisherPublishSleepStats_NilStats(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "nanit")

	// Should not error on nil stats
	err := publisher.PublishSleepStats("baby123", nil)
	if err != nil {
		t.Fatalf("PublishSleepStats(nil) error = %v", err)
	}

	// Nothing should be published
	if len(client.AllPublished()) != 0 {
		t.Errorf("Expected no publications for nil stats, got %d", len(client.AllPublished()))
	}
}

func TestPublisherPublishSleepEvent(t *testing.T) {
	client := NewMockMQTTClient()
	publisher := NewPublisher(client, "nanit")

	now := time.Now()
	event := SleepEvent{
		Key:     SleepEventKeyFellAsleep,
		UID:     "event-123",
		TimeRaw: float64(now.Unix()),
		BabyUID: "baby789",
	}

	err := publisher.PublishSleepEvent(event)
	if err != nil {
		t.Fatalf("PublishSleepEvent() error = %v", err)
	}

	// Check event topic
	eventTopic := "nanit/babies/baby789/events/fell_asleep"
	payload, ok := client.GetPublished(eventTopic)
	if !ok {
		t.Errorf("Event not published to %s", eventTopic)
	}

	// Verify payload contains expected fields
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &eventData); err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}

	if eventData["event_uid"] != "event-123" {
		t.Errorf("event_uid = %v, want event-123", eventData["event_uid"])
	}

	if eventData["type"] != "FELL_ASLEEP" {
		t.Errorf("type = %v, want FELL_ASLEEP", eventData["type"])
	}
}

func TestPublisherPublishSleepEvent_AllTypes(t *testing.T) {
	types := []struct {
		key        string
		eventTopic string
	}{
		{SleepEventKeyFellAsleep, "events/fell_asleep"},
		{SleepEventKeyWokeUp, "events/woke_up"},
		{SleepEventKeyPutInBed, "events/put_in_bed"},
		{SleepEventKeyPutToSleep, "events/put_to_sleep"},
		{SleepEventKeyRemoved, "events/removed"},
		{SleepEventKeyRemovedAsleep, "events/removed_asleep"},
		{SleepEventKeyVisit, "events/visit"},
	}

	for _, tt := range types {
		t.Run(tt.key, func(t *testing.T) {
			client := NewMockMQTTClient()
			publisher := NewPublisher(client, "nanit")

			event := SleepEvent{
				Key:     tt.key,
				UID:     "test-uid",
				TimeRaw: float64(time.Now().Unix()),
				BabyUID: "baby",
			}

			err := publisher.PublishSleepEvent(event)
			if err != nil {
				t.Fatalf("PublishSleepEvent() error = %v", err)
			}

			// Check event topic
			eventTopic := "nanit/babies/baby/" + tt.eventTopic
			_, ok := client.GetPublished(eventTopic)
			if !ok {
				t.Errorf("Event not published to %s", eventTopic)
			}
		})
	}
}
