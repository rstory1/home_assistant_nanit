# Changelog

All notable changes to this project will be documented in this file.

## [2.2.0] - 2025-01-12

This release is a major update from the indiefan/home_assistant_nanit fork, adding security features, stream reliability, and Home Assistant integration improvements.

### Breaking Changes

- **RTMP IP Access Control**: The RTMP server now denies all connections by default. You must configure allowed IPs or presets:
  ```bash
  -e NANIT_RTMP_ALLOWED_PRESETS=private  # Allow all private network IPs
  # OR
  -e NANIT_RTMP_ALLOWED_IPS=192.168.1.0/24  # Explicit whitelist
  ```
  Without this configuration, your stream will not work.

### Added

- **IP-based RTMP security** with deny-by-default model
  - Presets: `hassio`, `frigate`, `docker`, `localhost`, `private`
  - Explicit IP/CIDR whitelist support
  - Connection logging for denied/allowed attempts

- **Stream failover** between local and remote (cloud) streaming
  - Automatic fallback when local streaming fails
  - PersistentBroadcaster maintains client connections during source switches
  - Auto-restoration with exponential backoff

- **Notification event polling** (12+ event types)
  - Motion, sound, crying, standing, left bed, alert zone
  - Temperature/humidity/breathing alerts
  - Camera online/offline status
  - Low battery warnings

- **Sleep tracking sensors**
  - `is_asleep` / `in_bed` binary sensors
  - Sleep statistics from Nanit API

- **Home Assistant MQTT auto-discovery**
  - Sensors, binary sensors, and switches auto-register
  - Stream URL sensor for easy camera setup

- **Dynamic auth token refresh** for RTMP reconnections

### Changed

- Module path updated to `github.com/scgreenhalgh/home_assistant_nanit`
- Docker image: `seangreenhalgh/nanit`
- Container runs as non-root user for security
- Upgraded dependencies

## [1.x] - Previous releases

See [indiefan/home_assistant_nanit](https://github.com/indiefan/home_assistant_nanit) for earlier history.

Initial 2FA support was added by indiefan when Nanit made it mandatory.

Original streaming implementation by [Adam Staněk](https://gitlab.com/adam.stanek/nanit).
