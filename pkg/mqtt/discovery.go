package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
)

// DiscoveryConfig holds configuration for Home Assistant MQTT discovery
type DiscoveryConfig struct {
	Enabled      bool
	TopicPrefix  string // e.g., "nanit"
	NodeID       string // e.g., "nanit"
	RTMPAddr     string // RTMP server address for stream URL
}

// DeviceInfo represents a Home Assistant device
type DeviceInfo struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model,omitempty"`
}

// SensorConfig represents a Home Assistant sensor discovery config
type SensorConfig struct {
	Name              string      `json:"name"`
	UniqueID          string      `json:"unique_id"`
	StateTopic        string      `json:"state_topic"`
	Device            *DeviceInfo `json:"device,omitempty"`
	DeviceClass       string      `json:"device_class,omitempty"`
	UnitOfMeasurement string      `json:"unit_of_measurement,omitempty"`
	Icon              string      `json:"icon,omitempty"`
	ValueTemplate     string      `json:"value_template,omitempty"`
}

// BinarySensorConfig represents a Home Assistant binary sensor discovery config
type BinarySensorConfig struct {
	Name         string      `json:"name"`
	UniqueID     string      `json:"unique_id"`
	StateTopic   string      `json:"state_topic"`
	Device       *DeviceInfo `json:"device,omitempty"`
	DeviceClass  string      `json:"device_class,omitempty"`
	PayloadOn    string      `json:"payload_on,omitempty"`
	PayloadOff   string      `json:"payload_off,omitempty"`
	Icon         string      `json:"icon,omitempty"`
}

// SwitchConfig represents a Home Assistant switch discovery config
type SwitchConfig struct {
	Name          string      `json:"name"`
	UniqueID      string      `json:"unique_id"`
	StateTopic    string      `json:"state_topic"`
	CommandTopic  string      `json:"command_topic"`
	Device        *DeviceInfo `json:"device,omitempty"`
	PayloadOn     string      `json:"payload_on,omitempty"`
	PayloadOff    string      `json:"payload_off,omitempty"`
	Icon          string      `json:"icon,omitempty"`
}

// Publisher handles MQTT discovery message publishing
type DiscoveryPublisher struct {
	config DiscoveryConfig
	client MQTT.Client
}

// NewDiscoveryPublisher creates a new discovery publisher
func NewDiscoveryPublisher(config DiscoveryConfig, client MQTT.Client) *DiscoveryPublisher {
	return &DiscoveryPublisher{
		config: config,
		client: client,
	}
}

// PublishDiscovery publishes all discovery configs for a baby
func (p *DiscoveryPublisher) PublishDiscovery(babyUID string, babyName string) error {
	if !p.config.Enabled {
		return nil
	}

	device := &DeviceInfo{
		Identifiers:  []string{fmt.Sprintf("nanit_%s", babyUID)},
		Name:         fmt.Sprintf("Nanit %s", babyName),
		Manufacturer: "Nanit",
		Model:        "Baby Monitor",
	}

	// Publish all sensor configs
	if err := p.publishSensors(babyUID, device); err != nil {
		return err
	}

	// Publish all binary sensor configs
	if err := p.publishBinarySensors(babyUID, device); err != nil {
		return err
	}

	// Publish all switch configs
	if err := p.publishSwitches(babyUID, device); err != nil {
		return err
	}

	// Publish stream URL sensor
	if err := p.publishStreamURL(babyUID, device); err != nil {
		return err
	}

	// Publish initial states for sensors that need them
	if err := p.publishInitialStates(babyUID); err != nil {
		return err
	}

	log.Info().
		Str("baby_uid", babyUID).
		Msg("Published MQTT discovery configs for Home Assistant")

	return nil
}

// publishInitialStates publishes initial values for sensors that need them
func (p *DiscoveryPublisher) publishInitialStates(babyUID string) error {
	// Camera is online since we've successfully connected to Nanit API
	cameraOnlineTopic := fmt.Sprintf("%s/babies/%s/camera_online", p.config.TopicPrefix, babyUID)
	if token := p.client.Publish(cameraOnlineTopic, 0, true, "true"); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish camera_online initial state: %w", token.Error())
	}

	log.Debug().
		Str("topic", cameraOnlineTopic).
		Msg("Published initial camera_online state")

	// Stream source defaults to Local (will be updated when stream starts)
	streamSourceTopic := fmt.Sprintf("%s/babies/%s/stream_source", p.config.TopicPrefix, babyUID)
	if token := p.client.Publish(streamSourceTopic, 0, true, "Local"); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish stream_source initial state: %w", token.Error())
	}

	log.Debug().
		Str("topic", streamSourceTopic).
		Msg("Published initial stream_source state")

	return nil
}

