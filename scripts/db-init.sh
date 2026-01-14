#!/usr/bin/env bash
set -e

# Check if database is initialized
if [ ! -f "$PGDATA/PG_VERSION" ]; then
  echo "Initializing PostgreSQL data directory..."
  # Some local extensions need to be added via shared_preload_libraries
  initdb -D "$PGDATA" -U "$POSTGRES_USER"
else
  # Database already initialized
  exit 0
fi

# Start service
./scripts/manage-service.sh start postgresql

# Create primary database
if ! psql -d postgres -U "$POSTGRES_USER" -Atqc "SELECT 1 FROM pg_database WHERE datname = '$POSTGRES_DATABASE'" | grep -q 1; then
  createdb "$POSTGRES_DATABASE" -h "$PGHOST" -U "$POSTGRES_USER"
fi

# Stop service
./scripts/manage-service.sh stop postgresql
