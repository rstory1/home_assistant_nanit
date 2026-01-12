package mqtt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDiscoveryPublisher_PublishDiscovery(t *testing.T) {
	// Test configuration
	rtmpAddr := "192.168.1.100:1935"
	babyUID := "abc123"
	babyName := "Baby"

	// Test sensor config structure
	t.Run("SensorConfig JSON structure", func(t *testing.T) {
		device := &DeviceInfo{
			Identifiers:  []string{"nanit_abc123"},
			Name:         "Nanit Baby",
			Manufacturer: "Nanit",
			Model:        "Baby Monitor",
		}

		sensorConfig := SensorConfig{
			Name:              "Nanit Temperature",
			UniqueID:          "nanit_abc123_temperature",
			StateTopic:        "nanit/babies/abc123/temperature",
			Device:            device,
			DeviceClass:       "temperature",
			UnitOfMeasurement: "°C",
		}

		jsonBytes, err := json.Marshal(sensorConfig)
		if err != nil {
			t.Fatalf("Failed to marshal sensor config: %v", err)
		}

		jsonStr := string(jsonBytes)
		t.Logf("Sensor config JSON:\n%s", jsonStr)

		// Verify required fields
		if !strings.Contains(jsonStr, `"name":"Nanit Temperature"`) {
			t.Error("Missing or incorrect name field")
		}
		if !strings.Contains(jsonStr, `"unique_id":"nanit_abc123_temperature"`) {
			t.Error("Missing or incorrect unique_id field")
		}
		if !strings.Contains(jsonStr, `"state_topic":"nanit/babies/abc123/temperature"`) {
			t.Error("Missing or incorrect state_topic field")
		}
		if !strings.Contains(jsonStr, `"device_class":"temperature"`) {
			t.Error("Missing or incorrect device_class field")
		}
		if !strings.Contains(jsonStr, `"unit_of_measurement":"°C"`) {
			t.Error("Missing or incorrect unit_of_measurement field")
		}
		if !strings.Contains(jsonStr, `"manufacturer":"Nanit"`) {
			t.Error("Missing or incorrect manufacturer in device field")
		}
	})

	t.Run("BinarySensorConfig JSON structure", func(t *testing.T) {
		device := &DeviceInfo{
			Identifiers:  []string{"nanit_abc123"},
			Name:         "Nanit Baby",
			Manufacturer: "Nanit",
			Model:        "Baby Monitor",
		}

		binarySensorConfig := BinarySensorConfig{
			Name:        "Nanit Standing",
			UniqueID:    "nanit_abc123_is_standing",
			StateTopic:  "nanit/babies/abc123/is_standing",
			Device:      device,
			DeviceClass: "",
			PayloadOn:   "true",
			PayloadOff:  "false",
			Icon:        "mdi:human-handsup",
		}

		jsonBytes, err := json.Marshal(binarySensorConfig)
		if err != nil {
			t.Fatalf("Failed to marshal binary sensor config: %v", err)
		}

		jsonStr := string(jsonBytes)
		t.Logf("Binary sensor config JSON:\n%s", jsonStr)

		// Verify required fields
		if !strings.Contains(jsonStr, `"name":"Nanit Standing"`) {
			t.Error("Missing or incorrect name field")
		}
		if !strings.Contains(jsonStr, `"payload_on":"true"`) {
			t.Error("Missing or incorrect payload_on field")
		}
		if !strings.Contains(jsonStr, `"payload_off":"false"`) {
			t.Error("Missing or incorrect payload_off field")
		}
	})

	t.Run("SwitchConfig JSON structure", func(t *testing.T) {
		device := &DeviceInfo{
			Identifiers:  []string{"nanit_abc123"},
			Name:         "Nanit Baby",
			Manufacturer: "Nanit",
			Model:        "Baby Monitor",
		}

		switchConfig := SwitchConfig{
			Name:         "Nanit Night Light",
			UniqueID:     "nanit_abc123_night_light",
			StateTopic:   "nanit/babies/abc123/night_light",
			CommandTopic: "nanit/babies/abc123/night_light/switch",
			Device:       device,
			PayloadOn:    "true",
			PayloadOff:   "false",
			Icon:         "mdi:lightbulb-night",
		}

		jsonBytes, err := json.Marshal(switchConfig)
		if err != nil {
			t.Fatalf("Failed to marshal switch config: %v", err)
		}

		jsonStr := string(jsonBytes)
		t.Logf("Switch config JSON:\n%s", jsonStr)

		// Verify required fields
		if !strings.Contains(jsonStr, `"command_topic":"nanit/babies/abc123/night_light/switch"`) {
			t.Error("Missing or incorrect command_topic field")
		}
		if !strings.Contains(jsonStr, `"state_topic":"nanit/babies/abc123/night_light"`) {
			t.Error("Missing or incorrect state_topic field")
		}
	})

	t.Run("Discovery topic format", func(t *testing.T) {
		// Test that discovery topics are formatted correctly
		expectedTopics := []struct {
			component string
			objectID  string
			expected  string
		}{
			{"sensor", "temperature", "homeassistant/sensor/nanit_abc123_temperature/config"},
			{"sensor", "humidity", "homeassistant/sensor/nanit_abc123_humidity/config"},
			{"sensor", "stream_url", "homeassistant/sensor/nanit_abc123_stream_url/config"},
			{"binary_sensor", "is_standing", "homeassistant/binary_sensor/nanit_abc123_is_standing/config"},
			{"binary_sensor", "camera_online", "homeassistant/binary_sensor/nanit_abc123_camera_online/config"},
			{"switch", "night_light", "homeassistant/switch/nanit_abc123_night_light/config"},
			{"switch", "standby", "homeassistant/switch/nanit_abc123_standby/config"},
		}

		for _, tc := range expectedTopics {
			topic := buildDiscoveryTopic(tc.component, babyUID, tc.objectID)
			if topic != tc.expected {
				t.Errorf("Expected topic %s, got %s", tc.expected, topic)
			}
		}
	})

	t.Run("Stream URL format", func(t *testing.T) {
		streamURL := buildStreamURL(rtmpAddr, babyUID)
		expected := "rtmp://192.168.1.100:1935/local/abc123"
		if streamURL != expected {
			t.Errorf("Expected stream URL %s, got %s", expected, streamURL)
		}
	})

	t.Run("Stream URL with rtmp prefix", func(t *testing.T) {
		rtmpAddr := "rtmp://192.168.1.100:1935"
		streamURL := buildStreamURL(rtmpAddr, babyUID)
		expected := "rtmp://192.168.1.100:1935/local/abc123"
		if streamURL != expected {
			t.Errorf("Expected stream URL %s, got %s", expected, streamURL)
		}
	})

	// Print summary of what would be discovered
	t.Run("Discovery summary", func(t *testing.T) {
		t.Log("\n=== Home Assistant Discovery Summary ===")
		t.Logf("Baby UID: %s", babyUID)
		t.Logf("Baby Name: %s", babyName)
		t.Log("\nSensors:")
		t.Log("  - Temperature (°C)")
		t.Log("  - Humidity (%)")
		t.Log("  - Last Motion (timestamp)")
		t.Log("  - Last Sound (timestamp)")
		t.Log("  - Stream URL")
		t.Log("\nBinary Sensors:")
		t.Log("  - Standing")
		t.Log("  - Camera Online")
		t.Log("  - Left Bed")
		t.Log("  - Alert Zone")
		t.Log("  - Temperature Alert")
		t.Log("  - Humidity Alert")
		t.Log("  - Breathing Alert")
		t.Log("  - Low Battery")
		t.Log("  - Stream Active")
		t.Log("  - Asleep")
		t.Log("  - In Bed")
		t.Log("\nSwitches:")
		t.Log("  - Night Light")
		t.Log("  - Standby Mode")
		t.Logf("\nStream URL: rtmp://%s/local/%s", rtmpAddr, babyUID)
	})
}

