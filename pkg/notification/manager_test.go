package notification

import (
	"context"
	"testing"
	"time"

	"github.com/scgreenhalgh/home_assistant_nanit/pkg/baby"
	"github.com/scgreenhalgh/home_assistant_nanit/pkg/message"
)

func TestManager_Run_PollsAndPublishes(t *testing.T) {
	timestamp := time.Now()
	unixTime := message.UnixTime(timestamp)

	fetcher := &MockMessageFetcher{
		messages: []message.Message{
			{Id: 1, BabyUid: "baby1", Type: "MOTION", Time: unixTime},
		},
	}

	mqttClient := NewMockMQTTClient()

	config := ManagerConfig{
		PollerConfig: DefaultPollerConfig(),
		TopicPrefix:  "nanit",
		Babies: []baby.Baby{
			{UID: "baby1", Name: "Test Baby"},
		},
	}

	manager := NewManager(config, fetcher, mqttClient)

	// Run for a short time
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	manager.Run(ctx)

	// Verify event was published
	eventTopic := "nanit/babies/baby1/events/motion"
	_, ok := mqttClient.GetPublished(eventTopic)
	if !ok {
		t.Errorf("Event not published to %s", eventTopic)
	}

	// Verify state was published
	stateTopic := "nanit/babies/baby1/motion_timestamp"
	_, ok = mqttClient.GetPublished(stateTopic)
	if !ok {
		t.Errorf("State not published to %s", stateTopic)
	}
}

func TestManager_Run_MultipleBabies(t *testing.T) {
	timestamp := time.Now()
	unixTime := message.UnixTime(timestamp)

	fetcher := &MockMessageFetcher{
		messages: []message.Message{
			{Id: 1, BabyUid: "baby1", Type: "MOTION", Time: unixTime},
		},
	}

	mqttClient := NewMockMQTTClient()

	config := ManagerConfig{
		PollerConfig: DefaultPollerConfig(),
		TopicPrefix:  "nanit",
		Babies: []baby.Baby{
			{UID: "baby1", Name: "Baby One"},
			{UID: "baby2", Name: "Baby Two"},
		},
	}

	manager := NewManager(config, fetcher, mqttClient)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	manager.Run(ctx)

	// Both babies should have been polled
	// (same events for simplicity, but in real use would be different)
	stats := manager.Stats()
	if stats.PollCount < 2 {
		t.Errorf("PollCount = %d, want >= 2 (one per baby)", stats.PollCount)
	}
}

func TestManager_Stats(t *testing.T) {
	timestamp := time.Now()
	unixTime := message.UnixTime(timestamp)

	fetcher := &MockMessageFetcher{
		messages: []message.Message{
			{Id: 1, BabyUid: "baby1", Type: "MOTION", Time: unixTime},
			{Id: 2, BabyUid: "baby1", Type: "SOUND", Time: unixTime},
		},
	}

	mqttClient := NewMockMQTTClient()

	config := ManagerConfig{
		PollerConfig: DefaultPollerConfig(),
		TopicPrefix:  "nanit",
		Babies: []baby.Baby{
			{UID: "baby1", Name: "Test Baby"},
		},
	}

	manager := NewManager(config, fetcher, mqttClient)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	manager.Run(ctx)

	stats := manager.Stats()
	if stats.PollCount == 0 {
		t.Error("PollCount should be > 0")
	}

	if stats.EventsReceived == 0 {
		t.Error("EventsReceived should be > 0")
	}
}

func TestManager_NoBabies(t *testing.T) {
	fetcher := &MockMessageFetcher{}
	mqttClient := NewMockMQTTClient()

	config := ManagerConfig{
		PollerConfig: DefaultPollerConfig(),
		TopicPrefix:  "nanit",
		Babies:       []baby.Baby{}, // No babies
	}

	manager := NewManager(config, fetcher, mqttClient)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := manager.Run(ctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil for no babies", err)
	}
}

// MockCombinedFetcher implements both MessageFetcher and SleepEventFetcher and StatsFetcher
type MockCombinedFetcher struct {
	*MockMessageFetcher
	sleepEvents []SleepEvent
	sleepStats  *SleepStats
	sleepErr    error
	statsErr    error
}

func (m *MockCombinedFetcher) TryFetchSleepEvents(babyUID string, limit int) ([]SleepEvent, error) {
	if m.sleepErr != nil {
		return nil, m.sleepErr
	}
	return m.sleepEvents, nil
}

func (m *MockCombinedFetcher) TryFetchSleepStats(babyUID string) (*SleepStats, error) {
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	return m.sleepStats, nil
}

