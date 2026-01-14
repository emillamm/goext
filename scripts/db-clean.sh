#!/usr/bin/env bash
set -e

if ! pg_isready -q -h "$PGHOST" -U "$POSTGRES_USER"; then
  rm -rf $PGDATA
  echo "Data successfully cleaned"
else
  echo "Postgres us running. Service must be stopped before you can clean the data."
fi