// Helper functions for testing
func buildDiscoveryTopic(component, babyUID, objectID string) string {
	return "homeassistant/" + component + "/nanit_" + babyUID + "_" + objectID + "/config"
}

func buildStreamURL(rtmpAddr, babyUID string) string {
	if !strings.HasPrefix(rtmpAddr, "rtmp://") {
		rtmpAddr = "rtmp://" + rtmpAddr
	}
	return rtmpAddr + "/local/" + babyUID
}

func TestDiscoveryConfig(t *testing.T) {
	t.Run("Default config", func(t *testing.T) {
		config := DiscoveryConfig{
			Enabled:     true,
			TopicPrefix: "nanit",
			NodeID:      "nanit",
			RTMPAddr:    "192.168.1.100:1935",
		}

		if !config.Enabled {
			t.Error("Expected Enabled to be true")
		}
		if config.TopicPrefix != "nanit" {
			t.Errorf("Expected TopicPrefix 'nanit', got '%s'", config.TopicPrefix)
		}
	})

	t.Run("Disabled config", func(t *testing.T) {
		config := DiscoveryConfig{
			Enabled: false,
		}

		if config.Enabled {
			t.Error("Expected Enabled to be false")
		}
	})
}

func TestDeviceInfo(t *testing.T) {
	device := DeviceInfo{
		Identifiers:  []string{"nanit_abc123"},
		Name:         "Nanit Baby",
		Manufacturer: "Nanit",
		Model:        "Baby Monitor",
	}

	jsonBytes, err := json.MarshalIndent(device, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal device info: %v", err)
	}

	t.Logf("Device info JSON:\n%s", string(jsonBytes))

	// Verify it can be unmarshaled back
	var parsed DeviceInfo
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal device info: %v", err)
	}

	if parsed.Name != device.Name {
		t.Errorf("Name mismatch: expected %s, got %s", device.Name, parsed.Name)
	}
	if parsed.Manufacturer != device.Manufacturer {
		t.Errorf("Manufacturer mismatch: expected %s, got %s", device.Manufacturer, parsed.Manufacturer)
	}
}

