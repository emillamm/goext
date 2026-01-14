#!/usr/bin/env bash
# ------------------------------------------------------------------------------
# Script: db-reset.sh
#
# Description:
#   Resets the PostgreSQL database by cleaning, initializing, and migrating.
#   Automatically handles the case where PostgreSQL is already running by
#   stopping it before reset and restarting it after completion.
#
# Usage:
#   ./db-reset.sh
#
# Notes:
#   - Runs db-clean.sh, db-init.sh, and db-migrate.sh in sequence
#   - If PostgreSQL is running, it will be stopped and restarted automatically
# ------------------------------------------------------------------------------

set -e

# Check if postgres is currently running
POSTGRES_WAS_RUNNING=false
if pg_isready -q -h "$PGHOST" -U "$POSTGRES_USER" 2>/dev/null; then
  POSTGRES_WAS_RUNNING=true
  echo "PostgreSQL is running. Stopping service..."
  ./scripts/manage-service.sh stop postgresql
fi

# Run the reset sequence
echo "Cleaning database..."
./scripts/db-clean.sh

echo "Initializing database..."
./scripts/db-init.sh

# Small delay to ensure services are ready
sleep 1

echo "Running migrations..."
./scripts/db-migrate.sh

# Restart postgres if it was running before
if [ "$POSTGRES_WAS_RUNNING" = true ]; then
  echo "Restarting PostgreSQL..."
  ./scripts/manage-service.sh start postgresql
fi

echo "Database reset complete."
