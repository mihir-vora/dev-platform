package db

import (
	"context"
	"fmt"
	"time"

	"github.com/dev-platform/backend/internal/logs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OAuthProfile struct {
	Provider       string
	ProviderUserID string
	Email          string
	Name           string
	AvatarURL      string
}

func (s *Store) UpsertOAuthUser(ctx context.Context, profile OAuthProfile) (*User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT user_id FROM oauth_accounts
		WHERE provider = $1::oauth_provider AND provider_user_id = $2
	`, profile.Provider, profile.ProviderUserID).Scan(&userID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `
			INSERT INTO users (email, name, avatar_url)
			VALUES ($1, $2, $3)
			ON CONFLICT (email) DO UPDATE SET
				name = EXCLUDED.name,
				avatar_url = EXCLUDED.avatar_url,
				updated_at = NOW()
			RETURNING id
		`, profile.Email, profile.Name, profile.AvatarURL).Scan(&userID)
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO oauth_accounts (user_id, provider, provider_user_id, created_by)
			VALUES ($1, $2::oauth_provider, $3, $1)
			ON CONFLICT (provider, provider_user_id) DO UPDATE SET
				user_id = EXCLUDED.user_id,
				updated_at = NOW()
		`, userID, profile.Provider, profile.ProviderUserID)
		if err != nil {
			return nil, err
		}
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE users SET name = $2, avatar_url = $3, updated_at = NOW()
			WHERE id = $1
		`, userID, profile.Name, profile.AvatarURL)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, userID)
}

func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

type Project struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Name        string    `json:"name"`
	GitProvider string    `json:"git_provider"`
	RepoURL     string    `json:"repo_url"`
	Branch      string    `json:"branch"`
	RuntimeType string    `json:"runtime_type"`
	Environment string    `json:"environment"`
	CreatedBy   uuid.UUID `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateProjectInput struct {
	Name        string
	GitProvider string
	RepoURL     string
	Branch      string
	RuntimeType string
	Environment string
	UserID      uuid.UUID
}

