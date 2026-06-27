CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE git_provider AS ENUM ('github', 'gitlab');
CREATE TYPE runtime_type AS ENUM ('go', 'node', 'python', 'static');
CREATE TYPE environment_name AS ENUM ('dev', 'staging', 'prod');
CREATE TYPE build_status AS ENUM ('queued', 'building', 'scanning', 'deploying', 'success', 'failed', 'cancelled');
CREATE TYPE oauth_provider AS ENUM ('github', 'google', 'gitlab');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE oauth_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider oauth_provider NOT NULL,
    provider_user_id TEXT NOT NULL,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX idx_oauth_accounts_user_id ON oauth_accounts(user_id);

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    git_provider git_provider NOT NULL,
    repo_url TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT 'main',
    runtime_type runtime_type NOT NULL,
    environment environment_name NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_projects_user_id ON projects(user_id);

CREATE TABLE build_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status build_status NOT NULL DEFAULT 'queued',
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    failure_reason TEXT,
    cancelled_at TIMESTAMPTZ,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    building_at TIMESTAMPTZ,
    scanning_at TIMESTAMPTZ,
    deploying_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_build_jobs_project_id ON build_jobs(project_id);
CREATE INDEX idx_build_jobs_status_queued ON build_jobs(status, created_at) WHERE status = 'queued';

CREATE TABLE build_log_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES build_jobs(id) ON DELETE CASCADE,
    seq BIGINT NOT NULL,
    logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (job_id, seq)
);

CREATE INDEX idx_build_log_lines_job_seq ON build_log_lines(job_id, seq);

CREATE OR REPLACE FUNCTION notify_build_log_insert()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('build_log_' || NEW.job_id::text, NEW.seq::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_build_log_notify
AFTER INSERT ON build_log_lines
FOR EACH ROW EXECUTE FUNCTION notify_build_log_insert();