func TestManager_WithSleepTracking_PollsAndPublishes(t *testing.T) {
	timestamp := time.Now()
	unixTime := message.UnixTime(timestamp)

	fetcher := &MockCombinedFetcher{
		MockMessageFetcher: &MockMessageFetcher{
			messages: []message.Message{
				{Id: 1, BabyUid: "baby1", Type: "MOTION", Time: unixTime},
			},
		},
		sleepEvents: []SleepEvent{
			{Key: SleepEventKeyFellAsleep, UID: "e1", TimeRaw: float64(time.Now().Unix()), BabyUID: "baby1"},
		},
		sleepStats: &SleepStats{
			Valid:              true,
			TimesWokeUp:        2,
			SleepInterventions: 1,
			TotalAwakeTime:     600,
			TotalSleepTime:     3600,
		},
	}

	mqttClient := NewMockMQTTClient()

	config := ManagerConfig{
		PollerConfig:           DefaultPollerConfig(),
		TopicPrefix:            "nanit",
		Babies:                 []baby.Baby{{UID: "baby1", Name: "Test Baby"}},
		EnableSleepTracking:    true,
		SleepEventPollerConfig: DefaultSleepEventPollerConfig(),
		StatsPollerConfig:      DefaultStatsPollerConfig(),
	}
	// Use shorter intervals for testing
	config.SleepEventPollerConfig.PollInterval = 10 * time.Millisecond
	config.StatsPollerConfig.PollInterval = 10 * time.Millisecond

	manager := NewManagerWithSleepTracking(config, fetcher, fetcher, fetcher, mqttClient)

	// Run for a short time
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	manager.Run(ctx)

	// Verify sleep state was published
	isAsleepTopic := "nanit/babies/baby1/is_asleep"
	payload, ok := mqttClient.GetPublished(isAsleepTopic)
	if !ok {
		t.Errorf("Sleep state not published to %s", isAsleepTopic)
	}
	if payload != "true" {
		t.Errorf("is_asleep = %s, want true", payload)
	}

	// Verify in_bed was published
	inBedTopic := "nanit/babies/baby1/in_bed"
	payload, ok = mqttClient.GetPublished(inBedTopic)
	if !ok {
		t.Errorf("Sleep state not published to %s", inBedTopic)
	}
	if payload != "true" {
		t.Errorf("in_bed = %s, want true", payload)
	}

	// Verify stats were published
	timesWokeUpTopic := "nanit/babies/baby1/times_woke_up"
	payload, ok = mqttClient.GetPublished(timesWokeUpTopic)
	if !ok {
		t.Errorf("Stats not published to %s", timesWokeUpTopic)
	}
	if payload != "2" {
		t.Errorf("times_woke_up = %s, want 2", payload)
	}

	// Verify sleep event was published
	sleepEventTopic := "nanit/babies/baby1/events/fell_asleep"
	_, ok = mqttClient.GetPublished(sleepEventTopic)
	if !ok {
		t.Errorf("Sleep event not published to %s", sleepEventTopic)
	}
}

func TestManager_SleepStateTracker_UpdatesOnEvents(t *testing.T) {
	timestamp := time.Now()
	unixTime := message.UnixTime(timestamp)

	// Start awake, then fell asleep
	fetcher := &MockCombinedFetcher{
		MockMessageFetcher: &MockMessageFetcher{
			messages: []message.Message{
				{Id: 1, BabyUid: "baby1", Type: "MOTION", Time: unixTime},
			},
		},
		sleepEvents: []SleepEvent{
			{Key: SleepEventKeyFellAsleep, UID: "e1", TimeRaw: float64(time.Now().Unix()), BabyUID: "baby1"},
		},
		sleepStats: nil,
	}

	mqttClient := NewMockMQTTClient()

	config := ManagerConfig{
		PollerConfig:           DefaultPollerConfig(),
		TopicPrefix:            "nanit",
		Babies:                 []baby.Baby{{UID: "baby1", Name: "Test Baby"}},
		EnableSleepTracking:    true,
		SleepEventPollerConfig: DefaultSleepEventPollerConfig(),
		StatsPollerConfig:      DefaultStatsPollerConfig(),
	}
	config.SleepEventPollerConfig.PollInterval = 10 * time.Millisecond
	config.StatsPollerConfig.PollInterval = 10 * time.Millisecond

	manager := NewManagerWithSleepTracking(config, fetcher, fetcher, fetcher, mqttClient)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	manager.Run(ctx)

	// Verify state tracker was updated
	tracker := manager.SleepStateTracker("baby1")
	if tracker == nil {
		t.Fatal("Expected sleep state tracker to be created")
	}

	if !tracker.IsAsleep() {
		t.Error("Expected IsAsleep() to be true after FELL_ASLEEP event")
	}
}

func TestManager_SleepTracking_StatsPublishedOnChange(t *testing.T) {
	timestamp := time.Now()
	unixTime := message.UnixTime(timestamp)

	fetcher := &MockCombinedFetcher{
		MockMessageFetcher: &MockMessageFetcher{
			messages: []message.Message{},
		},
		sleepEvents: []SleepEvent{},
		sleepStats: &SleepStats{
			Valid:          true,
			TimesWokeUp:    3,
			TotalAwakeTime: 1200,
			TotalSleepTime: 7200,
		},
	}
	_ = unixTime // Silence unused variable

	mqttClient := NewMockMQTTClient()

	config := ManagerConfig{
		PollerConfig:           DefaultPollerConfig(),
		TopicPrefix:            "nanit",
		Babies:                 []baby.Baby{{UID: "baby1", Name: "Test Baby"}},
		EnableSleepTracking:    true,
		SleepEventPollerConfig: DefaultSleepEventPollerConfig(),
		StatsPollerConfig:      DefaultStatsPollerConfig(),
	}
	config.SleepEventPollerConfig.PollInterval = 10 * time.Millisecond
	config.StatsPollerConfig.PollInterval = 10 * time.Millisecond

	manager := NewManagerWithSleepTracking(config, fetcher, fetcher, fetcher, mqttClient)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	manager.Run(ctx)

	// Verify times_woke_up was published
	topic := "nanit/babies/baby1/times_woke_up"
	payload, ok := mqttClient.GetPublished(topic)
	if !ok {
		t.Errorf("Stats not published to %s", topic)
	}
	if payload != "3" {
		t.Errorf("times_woke_up = %s, want 3", payload)
	}

	// Verify awake_time_today (1200s = 20 min)
	topic = "nanit/babies/baby1/awake_time_today"
	payload, ok = mqttClient.GetPublished(topic)
	if !ok {
		t.Errorf("Stats not published to %s", topic)
	}
	if payload != "20" {
		t.Errorf("awake_time_today = %s, want 20", payload)
	}
}