func (s *Store) CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error) {
	var project Project
	err := s.pool.QueryRow(ctx, `
		INSERT INTO projects (
			user_id, name, git_provider, repo_url, branch, runtime_type, environment, created_by
		) VALUES ($1, $2, $3::git_provider, $4, $5, $6::runtime_type, $7::environment_name, $1)
		RETURNING id, user_id, name, git_provider::text, repo_url, branch,
			runtime_type::text, environment::text, created_by, created_at, updated_at
	`, input.UserID, input.Name, input.GitProvider, input.RepoURL, input.Branch,
		input.RuntimeType, input.Environment).Scan(
		&project.ID, &project.UserID, &project.Name, &project.GitProvider, &project.RepoURL,
		&project.Branch, &project.RuntimeType, &project.Environment, &project.CreatedBy,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (s *Store) ListProjects(ctx context.Context, userID uuid.UUID) ([]Project, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, git_provider::text, repo_url, branch,
			runtime_type::text, environment::text, created_by, created_at, updated_at
		FROM projects WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Name, &p.GitProvider, &p.RepoURL, &p.Branch,
			&p.RuntimeType, &p.Environment, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *Store) GetProjectForUser(ctx context.Context, projectID, userID uuid.UUID) (*Project, error) {
	var project Project
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, name, git_provider::text, repo_url, branch,
			runtime_type::text, environment::text, created_by, created_at, updated_at
		FROM projects WHERE id = $1 AND user_id = $2
	`, projectID, userID).Scan(
		&project.ID, &project.UserID, &project.Name, &project.GitProvider, &project.RepoURL,
		&project.Branch, &project.RuntimeType, &project.Environment, &project.CreatedBy,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

type BuildJob struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     uuid.UUID  `json:"project_id"`
	Status        string     `json:"status"`
	RetryCount    int        `json:"retry_count"`
	MaxRetries    int        `json:"max_retries"`
	FailureReason *string    `json:"failure_reason,omitempty"`
	CancelledAt   *time.Time `json:"cancelled_at,omitempty"`
	QueuedAt      time.Time  `json:"queued_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	BuildingAt    *time.Time `json:"building_at,omitempty"`
	ScanningAt    *time.Time `json:"scanning_at,omitempty"`
	DeployingAt   *time.Time `json:"deploying_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	CreatedBy     uuid.UUID  `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (s *Store) CreateBuildJob(ctx context.Context, projectID, userID uuid.UUID) (*BuildJob, error) {
	if _, err := s.GetProjectForUser(ctx, projectID, userID); err != nil {
		return nil, err
	}

	var job BuildJob
	err := s.pool.QueryRow(ctx, `
		INSERT INTO build_jobs (project_id, status, created_by)
		VALUES ($1, 'queued', $2)
		RETURNING id, project_id, status::text, retry_count, max_retries, failure_reason,
			cancelled_at, queued_at, started_at, building_at, scanning_at, deploying_at,
			finished_at, created_by, created_at, updated_at
	`, projectID, userID).Scan(
		&job.ID, &job.ProjectID, &job.Status, &job.RetryCount, &job.MaxRetries, &job.FailureReason,
		&job.CancelledAt, &job.QueuedAt, &job.StartedAt, &job.BuildingAt, &job.ScanningAt,
		&job.DeployingAt, &job.FinishedAt, &job.CreatedBy, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) ListBuildJobsForProject(ctx context.Context, projectID, userID uuid.UUID) ([]BuildJob, error) {
	if _, err := s.GetProjectForUser(ctx, projectID, userID); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, status::text, retry_count, max_retries, failure_reason,
			cancelled_at, queued_at, started_at, building_at, scanning_at, deploying_at,
			finished_at, created_by, created_at, updated_at
		FROM build_jobs WHERE project_id = $1 ORDER BY created_at DESC LIMIT 20
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []BuildJob
	for rows.Next() {
		var job BuildJob
		if err := rows.Scan(
			&job.ID, &job.ProjectID, &job.Status, &job.RetryCount, &job.MaxRetries, &job.FailureReason,
			&job.CancelledAt, &job.QueuedAt, &job.StartedAt, &job.BuildingAt, &job.ScanningAt,
			&job.DeployingAt, &job.FinishedAt, &job.CreatedBy, &job.CreatedAt, &job.UpdatedAt,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) GetBuildJobForUser(ctx context.Context, jobID, userID uuid.UUID) (*BuildJob, error) {
	var job BuildJob
	err := s.pool.QueryRow(ctx, `
		SELECT b.id, b.project_id, b.status::text, b.retry_count, b.max_retries, b.failure_reason,
			b.cancelled_at, b.queued_at, b.started_at, b.building_at, b.scanning_at, b.deploying_at,
			b.finished_at, b.created_by, b.created_at, b.updated_at
		FROM build_jobs b
		INNER JOIN projects p ON p.id = b.project_id
		WHERE b.id = $1 AND p.user_id = $2
	`, jobID, userID).Scan(
		&job.ID, &job.ProjectID, &job.Status, &job.RetryCount, &job.MaxRetries, &job.FailureReason,
		&job.CancelledAt, &job.QueuedAt, &job.StartedAt, &job.BuildingAt, &job.ScanningAt,
		&job.DeployingAt, &job.FinishedAt, &job.CreatedBy, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) CancelBuildJob(ctx context.Context, jobID, userID uuid.UUID) (*BuildJob, error) {
	var job BuildJob
	err := s.pool.QueryRow(ctx, `
		UPDATE build_jobs b SET
			cancelled_at = COALESCE(b.cancelled_at, NOW()),
			status = CASE
				WHEN b.status IN ('success', 'failed', 'cancelled') THEN b.status
				ELSE 'cancelled'
			END,
			finished_at = CASE
				WHEN b.status IN ('success', 'failed', 'cancelled') THEN b.finished_at
				ELSE NOW()
			END,
			updated_at = NOW()
		FROM projects p
		WHERE b.project_id = p.id AND b.id = $1 AND p.user_id = $2
		RETURNING b.id, b.project_id, b.status::text, b.retry_count, b.max_retries, b.failure_reason,
			b.cancelled_at, b.queued_at, b.started_at, b.building_at, b.scanning_at, b.deploying_at,
			b.finished_at, b.created_by, b.created_at, b.updated_at
	`, jobID, userID).Scan(
		&job.ID, &job.ProjectID, &job.Status, &job.RetryCount, &job.MaxRetries, &job.FailureReason,
		&job.CancelledAt, &job.QueuedAt, &job.StartedAt, &job.BuildingAt, &job.ScanningAt,
		&job.DeployingAt, &job.FinishedAt, &job.CreatedBy, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

type LogLine struct {
	Seq      int64     `json:"seq"`
	LoggedAt time.Time `json:"logged_at"`
	Level    string    `json:"level"`
	Message  string    `json:"message"`
}

func (s *Store) AppendLogLine(ctx context.Context, jobID uuid.UUID, level, message string) (*LogLine, error) {
	message = logs.Mask(message)
	var line LogLine
	err := s.pool.QueryRow(ctx, `
		INSERT INTO build_log_lines (job_id, seq, level, message)
		VALUES (
			$1,
			COALESCE((SELECT MAX(seq) FROM build_log_lines WHERE job_id = $1), 0) + 1,
			$2,
			$3
		)
		RETURNING seq, logged_at, level, message
	`, jobID, level, message).Scan(&line.Seq, &line.LoggedAt, &line.Level, &line.Message)
	if err != nil {
		return nil, err
	}
	return &line, nil
}

func (s *Store) ListLogLines(ctx context.Context, jobID uuid.UUID, afterSeq int64, limit int) ([]LogLine, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT seq, logged_at, level, message
		FROM build_log_lines
		WHERE job_id = $1 AND seq > $2
		ORDER BY seq ASC
		LIMIT $3
	`, jobID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []LogLine
	for rows.Next() {
		var line LogLine
		if err := rows.Scan(&line.Seq, &line.LoggedAt, &line.Level, &line.Message); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func (s *Store) ClaimNextJob(ctx context.Context) (*BuildJob, error) {
	var job BuildJob
	err := s.pool.QueryRow(ctx, `
		UPDATE build_jobs
		SET status = 'building',
			started_at = COALESCE(started_at, NOW()),
			building_at = NOW(),
			updated_at = NOW()
		WHERE id = (
			SELECT id FROM build_jobs
			WHERE status = 'queued' AND cancelled_at IS NULL
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, project_id, status::text, retry_count, max_retries, failure_reason,
			cancelled_at, queued_at, started_at, building_at, scanning_at, deploying_at,
			finished_at, created_by, created_at, updated_at
	`).Scan(
		&job.ID, &job.ProjectID, &job.Status, &job.RetryCount, &job.MaxRetries, &job.FailureReason,
		&job.CancelledAt, &job.QueuedAt, &job.StartedAt, &job.BuildingAt, &job.ScanningAt,
		&job.DeployingAt, &job.FinishedAt, &job.CreatedBy, &job.CreatedAt, &job.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) GetBuildJobByID(ctx context.Context, jobID uuid.UUID) (*BuildJob, error) {
	var job BuildJob
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, status::text, retry_count, max_retries, failure_reason,
			cancelled_at, queued_at, started_at, building_at, scanning_at, deploying_at,
			finished_at, created_by, created_at, updated_at
		FROM build_jobs WHERE id = $1
	`, jobID).Scan(
		&job.ID, &job.ProjectID, &job.Status, &job.RetryCount, &job.MaxRetries, &job.FailureReason,
		&job.CancelledAt, &job.QueuedAt, &job.StartedAt, &job.BuildingAt, &job.ScanningAt,
		&job.DeployingAt, &job.FinishedAt, &job.CreatedBy, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	switch status {
	case "scanning":
		_, err := s.pool.Exec(ctx, `
			UPDATE build_jobs SET status = 'scanning'::build_status, scanning_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, jobID)
		return err
	case "deploying":
		_, err := s.pool.Exec(ctx, `
			UPDATE build_jobs SET status = 'deploying'::build_status, deploying_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, jobID)
		return err
	case "success", "failed", "cancelled":
		_, err := s.pool.Exec(ctx, `
			UPDATE build_jobs SET status = $2::build_status, finished_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, jobID, status)
		return err
	case "queued":
		_, err := s.pool.Exec(ctx, `
			UPDATE build_jobs SET status = 'queued'::build_status, updated_at = NOW()
			WHERE id = $1
		`, jobID)
		return err
	default:
		_, err := s.pool.Exec(ctx, `
			UPDATE build_jobs SET status = $2::build_status, updated_at = NOW()
			WHERE id = $1
		`, jobID, status)
		return err
	}
}

func (s *Store) FailJob(ctx context.Context, jobID uuid.UUID, reason string, retry bool) error {
	if retry {
		_, err := s.pool.Exec(ctx, `
			UPDATE build_jobs SET
				status = 'queued',
				retry_count = retry_count + 1,
				failure_reason = $2,
				building_at = NULL,
				scanning_at = NULL,
				deploying_at = NULL,
				updated_at = NOW()
			WHERE id = $1
		`, jobID, reason)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE build_jobs SET
			status = 'failed',
			failure_reason = $2,
			finished_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, jobID, reason)
	return err
}

func (s *Store) IsJobCancelled(ctx context.Context, jobID uuid.UUID) (bool, error) {
	var cancelledAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT cancelled_at FROM build_jobs WHERE id = $1
	`, jobID).Scan(&cancelledAt)
	if err != nil {
		return false, err
	}
	return cancelledAt != nil, nil
}

func (s *Store) GetProjectByID(ctx context.Context, projectID uuid.UUID) (*Project, error) {
	var project Project
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, name, git_provider::text, repo_url, branch,
			runtime_type::text, environment::text, created_by, created_at, updated_at
		FROM projects WHERE id = $1
	`, projectID).Scan(
		&project.ID, &project.UserID, &project.Name, &project.GitProvider, &project.RepoURL,
		&project.Branch, &project.RuntimeType, &project.Environment, &project.CreatedBy,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &project, nil
}
