#!/bin/sh
set -e

case "$1" in
  api)
    exec /app/api
    ;;
  worker)
    exec /app/worker
    ;;
  migrate)
    exec migrate -path /app/migrations -database "$DATABASE_URL" up
    ;;
  start)
    if [ -n "$DATABASE_URL" ]; then
      migrate -path /app/migrations -database "$DATABASE_URL" up
    fi
    exec /app/api
    ;;
  *)
    exec "$@"
    ;;
esac
