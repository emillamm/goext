#!/usr/bin/env bash
set -e

manage-service.sh start postgresql
pgmigrate
manage-service.sh stop postgresql