func (p *DiscoveryPublisher) publishSensors(babyUID string, device *DeviceInfo) error {
	sensors := []struct {
		id          string
		name        string
		deviceClass string
		unit        string
		icon        string
	}{
		{"temperature", "Temperature", "temperature", "°C", ""},
		{"humidity", "Humidity", "humidity", "%", ""},
		{"motion_timestamp", "Last Motion", "timestamp", "", "mdi:motion-sensor"},
		{"sound_timestamp", "Last Sound", "timestamp", "", "mdi:volume-high"},
		{"stream_source", "Stream Source", "", "", "mdi:video-switch"},
		// Sleep stats sensors (from /stats/latest API)
		{"times_woke_up", "Times Woke Up", "", "", "mdi:sleep-off"},
		{"sleep_interventions", "Sleep Interventions", "", "", "mdi:human-greeting-proximity"},
		{"awake_time_today", "Awake Time Today", "", "min", "mdi:clock-time-four-outline"},
		{"sleep_time_today", "Sleep Time Today", "", "min", "mdi:clock-time-four"},
		{"last_sleep_event", "Last Sleep Event", "", "", "mdi:calendar-clock"},
	}

	for _, s := range sensors {
		config := SensorConfig{
			Name:              fmt.Sprintf("Nanit %s", s.name),
			UniqueID:          fmt.Sprintf("nanit_%s_%s", babyUID, s.id),
			StateTopic:        fmt.Sprintf("%s/babies/%s/%s", p.config.TopicPrefix, babyUID, s.id),
			Device:            device,
			DeviceClass:       s.deviceClass,
			UnitOfMeasurement: s.unit,
			Icon:              s.icon,
		}

		if err := p.publishConfig("sensor", babyUID, s.id, config); err != nil {
			return err
		}
	}

	return nil
}

func (p *DiscoveryPublisher) publishBinarySensors(babyUID string, device *DeviceInfo) error {
	sensors := []struct {
		id          string
		name        string
		deviceClass string
		icon        string
	}{
		{"is_standing", "Standing", "", "mdi:human-handsup"},
		{"camera_online", "Camera Online", "connectivity", ""},
		{"left_bed", "Left Bed", "", "mdi:bed-empty"},
		{"alert_zone_triggered", "Alert Zone", "motion", ""},
		{"temperature_alert", "Temperature Alert", "problem", "mdi:thermometer-alert"},
		{"humidity_alert", "Humidity Alert", "problem", "mdi:water-percent-alert"},
		{"breathing_alert", "Breathing Alert", "problem", "mdi:lungs"},
		{"is_stream_alive", "Stream Active", "connectivity", "mdi:video"},
		// Sleep state binary sensors (from /events API)
		{"is_asleep", "Asleep", "occupancy", "mdi:sleep"},
		{"in_bed", "In Bed", "occupancy", "mdi:bed"},
	}

	for _, s := range sensors {
		config := BinarySensorConfig{
			Name:        fmt.Sprintf("Nanit %s", s.name),
			UniqueID:    fmt.Sprintf("nanit_%s_%s", babyUID, s.id),
			StateTopic:  fmt.Sprintf("%s/babies/%s/%s", p.config.TopicPrefix, babyUID, s.id),
			Device:      device,
			DeviceClass: s.deviceClass,
			PayloadOn:   "true",
			PayloadOff:  "false",
			Icon:        s.icon,
		}

		if err := p.publishConfig("binary_sensor", babyUID, s.id, config); err != nil {
			return err
		}
	}

	return nil
}

