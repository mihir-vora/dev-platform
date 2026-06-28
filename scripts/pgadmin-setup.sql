-- Optional: run in pgAdmin Query Tool if you use a local PostgreSQL server
-- instead of the Docker postgres service.

CREATE USER devplatform WITH PASSWORD 'devplatform';
CREATE DATABASE devplatform OWNER devplatform;
GRANT ALL PRIVILEGES ON DATABASE devplatform TO devplatform;

-- After creating the database, run migrations with:
--   docker compose run --rm migrate
-- or apply backend/migrations/001_initial.up.sql in pgAdmin.
