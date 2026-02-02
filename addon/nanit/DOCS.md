# Nanit Local Streaming

## Setup

1. Copy this folder to your Home Assistant OS add-ons directory (local add-ons).
2. Install the add-on and open its configuration page.
3. Provide either `refresh_token` (recommended) or `email` and `password`.
4. Set `rtmp_addr` to the IP:port reachable by the camera (or leave empty to auto-detect).
5. Start the add-on and watch the logs for the `baby_uid` value.

> Refresh tokens are created using the `init-nanit.sh` script from this repository. The generated session file should be stored in `/data`.

## MQTT

If you enable MQTT, set `mqtt_broker_url` to your broker (e.g., `tcp://core-mosquitto:1883`) and Home Assistant auto-discovery will be enabled by default.

## Configuration options

- `email`: Nanit account email (optional if using refresh token)
- `password`: Nanit account password (optional if using refresh token)
- `refresh_token`: Nanit refresh token (recommended)
- `session_file`: Session file path (default: `/data/session.json`)
- `rtmp_enabled`: Enable RTMP streaming
- `rtmp_addr`: Public RTMP address (IP:port). Leave empty for auto-detect.
- `rtmp_port`: RTMP port (default: 1935)
- `rtmp_allowed_presets`: IP allowlist presets (e.g., `hassio`)
- `rtmp_allowed_ips`: Explicit IP/CIDR allowlist
- `rtmp_log_denied`: Log denied connections
- `rtmp_log_allowed`: Log allowed connections
- `prefer_remote`: Start with remote stream first
- `mqtt_enabled`: Enable MQTT integration
- `mqtt_broker_url`: Broker URL (required if MQTT enabled)
- `mqtt_client_id`: MQTT client ID
- `mqtt_username`: MQTT username
- `mqtt_password`: MQTT password
- `mqtt_prefix`: Topic prefix
- `mqtt_discovery`: Enable Home Assistant MQTT discovery
- `notifications_enabled`: Enable notification polling
- `notifications_poll_interval`: Poll interval in seconds
- `notifications_jitter`: Jitter factor 0–1
- `notifications_max_backoff`: Max backoff (seconds)
- `notifications_message_timeout`: Message timeout (seconds)
- `sleep_tracking_enabled`: Enable sleep tracking sensors
- `sleep_event_poll_interval`: Sleep event poll interval (seconds)
- `stats_poll_interval`: Sleep stats poll interval (seconds)
- `events_polling`: Enable legacy events polling
- `events_polling_interval`: Legacy events poll interval (seconds)
- `events_message_timeout`: Legacy events message timeout (seconds)
- `log_level`: Log level (`trace`, `debug`, `info`, `warn`, `error`)
