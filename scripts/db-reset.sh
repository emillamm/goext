#!/usr/bin/env bash
set -e

# Check if postgres is currently running
POSTGRES_WAS_RUNNING=false
if "$DEVBOX_SCRIPTS_DIR/services/postgresql/ready.sh" 2>/dev/null; then
  POSTGRES_WAS_RUNNING=true
  echo "PostgreSQL is running. Stopping service..."
  manage-service.sh stop postgresql
fi

echo "Cleaning database..."
manage-service.sh clean postgresql

echo "Initializing database..."
manage-service.sh init postgresql

sleep 1

echo "Running migrations..."
./scripts/db-migrate.sh

if [ "$POSTGRES_WAS_RUNNING" = true ]; then
  echo "Restarting PostgreSQL..."
  manage-service.sh start postgresql
fi

echo "Database reset complete."
