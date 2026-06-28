# pgAdmin 4 setup for Dev Platform

The Docker stack runs PostgreSQL with the database and migrations pre-configured.
Use pgAdmin to browse the data — no manual SQL is required unless you use your own PostgreSQL server.

## Option A: Connect pgAdmin to Docker PostgreSQL (recommended)

1. Start the stack:

```powershell
cd C:\Users\Mihir Vora\Projects\dev-platform
docker compose -f docker-compose.local.yml up -d
```

2. In pgAdmin 4, register a new server:

| Field | Value |
|-------|--------|
| **Name** | Dev Platform |
| **Host** | `localhost` |
| **Port** | `5433` |
| **Maintenance database** | `devplatform` |
| **Username** | `devplatform` |
| **Password** | `devplatform` |

3. Save the connection. You should see tables:

- `users`
- `oauth_accounts`
- `projects`
- `build_jobs`
- `build_log_lines`
- `schema_migrations`

Migrations run automatically when you `docker compose -f docker-compose.local.yml up` (via the `migrate` service).

To re-run migrations manually:

```powershell
docker compose -f docker-compose.local.yml run --rm migrate
```

## Option B: Use your local PostgreSQL (port 5432)

If you already have PostgreSQL on port 5432 (common with pgAdmin installs):

1. Open pgAdmin → connect to your local server as `postgres` (or admin user).
2. Open **Query Tool** and run [scripts/pgadmin-setup.sql](pgadmin-setup.sql).
3. Run migrations against localhost:

```powershell
docker run --rm -v "${PWD}/backend/migrations:/migrations" migrate/migrate:v4.17.1 `
  -path /migrations `
  -database "postgres://devplatform:devplatform@host.docker.internal:5432/devplatform?sslmode=disable" up
```

4. Update `.env`:

```env
DATABASE_URL=postgres://devplatform:devplatform@localhost:5432/devplatform?sslmode=disable
```

5. Point `docker-compose.local.yml` `api` and `worker` `DATABASE_URL` to `host.docker.internal:5432` instead of `postgres:5432`, then:

```powershell
docker compose -f docker-compose.local.yml up -d --force-recreate api worker
```

## Verify migrations

In pgAdmin Query Tool:

```sql
SELECT * FROM schema_migrations;
SELECT table_name FROM information_schema.tables WHERE table_schema = 'public';
```

Expected migration version: `1`.
