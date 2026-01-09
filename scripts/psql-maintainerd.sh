#!/usr/bin/env bash
set -euo pipefail

SQL="${1:-}"
if [ -z "$SQL" ]; then
  echo "Usage: $0 \"<sql>\""
  echo "Example: $0 \"select count(*) from projects;\""
  exit 1
fi

DB_HOST="${MD_DB_HOST:-10.0.10.121}"
DB_PORT="${MD_DB_PORT:-5432}"
DB_NAME="${MD_DB_NAME:-maintainerd}"
DB_USER="${MD_DB_USER:-admin}"
DB_PASSWORD="${MD_DB_PASSWORD:-}"

if [ -z "$DB_PASSWORD" ]; then
  echo "Set MD_DB_PASSWORD before running."
  exit 1
fi

kubectl -n maintainerd run psql-client --rm -it --restart=Never \
  --image=postgres:16-alpine \
  --env="PGPASSWORD=${DB_PASSWORD}" -- \
  psql "host=${DB_HOST} port=${DB_PORT} user=${DB_USER} dbname=${DB_NAME} sslmode=require" \
  -c "$SQL"
