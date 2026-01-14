#!/usr/bin/env bash
# ------------------------------------------------------------------------------
# Script: manage-service.sh
#
# Description:
#   Starts or stops a Devbox-managed service with reference counting.
#   Useful when multiple processes depend on the same service (e.g. PostgreSQL).
#
#   - On "start": Increments a usage counter and only starts the service if it's not running.
#   - On "stop": Decrements the usage counter and only stops the service when it reaches 0.
#
# Usage:
#   ./manage-service.sh <start|stop> <service-name>
#
# Example:
#   ./manage-service.sh start postgresql
#   ./manage-service.sh stop postgresql
#
# Notes:
#   - Supports PostgreSQL and NATS readiness checks (extendable to other services).
#   - Reference count is stored in /tmp and cleaned up automatically.
#   - Not fully concurrency-safe (can be enhanced with file locking).
# ------------------------------------------------------------------------------

set -euo pipefail
trap 'echo "Script failed at line $LINENO: $BASH_COMMAND"' ERR

ACTION="${1:-}"
SERVICE="${2:-}"
REF_FILE="/tmp/devbox-${SERVICE}.refcount"

if [[ "$ACTION" != "start" && "$ACTION" != "stop" ]]; then
  echo "Usage: $0 <start|stop> <service-name>"
  exit 1
fi

# Make sure the counter file exists and is safe to read/write
init_ref_file() {
  if [ ! -f "$REF_FILE" ]; then
    echo 0 > "$REF_FILE"
  fi
}

increment_ref() {
  init_ref_file
  local count
  count=$(<"$REF_FILE")
  echo $((count + 1)) > "$REF_FILE"
}

decrement_ref() {
  init_ref_file
  local count
  count=$(<"$REF_FILE")
  if (( count > 0 )); then
    echo $((count - 1)) > "$REF_FILE"
  fi
}

get_refcount() {
  init_ref_file
  cat "$REF_FILE"
}

is_service_started() {
  case "$SERVICE" in
    postgresql)
      pg_isready -q -h "$PGHOST" -U "$POSTGRES_USER" >/dev/null 2>&1
      return $?  # Safe return; won't break under `set -e
      ;;
    *)
      return 0  # Assume ready for unknown services
      ;;
  esac
}

wait_for_service_ready() {
  local max_retries=30
  local wait_time=0.5
  local count=0
  echo "\nWaiting for $SERVICE to become ready..."
  until is_service_started || false; do
    sleep "$wait_time"
    ((++count))
    if (( count >= max_retries )); then
      echo "Timeout waiting for $SERVICE to become ready."
      return 1
    fi
  done
}

if [[ "$ACTION" == "start" ]]; then
  if ! is_service_started; then
    rm -f "$REF_FILE"
    echo "Starting $SERVICE..."
    devbox services start "$SERVICE"
    wait_for_service_ready
  else
    # If service is already started and refcount is 0, make sure to set it to 1
    # so the services isn't terminated by the calling process.
    count=$(get_refcount)
    if (( count == 0 )); then
      increment_ref
    fi
  fi
  increment_ref
  #echo "$SERVICE refcount: $(get_refcount)"
elif [[ "$ACTION" == "stop" ]]; then
  if is_service_started; then
    decrement_ref
    count=$(get_refcount)
    #echo "$SERVICE refcount: $count"
    if (( count == 0 )); then
      echo "Stopping $SERVICE..."
      devbox services stop "$SERVICE"
      rm -f "$REF_FILE"
    fi
  else
    rm -f "$REF_FILE"
  fi
fi
