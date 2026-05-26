#!/bin/sh
set -e

echo "Running migrations..."
./mywebapp migrate \
  -migrations /app/migrations \
  -db-host "${DB_HOST:-db}" \
  -db-port "${DB_PORT:-3306}" \
  -db-user "${DB_USER}" \
  -db-pass "${DB_PASS}" \
  -db-name "${DB_NAME:-mywebapp}"

echo "Starting server..."
exec ./mywebapp serve \
  -listen "0.0.0.0:${APP_PORT:-5200}" \
  -db-host "${DB_HOST:-db}" \
  -db-port "${DB_PORT:-3306}" \
  -db-user "${DB_USER}" \
  -db-pass "${DB_PASS}" \
  -db-name "${DB_NAME:-mywebapp}"