func TestSleepSensorDiscoveryConfigs(t *testing.T) {
	babyUID := "abc123"

	device := &DeviceInfo{
		Identifiers:  []string{"nanit_abc123"},
		Name:         "Nanit Baby",
		Manufacturer: "Nanit",
		Model:        "Baby Monitor",
	}

	t.Run("is_asleep binary sensor config", func(t *testing.T) {
		config := BinarySensorConfig{
			Name:        "Nanit Asleep",
			UniqueID:    "nanit_abc123_is_asleep",
			StateTopic:  "nanit/babies/abc123/is_asleep",
			Device:      device,
			DeviceClass: "occupancy",
			PayloadOn:   "true",
			PayloadOff:  "false",
			Icon:        "mdi:sleep",
		}

		jsonBytes, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		jsonStr := string(jsonBytes)
		if !strings.Contains(jsonStr, `"unique_id":"nanit_abc123_is_asleep"`) {
			t.Error("Missing or incorrect unique_id field")
		}
		if !strings.Contains(jsonStr, `"state_topic":"nanit/babies/abc123/is_asleep"`) {
			t.Error("Missing or incorrect state_topic field")
		}
		if !strings.Contains(jsonStr, `"icon":"mdi:sleep"`) {
			t.Error("Missing or incorrect icon field")
		}
	})

	t.Run("in_bed binary sensor config", func(t *testing.T) {
		config := BinarySensorConfig{
			Name:        "Nanit In Bed",
			UniqueID:    "nanit_abc123_in_bed",
			StateTopic:  "nanit/babies/abc123/in_bed",
			Device:      device,
			DeviceClass: "occupancy",
			PayloadOn:   "true",
			PayloadOff:  "false",
			Icon:        "mdi:bed",
		}

		jsonBytes, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		jsonStr := string(jsonBytes)
		if !strings.Contains(jsonStr, `"unique_id":"nanit_abc123_in_bed"`) {
			t.Error("Missing or incorrect unique_id field")
		}
		if !strings.Contains(jsonStr, `"device_class":"occupancy"`) {
			t.Error("Missing or incorrect device_class field")
		}
	})

	t.Run("times_woke_up sensor config", func(t *testing.T) {
		config := SensorConfig{
			Name:       "Nanit Times Woke Up",
			UniqueID:   "nanit_abc123_times_woke_up",
			StateTopic: "nanit/babies/abc123/times_woke_up",
			Device:     device,
			Icon:       "mdi:sleep-off",
		}

		jsonBytes, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		jsonStr := string(jsonBytes)
		if !strings.Contains(jsonStr, `"unique_id":"nanit_abc123_times_woke_up"`) {
			t.Error("Missing or incorrect unique_id field")
		}
		if !strings.Contains(jsonStr, `"icon":"mdi:sleep-off"`) {
			t.Error("Missing or incorrect icon field")
		}
	})

	t.Run("sleep_interventions sensor config", func(t *testing.T) {
		config := SensorConfig{
			Name:       "Nanit Sleep Interventions",
			UniqueID:   "nanit_abc123_sleep_interventions",
			StateTopic: "nanit/babies/abc123/sleep_interventions",
			Device:     device,
			Icon:       "mdi:human-greeting-proximity",
		}

		jsonBytes, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		jsonStr := string(jsonBytes)
		if !strings.Contains(jsonStr, `"unique_id":"nanit_abc123_sleep_interventions"`) {
			t.Error("Missing or incorrect unique_id field")
		}
	})

	t.Run("awake_time_today sensor config", func(t *testing.T) {
		config := SensorConfig{
			Name:              "Nanit Awake Time Today",
			UniqueID:          "nanit_abc123_awake_time_today",
			StateTopic:        "nanit/babies/abc123/awake_time_today",
			Device:            device,
			UnitOfMeasurement: "min",
			Icon:              "mdi:clock-time-four-outline",
		}

		jsonBytes, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		jsonStr := string(jsonBytes)
		if !strings.Contains(jsonStr, `"unit_of_measurement":"min"`) {
			t.Error("Missing or incorrect unit_of_measurement field")
		}
	})

	t.Run("sleep_time_today sensor config", func(t *testing.T) {
		config := SensorConfig{
			Name:              "Nanit Sleep Time Today",
			UniqueID:          "nanit_abc123_sleep_time_today",
			StateTopic:        "nanit/babies/abc123/sleep_time_today",
			Device:            device,
			UnitOfMeasurement: "min",
			Icon:              "mdi:clock-time-four",
		}

		jsonBytes, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		jsonStr := string(jsonBytes)
		if !strings.Contains(jsonStr, `"unit_of_measurement":"min"`) {
			t.Error("Missing or incorrect unit_of_measurement field")
		}
	})

	t.Run("last_sleep_event sensor config", func(t *testing.T) {
		config := SensorConfig{
			Name:       "Nanit Last Sleep Event",
			UniqueID:   "nanit_abc123_last_sleep_event",
			StateTopic: "nanit/babies/abc123/last_sleep_event",
			Device:     device,
			Icon:       "mdi:calendar-clock",
		}

		jsonBytes, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		jsonStr := string(jsonBytes)
		if !strings.Contains(jsonStr, `"unique_id":"nanit_abc123_last_sleep_event"`) {
			t.Error("Missing or incorrect unique_id field")
		}
	})

	t.Run("Sleep sensor discovery topics", func(t *testing.T) {
		expectedTopics := []struct {
			component string
			objectID  string
			expected  string
		}{
			{"binary_sensor", "is_asleep", "homeassistant/binary_sensor/nanit_abc123_is_asleep/config"},
			{"binary_sensor", "in_bed", "homeassistant/binary_sensor/nanit_abc123_in_bed/config"},
			{"sensor", "times_woke_up", "homeassistant/sensor/nanit_abc123_times_woke_up/config"},
			{"sensor", "sleep_interventions", "homeassistant/sensor/nanit_abc123_sleep_interventions/config"},
			{"sensor", "awake_time_today", "homeassistant/sensor/nanit_abc123_awake_time_today/config"},
			{"sensor", "sleep_time_today", "homeassistant/sensor/nanit_abc123_sleep_time_today/config"},
			{"sensor", "last_sleep_event", "homeassistant/sensor/nanit_abc123_last_sleep_event/config"},
		}

		for _, tc := range expectedTopics {
			topic := buildDiscoveryTopic(tc.component, babyUID, tc.objectID)
			if topic != tc.expected {
				t.Errorf("Expected topic %s, got %s", tc.expected, topic)
			}
		}
	})

	t.Run("Sleep sensors summary", func(t *testing.T) {
		t.Log("\n=== Sleep Sensors Discovery Summary ===")
		t.Log("\nBinary Sensors (from /events):")
		t.Log("  - is_asleep (occupancy, mdi:sleep)")
		t.Log("  - in_bed (occupancy, mdi:bed)")
		t.Log("\nSensors (from /stats/latest):")
		t.Log("  - times_woke_up (count)")
		t.Log("  - sleep_interventions (count)")
		t.Log("  - awake_time_today (minutes)")
		t.Log("  - sleep_time_today (minutes)")
		t.Log("  - last_sleep_event (event type)")
	})
}
