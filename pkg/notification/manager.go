package notification

import (
	"context"
	"sync"
	"time"

	"github.com/scgreenhalgh/home_assistant_nanit/pkg/baby"
	"github.com/rs/zerolog/log"
)

// ManagerConfig configures the NotificationManager
type ManagerConfig struct {
	// Poller configuration
	PollerConfig PollerConfig

	// MQTT topic prefix
	TopicPrefix string

	// Babies to poll for
	Babies []baby.Baby

	// EnableSleepTracking enables polling of /events and /stats/latest endpoints
	EnableSleepTracking bool

	// SleepEventPollerConfig configures the sleep event poller (if enabled)
	SleepEventPollerConfig SleepEventPollerConfig

	// StatsPollerConfig configures the stats poller (if enabled)
	StatsPollerConfig StatsPollerConfig
}

// Manager coordinates polling and publishing of notification events
type Manager struct {
	config    ManagerConfig
	poller    *Poller
	publisher *Publisher
	fetcher   MessageFetcher

	// Sleep tracking (optional)
	sleepEventFetcher SleepEventFetcher
	statsFetcher      StatsFetcher
	sleepEventPollers map[string]*SleepEventPoller // per baby UID
	statsPollers      map[string]*StatsPoller      // per baby UID
	stateTrackers     map[string]*SleepStateTracker
	sleepTrackingMu   sync.RWMutex
}

// NewManager creates a new NotificationManager
func NewManager(config ManagerConfig, fetcher MessageFetcher, mqttClient MQTTClient) *Manager {
	return &Manager{
		config:    config,
		poller:    NewPoller(config.PollerConfig, fetcher),
		publisher: NewPublisher(mqttClient, config.TopicPrefix),
		fetcher:   fetcher,
	}
}

// NewManagerWithSleepTracking creates a NotificationManager with sleep tracking enabled
func NewManagerWithSleepTracking(
	config ManagerConfig,
	fetcher MessageFetcher,
	sleepEventFetcher SleepEventFetcher,
	statsFetcher StatsFetcher,
	mqttClient MQTTClient,
) *Manager {
	return &Manager{
		config:            config,
		poller:            NewPoller(config.PollerConfig, fetcher),
		publisher:         NewPublisher(mqttClient, config.TopicPrefix),
		fetcher:           fetcher,
		sleepEventFetcher: sleepEventFetcher,
		statsFetcher:      statsFetcher,
		sleepEventPollers: make(map[string]*SleepEventPoller),
		statsPollers:      make(map[string]*StatsPoller),
		stateTrackers:     make(map[string]*SleepStateTracker),
	}
}

// SleepStateTracker returns the sleep state tracker for a given baby UID
func (m *Manager) SleepStateTracker(babyUID string) *SleepStateTracker {
	m.sleepTrackingMu.RLock()
	defer m.sleepTrackingMu.RUnlock()
	if m.stateTrackers == nil {
		return nil
	}
	return m.stateTrackers[babyUID]
}

