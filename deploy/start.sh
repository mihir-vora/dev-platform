#!/bin/sh
set -e

LISTEN_PORT="${PORT:-8080}"
export API_PORT=8081
NGINX_CONF=/app/nginx.vibsl.main.conf

shutdown() {
  kill "$WORKER_PID" "$API_PID" "$FRONTEND_PID" 2>/dev/null || true
  nginx -c "$NGINX_CONF" -s quit 2>/dev/null || true
  exit 0
}
trap shutdown TERM INT

if [ -n "$DATABASE_URL" ]; then
  echo "Running database migrations..."
  migrate -path /app/migrations -database "$DATABASE_URL" up
fi

mkdir -p /tmp/nginx/http.d \
  /tmp/nginx/client_body /tmp/nginx/proxy /tmp/nginx/fastcgi \
  /tmp/nginx/uwsgi /tmp/nginx/scgi
sed "s/__LISTEN_PORT__/${LISTEN_PORT}/g" /app/nginx.vibsl.conf.template > /tmp/nginx/http.d/default.conf

echo "Starting worker..."
/app/worker &
WORKER_PID=$!

echo "Starting API on :8081..."
/app/api &
API_PID=$!

echo "Starting frontend on :3000..."
cd /app/frontend
HOSTNAME=127.0.0.1 PORT=3000 node server.js &
FRONTEND_PID=$!

echo "Waiting for API..."
for _ in $(seq 1 60); do
  if wget -qO- "http://127.0.0.1:8081/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "Starting nginx on :${LISTEN_PORT}..."
exec nginx -c "$NGINX_CONF" -g 'daemon off;'
