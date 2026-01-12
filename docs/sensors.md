# Sensors

App exposes cam sensors by publishing the updates to MQTT. See `NANIT_MQTT_*` variables in the [.env.sample](../.env.sample) file for configuration.

## Sensor Topics

### Real-time Sensors (from WebSocket)

These sensors are updated in real-time via the WebSocket connection to the camera:

- `nanit/babies/{baby_uid}/temperature` - Temperature in degrees Celsius (float)
- `nanit/babies/{baby_uid}/humidity` - Humidity in percent (float)
- `nanit/babies/{baby_uid}/is_night` - Night mode status (bool)
- `nanit/babies/{baby_uid}/night_light` - Night light on/off status (bool)
- `nanit/babies/{baby_uid}/standby` - Camera standby mode (bool)

### Streaming Sensors

These sensors track the streaming state:

- `nanit/babies/{baby_uid}/stream_url` - Current RTMP stream URL (string)
- `nanit/babies/{baby_uid}/stream_source` - Stream source: "local", "remote", or "none" (string)
- `nanit/babies/{baby_uid}/is_stream_alive` - Whether a stream is currently active (bool)

### Notification Event Sensors

When notification polling is enabled (`NANIT_NOTIFICATIONS_ENABLED=true`), these event sensors are published:

- `nanit/babies/{baby_uid}/motion_timestamp` - Last motion detection time (ISO 8601)
- `nanit/babies/{baby_uid}/sound_timestamp` - Last sound detection time (ISO 8601)
- `nanit/babies/{baby_uid}/camera_online` - Camera online status (bool)
- `nanit/babies/{baby_uid}/is_standing` - Baby standing detected (bool)
- `nanit/babies/{baby_uid}/left_bed` - Toddler left bed detected (bool)
- `nanit/babies/{baby_uid}/alert_zone_triggered` - Alert zone triggered (bool)
- `nanit/babies/{baby_uid}/temperature_alert` - Temperature alert active (bool)
- `nanit/babies/{baby_uid}/humidity_alert` - Humidity alert active (bool)
- `nanit/babies/{baby_uid}/breathing_alert` - Breathing alert active (bool)
- `nanit/babies/{baby_uid}/low_battery` - Low battery alert (bool)

### Sleep Tracking Sensors

When sleep tracking is enabled (`NANIT_SLEEP_TRACKING_ENABLED=true`, default when notifications enabled), these sensors track sleep state:

- `nanit/babies/{baby_uid}/is_asleep` - Baby is currently asleep (bool)
- `nanit/babies/{baby_uid}/in_bed` - Baby is currently in bed (bool)
- `nanit/babies/{baby_uid}/last_sleep_event` - Last sleep event type (string: FELL_ASLEEP, WOKE_UP, PUT_IN_BED, etc.)
- `nanit/babies/{baby_uid}/times_woke_up` - Number of times baby woke up today (int)
- `nanit/babies/{baby_uid}/sleep_interventions` - Number of sleep interventions today (int)
- `nanit/babies/{baby_uid}/awake_time_today` - Total awake time today in minutes (int)
- `nanit/babies/{baby_uid}/sleep_time_today` - Total sleep time today in minutes (int)

### Event Topics

Events are also published to event topics for automation:

- `nanit/babies/{baby_uid}/events/motion` - Motion event payload (JSON)
- `nanit/babies/{baby_uid}/events/sound` - Sound event payload (JSON)
- `nanit/babies/{baby_uid}/events/fell_asleep` - Fell asleep event (JSON)
- `nanit/babies/{baby_uid}/events/woke_up` - Woke up event (JSON)
- ... and other event types

## Home Assistant Integration

When MQTT Discovery is enabled (`NANIT_MQTT_DISCOVERY=true`, default), all sensors are automatically discovered by Home Assistant. See [Home Assistant setup](./home-assistant.md) for more details.

## Troubleshooting

In case you run into trouble and need to see what is going on, you can try using [MQTT Explorer](http://mqtt-explorer.com/).

If sensors show "Unknown" status:

1. **Sleep sensors not updating**: Ensure `NANIT_NOTIFICATIONS_ENABLED=true` and `NANIT_SLEEP_TRACKING_ENABLED=true`
2. **Alert sensors not updating**: These only update when the corresponding event occurs (baby standing, leaving bed, etc.)
3. **Check logs**: Look for polling errors in the application logs
