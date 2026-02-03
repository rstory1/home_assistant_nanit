#!/usr/bin/env bash
set -euo pipefail

CONFIG_PATH="/data/options.json"

if [ ! -f "${CONFIG_PATH}" ]; then
  echo "Missing ${CONFIG_PATH}" >&2
  exit 1
fi

get_opt() {
  jq -r "$1 // empty" "${CONFIG_PATH}"
}

export NANIT_EMAIL="$(get_opt '.email')"
export NANIT_PASSWORD="$(get_opt '.password')"
export NANIT_REFRESH_TOKEN="$(get_opt '.refresh_token')"
export NANIT_SESSION_FILE="$(get_opt '.session_file')"

export NANIT_RTMP_ENABLED="$(get_opt '.rtmp_enabled')"
export NANIT_RTMP_ADDR="$(get_opt '.rtmp_addr')"
export NANIT_RTMP_PORT="$(get_opt '.rtmp_port')"
export NANIT_RTMP_ALLOWED_PRESETS="$(get_opt '.rtmp_allowed_presets')"
export NANIT_RTMP_ALLOWED_IPS="$(get_opt '.rtmp_allowed_ips')"
export NANIT_RTMP_LOG_DENIED="$(get_opt '.rtmp_log_denied')"
export NANIT_RTMP_LOG_ALLOWED="$(get_opt '.rtmp_log_allowed')"
export NANIT_PREFER_REMOTE="$(get_opt '.prefer_remote')"

export NANIT_MQTT_ENABLED="$(get_opt '.mqtt_enabled')"
export NANIT_MQTT_BROKER_URL="$(get_opt '.mqtt_broker_url')"
export NANIT_MQTT_CLIENT_ID="$(get_opt '.mqtt_client_id')"
export NANIT_MQTT_USERNAME="$(get_opt '.mqtt_username')"
export NANIT_MQTT_PASSWORD="$(get_opt '.mqtt_password')"
export NANIT_MQTT_PREFIX="$(get_opt '.mqtt_prefix')"
export NANIT_MQTT_DISCOVERY="$(get_opt '.mqtt_discovery')"

export NANIT_NOTIFICATIONS_ENABLED="$(get_opt '.notifications_enabled')"
export NANIT_NOTIFICATIONS_POLL_INTERVAL="$(get_opt '.notifications_poll_interval')"
export NANIT_NOTIFICATIONS_JITTER="$(get_opt '.notifications_jitter')"
export NANIT_NOTIFICATIONS_MAX_BACKOFF="$(get_opt '.notifications_max_backoff')"
export NANIT_NOTIFICATIONS_MESSAGE_TIMEOUT="$(get_opt '.notifications_message_timeout')"
export NANIT_SLEEP_TRACKING_ENABLED="$(get_opt '.sleep_tracking_enabled')"
export NANIT_SLEEP_EVENT_POLL_INTERVAL="$(get_opt '.sleep_event_poll_interval')"
export NANIT_STATS_POLL_INTERVAL="$(get_opt '.stats_poll_interval')"

export NANIT_EVENTS_POLLING="$(get_opt '.events_polling')"
export NANIT_EVENTS_POLLING_INTERVAL="$(get_opt '.events_polling_interval')"
export NANIT_EVENTS_MESSAGE_TIMEOUT="$(get_opt '.events_message_timeout')"

export NANIT_LOG_LEVEL="$(get_opt '.log_level')"

echo "[run.sh] Starting Nanit addon..." >&2
echo "[run.sh] Log level: ${NANIT_LOG_LEVEL}" >&2
echo "[run.sh] Session file: ${NANIT_SESSION_FILE}" >&2
echo "[run.sh] Refresh token set: $([ -n "${NANIT_REFRESH_TOKEN}" ] && echo "yes" || echo "no")" >&2
echo "[run.sh] Executing /app/bin/nanit" >&2

exec /app/bin/nanit
