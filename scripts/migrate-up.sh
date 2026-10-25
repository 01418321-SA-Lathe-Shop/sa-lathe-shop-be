#!/bin/sh
# Applies pending migrations inside the API image (Railway pre-deploy command).
# Requires DATABASE_URL; never prints it. Down migrations are not run here.
set -eu
if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required to apply migrations" >&2
  exit 1
fi
exec /app/migrate -path /app/migrations -database "$DATABASE_URL" up