// Run starts the notification polling loop for all babies
// It blocks until the context is cancelled
func (m *Manager) Run(ctx context.Context) error {
	if len(m.config.Babies) == 0 {
		log.Warn().Msg("No babies configured for notification polling")
		return nil
	}

	log.Info().
		Int("babies", len(m.config.Babies)).
		Dur("poll_interval", m.config.PollerConfig.PollInterval).
		Float64("jitter", m.config.PollerConfig.Jitter).
		Bool("sleep_tracking", m.config.EnableSleepTracking).
		Msg("Starting notification polling")

	// Start sleep tracking if enabled
	if m.config.EnableSleepTracking && m.sleepEventFetcher != nil {
		m.startSleepTracking(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Notification polling stopped")
			return ctx.Err()
		default:
		}

		// Poll each baby
		for _, babyInfo := range m.config.Babies {
			if err := m.pollBaby(ctx, babyInfo); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// Error already logged, continue to next baby
			}
		}

		// Wait for next poll interval
		interval := m.poller.NextInterval()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// pollBaby polls and publishes events for a single baby
func (m *Manager) pollBaby(ctx context.Context, babyInfo baby.Baby) error {
	events, err := m.poller.Poll(ctx, babyInfo.UID)
	if err != nil {
		m.poller.IncrementBackoff()
		log.Error().
			Err(err).
			Str("baby_uid", babyInfo.UID).
			Str("baby_name", babyInfo.Name).
			Int("backoff_errors", m.poller.ErrorCount()).
			Dur("next_backoff", m.poller.CurrentBackoff()).
			Msg("Failed to poll for events")
		return err
	}

	// Reset backoff on success
	m.poller.ResetBackoff()

	// Publish each event
	for _, event := range events {
		log.Info().
			Str("type", string(event.Type)).
			Str("baby_uid", babyInfo.UID).
			Str("baby_name", babyInfo.Name).
			Str("event_uid", event.EventUID).
			Time("timestamp", event.Timestamp).
			Msg("Received notification event")

		if err := m.publisher.PublishEvent(event); err != nil {
			log.Error().
				Err(err).
				Str("type", string(event.Type)).
				Str("baby_uid", babyInfo.UID).
				Msg("Failed to publish event")
			// Continue with other events
		}
	}

	return nil
}

// Stats returns the current poller statistics
func (m *Manager) Stats() PollerStats {
	return m.poller.Stats()
}

// startSleepTracking initializes and starts sleep event and stats pollers for all babies
func (m *Manager) startSleepTracking(ctx context.Context) {
	m.sleepTrackingMu.Lock()
	defer m.sleepTrackingMu.Unlock()

	for _, babyInfo := range m.config.Babies {
		// Create state tracker for this baby
		tracker := NewSleepStateTracker()
		m.stateTrackers[babyInfo.UID] = tracker

		// Create sleep event poller
		sleepEventPoller := NewSleepEventPoller(m.config.SleepEventPollerConfig, m.sleepEventFetcher)
		sleepEventPoller.SetStateTracker(tracker)
		m.sleepEventPollers[babyInfo.UID] = sleepEventPoller

		// Create stats poller
		statsPoller := NewStatsPoller(m.config.StatsPollerConfig, m.statsFetcher)
		statsPoller.SetStateTracker(tracker)
		m.statsPollers[babyInfo.UID] = statsPoller

		log.Info().
			Str("baby_uid", babyInfo.UID).
			Str("baby_name", babyInfo.Name).
			Dur("sleep_event_interval", m.config.SleepEventPollerConfig.PollInterval).
			Dur("stats_interval", m.config.StatsPollerConfig.PollInterval).
			Msg("Starting sleep tracking for baby")

		// Start polling goroutines
		go m.pollSleepEventsLoop(ctx, babyInfo.UID, sleepEventPoller)
		go m.pollStatsLoop(ctx, babyInfo.UID, statsPoller)
	}
}

// pollSleepEventsLoop continuously polls for sleep events for a baby
func (m *Manager) pollSleepEventsLoop(ctx context.Context, babyUID string, poller *SleepEventPoller) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		events, err := poller.Poll(ctx, babyUID)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error().
				Err(err).
				Str("baby_uid", babyUID).
				Msg("Failed to poll sleep events")
		} else {
			// Publish new events
			for _, event := range events {
				if err := m.publisher.PublishSleepEvent(event); err != nil {
					log.Error().
						Err(err).
						Str("baby_uid", babyUID).
						Str("event_key", event.Key).
						Msg("Failed to publish sleep event")
				}
			}

			// Publish updated sleep state
			m.sleepTrackingMu.RLock()
			tracker := m.stateTrackers[babyUID]
			m.sleepTrackingMu.RUnlock()

			if tracker != nil {
				snapshot := tracker.GetState()
				if err := m.publisher.PublishSleepState(babyUID, snapshot); err != nil {
					log.Error().
						Err(err).
						Str("baby_uid", babyUID).
						Msg("Failed to publish sleep state")
				}
			}
		}

		// Wait for next poll interval
		interval := poller.NextInterval()
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// pollStatsLoop continuously polls for sleep stats for a baby
func (m *Manager) pollStatsLoop(ctx context.Context, babyUID string, poller *StatsPoller) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stats, err := poller.Poll(ctx, babyUID)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error().
				Err(err).
				Str("baby_uid", babyUID).
				Msg("Failed to poll sleep stats")
		} else if stats != nil {
			// Publish stats if they've changed (HasChanged also updates lastStats internally)
			if poller.HasChanged(stats) {
				if err := m.publisher.PublishSleepStats(babyUID, stats); err != nil {
					log.Error().
						Err(err).
						Str("baby_uid", babyUID).
						Msg("Failed to publish sleep stats")
				}
			}
		}

		// Wait for next poll interval
		interval := poller.NextInterval()
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
