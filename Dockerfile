# Full-stack production image for VIBSL (nginx + frontend + API + worker).
# Local development: docker compose -f docker-compose.local.yml up --build

FROM golang:1.22-alpine AS go-builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

FROM node:22-alpine AS frontend-builder

WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ .
ENV NEXT_TELEMETRY_DISABLED=1
ENV API_INTERNAL_URL=http://127.0.0.1:8081
ENV NEXT_PUBLIC_API_URL=
RUN npm run build

FROM migrate/migrate:v4.17.1 AS migrate-bin

FROM node:22-alpine

RUN apk add --no-cache nginx ca-certificates wget
WORKDIR /app

COPY --from=migrate-bin /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=go-builder /bin/api /app/api
COPY --from=go-builder /bin/worker /app/worker
COPY backend/migrations /app/migrations
COPY --from=frontend-builder /app/public /app/frontend/public
COPY --from=frontend-builder /app/.next/standalone /app/frontend
COPY --from=frontend-builder /app/.next/static /app/frontend/.next/static
COPY deploy/nginx.vibsl.conf.template /app/nginx.vibsl.conf.template
COPY deploy/start.sh /app/start.sh
RUN sed -i 's/\r$//' /app/start.sh && chmod +x /app/start.sh

ENV PORT=8080
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT:-8080}/health" || exit 1

CMD ["/app/start.sh"]