func (p *DiscoveryPublisher) publishSwitches(babyUID string, device *DeviceInfo) error {
	switches := []struct {
		id   string
		name string
		icon string
	}{
		{"night_light", "Night Light", "mdi:lightbulb-night"},
		{"standby", "Standby Mode", "mdi:power-standby"},
	}

	for _, s := range switches {
		config := SwitchConfig{
			Name:         fmt.Sprintf("Nanit %s", s.name),
			UniqueID:     fmt.Sprintf("nanit_%s_%s", babyUID, s.id),
			StateTopic:   fmt.Sprintf("%s/babies/%s/%s", p.config.TopicPrefix, babyUID, s.id),
			CommandTopic: fmt.Sprintf("%s/babies/%s/%s/switch", p.config.TopicPrefix, babyUID, s.id),
			Device:       device,
			PayloadOn:    "true",
			PayloadOff:   "false",
			Icon:         s.icon,
		}

		if err := p.publishConfig("switch", babyUID, s.id, config); err != nil {
			return err
		}
	}

	return nil
}

func (p *DiscoveryPublisher) publishStreamURL(babyUID string, device *DeviceInfo) error {
	if p.config.RTMPAddr == "" {
		return nil
	}

	// Clean up the RTMP address
	rtmpAddr := p.config.RTMPAddr
	if !strings.HasPrefix(rtmpAddr, "rtmp://") {
		rtmpAddr = "rtmp://" + rtmpAddr
	}

	streamURL := fmt.Sprintf("%s/local/%s", rtmpAddr, babyUID)

	// Publish discovery config for stream URL sensor
	config := SensorConfig{
		Name:       "Nanit Stream URL",
		UniqueID:   fmt.Sprintf("nanit_%s_stream_url", babyUID),
		StateTopic: fmt.Sprintf("%s/babies/%s/stream_url", p.config.TopicPrefix, babyUID),
		Device:     device,
		Icon:       "mdi:video",
	}

	if err := p.publishConfig("sensor", babyUID, "stream_url", config); err != nil {
		return err
	}

	// Publish the actual stream URL value
	topic := fmt.Sprintf("%s/babies/%s/stream_url", p.config.TopicPrefix, babyUID)
	if token := p.client.Publish(topic, 0, true, streamURL); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish stream URL: %w", token.Error())
	}

	log.Debug().
		Str("topic", topic).
		Str("url", streamURL).
		Msg("Published stream URL")

	return nil
}

func (p *DiscoveryPublisher) publishConfig(component, babyUID, objectID string, config interface{}) error {
	// Topic format: homeassistant/{component}/nanit_{baby_uid}_{object_id}/config
	topic := fmt.Sprintf("homeassistant/%s/nanit_%s_%s/config", component, babyUID, objectID)

	payload, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal discovery config: %w", err)
	}

	log.Trace().
		Str("topic", topic).
		RawJSON("config", payload).
		Msg("Publishing discovery config")

	// Publish with retain=true so HA discovers on restart
	if token := p.client.Publish(topic, 0, true, string(payload)); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish discovery config: %w", token.Error())
	}

	return nil
}

// RemoveDiscovery removes all discovery configs for a baby (publishes empty payloads)
func (p *DiscoveryPublisher) RemoveDiscovery(babyUID string) error {
	if !p.config.Enabled {
		return nil
	}

	components := []string{"sensor", "binary_sensor", "switch"}
	objectIDs := []string{
		// Sensors
		"temperature", "humidity", "motion_timestamp", "sound_timestamp", "stream_url", "stream_source",
		// Sleep stats sensors
		"times_woke_up", "sleep_interventions", "awake_time_today", "sleep_time_today", "last_sleep_event",
		// Binary sensors
		"is_standing", "camera_online", "left_bed", "alert_zone_triggered",
		"temperature_alert", "humidity_alert", "breathing_alert", "is_stream_alive",
		// Sleep state binary sensors
		"is_asleep", "in_bed",
		// Switches
		"night_light", "standby",
	}

	for _, component := range components {
		for _, objectID := range objectIDs {
			topic := fmt.Sprintf("homeassistant/%s/nanit_%s_%s/config", component, babyUID, objectID)
			// Empty payload removes the entity
			if token := p.client.Publish(topic, 0, true, ""); token.Wait() && token.Error() != nil {
				log.Warn().Err(token.Error()).Str("topic", topic).Msg("Failed to remove discovery config")
			}
		}
	}

	log.Info().
		Str("baby_uid", babyUID).
		Msg("Removed MQTT discovery configs")

	return nil
}
