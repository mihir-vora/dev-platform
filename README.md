# Dev Platform

A small developer platform for OAuth login, project management, asynchronous build jobs, and live build logs.

**Repository:** _Add your GitHub repository URL here after creating the remote._

**Demo URL:** Deployment to [https://app-dev.vibsl.com/](https://app-dev.vibsl.com/) is performed by the VIBSL team after review. See [Staging deployment](#staging-deployment) below.

## Features

- OAuth login (GitHub, Google, GitLab)
- Project creation with Git provider, repo URL, branch, runtime, and environment
- Asynchronous build worker with staged statuses: `queued → building → scanning → deploying → success/failed/cancelled`
- PostgreSQL-backed job state and logs
- Live log streaming via Server-Sent Events
- Secret masking in stored logs
- Worker concurrency limit and `SELECT FOR UPDATE SKIP LOCKED` job claiming
- Job cancellation and retry metadata
- Docker Compose for local and staging deployment

## Architecture

See [docs/architecture.md](docs/architecture.md) for data model, auth flow, and worker design.

```text
Browser → Nginx → Next.js (UI)
              └→ Go API (REST + SSE + OAuth)
Go Worker → PostgreSQL / Redis
```

## Prerequisites

- Docker and Docker Compose
- Optional for local development without Docker: Go 1.22+, Node.js 22+, PostgreSQL 16, Redis 7

## Quick Start (Docker)

1. Clone the repository and copy environment variables:

```bash
cp .env.example .env
```

2. Configure OAuth credentials in `.env` (see [OAuth setup](#oauth-setup)).

3. Start the stack:

```bash
docker compose up --build
```

4. Open the app at [http://localhost:8088](http://localhost:8088)

Services:

| Service  | URL |
|----------|-----|
| App (via nginx) | http://localhost:8088 |
| API (direct) | http://localhost:8080 |
| Frontend (direct) | http://localhost:3000 |
| PostgreSQL | localhost:5432 |
| Redis | localhost:6379 |

## OAuth Setup

Create OAuth applications for each provider you want to enable. Use these callback URLs:

| Provider | Callback URL (local) | Callback URL (staging) |
|----------|------------------------|-------------------------|
| GitHub | `http://localhost:8088/api/v1/auth/github/callback` | `https://app-dev.vibsl.com/api/v1/auth/github/callback` |
| Google | `http://localhost:8088/api/v1/auth/google/callback` | `https://app-dev.vibsl.com/api/v1/auth/google/callback` |
| GitLab | `http://localhost:8088/api/v1/auth/gitlab/callback` | `https://app-dev.vibsl.com/api/v1/auth/gitlab/callback` |

Set the resulting client IDs and secrets in `.env`:

```env
GITHUB_CLIENT_ID=...
GITHUB_CLIENT_SECRET=...
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GITLAB_CLIENT_ID=...
GITLAB_CLIENT_SECRET=...
```

Providers without credentials are disabled automatically.

## API Overview

All authenticated endpoints require a session cookie obtained via OAuth.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/me` | Current user |
| POST | `/api/v1/projects` | Create project |
| GET | `/api/v1/projects` | List projects |
| GET | `/api/v1/projects/{id}` | Project details |
| GET | `/api/v1/projects/{id}/builds` | List builds |
| POST | `/api/v1/projects/{id}/builds` | Trigger build |
| GET | `/api/v1/builds/{id}` | Build status |
| GET | `/api/v1/builds/{id}/logs` | Paginated logs |
| GET | `/api/v1/builds/{id}/logs/stream` | SSE live logs |
| POST | `/api/v1/builds/{id}/cancel` | Cancel build |
| POST | `/api/v1/auth/logout` | Log out |

Example:

```bash
curl -b cookies.txt http://localhost:8088/api/v1/projects
```

## Local Development (without Docker)

### Backend

```bash
cd backend
export DATABASE_URL=postgres://devplatform:devplatform@localhost:5432/devplatform?sslmode=disable
export REDIS_URL=redis://localhost:6379/0
export SESSION_SECRET=local-dev-session-secret-change-me-32chars
export FRONTEND_URL=http://localhost:8088
export OAUTH_CALLBACK_BASE_URL=http://localhost:8080
go run ./cmd/api
go run ./cmd/worker
```

Run migrations with [golang-migrate](https://github.com/golang-migrate/migrate):

```bash
migrate -path migrations -database "$DATABASE_URL" up
```

### Frontend

```bash
cd frontend
export NEXT_PUBLIC_API_URL=http://localhost:8080
npm install
npm run dev
```

## Staging Deployment

For staging at `app-dev.vibsl.com`:

1. Set production values in `.env`:

```env
FRONTEND_URL=https://app-dev.vibsl.com
OAUTH_CALLBACK_BASE_URL=https://app-dev.vibsl.com
NEXT_PUBLIC_API_URL=https://app-dev.vibsl.com
COOKIE_SECURE=true
SESSION_SECRET=<strong-random-secret>
```

2. Deploy with production overrides:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

3. Ensure TLS terminates at your load balancer and forwards `X-Forwarded-Proto: https`.

Nginx config: [deploy/nginx.conf](deploy/nginx.conf)

## Environment Variables

See [.env.example](.env.example) for the full list.

## License

MIT
