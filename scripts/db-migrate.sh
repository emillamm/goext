#!/usr/bin/env bash
set -e

# Start service
./scripts/manage-service.sh start postgresql

# Run migrations
pgmigrate

# Stop service
./scripts/manage-service.sh stop postgresql
