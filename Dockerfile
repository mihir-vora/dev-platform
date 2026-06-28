# Production image for VIBSL (single process: API).
# Local full stack: docker compose -f docker-compose.local.yml up --build

FROM golang:1.22-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api

FROM migrate/migrate:v4.17.1 AS migrate-bin

FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget
WORKDIR /app

COPY --from=migrate-bin /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=builder /bin/api /app/api
COPY backend/migrations /app/migrations
COPY backend/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN sed -i 's/\r$//' /app/docker-entrypoint.sh && chmod +x /app/docker-entrypoint.sh

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["start"]
