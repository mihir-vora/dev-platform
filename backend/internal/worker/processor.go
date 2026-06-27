package worker

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/dev-platform/backend/internal/config"
	"github.com/dev-platform/backend/internal/db"
	"github.com/google/uuid"
)

type Processor struct {
	cfg   *config.Config
	store *db.Store
	sem   chan struct{}
}

func NewProcessor(cfg *config.Config, store *db.Store) *Processor {
	concurrency := cfg.WorkerConcurrency
	if concurrency < 1 {
		concurrency = 1
	}
	return &Processor{
		cfg:   cfg,
		store: store,
		sem:   make(chan struct{}, concurrency),
	}
}

func (p *Processor) Run(ctx context.Context) {
	slog.Info("worker started", "concurrency", p.cfg.WorkerConcurrency)
	ticker := time.NewTicker(p.cfg.WorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopping")
			return
		default:
			p.tryProcess(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Processor) tryProcess(ctx context.Context) {
	select {
	case p.sem <- struct{}{}:
	default:
		return
	}

	go func() {
		defer func() { <-p.sem }()
		if err := p.processOne(ctx); err != nil {
			slog.Error("process job failed", "error", err)
		}
	}()
}

func (p *Processor) processOne(ctx context.Context) error {
	job, err := p.store.ClaimNextJob(ctx)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}
	return p.runJob(ctx, job)
}

func (p *Processor) runJob(ctx context.Context, job *db.BuildJob) error {
	project, err := p.store.GetProjectByID(ctx, job.ProjectID)
	if err != nil {
		return err
	}

	stages := []struct {
		status  string
		lines   []string
		sleep   time.Duration
	}{
		{
			status: "building",
			lines: []string{
				fmt.Sprintf("Cloning %s (branch %s)", project.RepoURL, project.Branch),
				fmt.Sprintf("Detected runtime: %s", project.RuntimeType),
				"Installing dependencies...",
				fmt.Sprintf("Build token=ghp_simulated_secret_%s", uuid.NewString()[:8]),
				"Compiling application...",
			},
			sleep: randomDuration(2, 4),
		},
		{
			status: "scanning",
			lines: []string{
				"Running security scan...",
				"Checking dependency vulnerabilities...",
				"Scan complete: no critical issues",
			},
			sleep: randomDuration(2, 3),
		},
		{
			status: "deploying",
			lines: []string{
				fmt.Sprintf("Deploying to %s environment", project.Environment),
				"Applying configuration...",
				"Health check passed",
			},
			sleep: randomDuration(2, 4),
		},
	}

	for i, stage := range stages {
		if i > 0 {
			if err := p.store.UpdateJobStatus(ctx, job.ID, stage.status); err != nil {
				return err
			}
		}
		if cancelled, err := p.store.IsJobCancelled(ctx, job.ID); err != nil {
			return err
		} else if cancelled {
			_, _ = p.store.AppendLogLine(ctx, job.ID, "warn", "Build cancelled by user")
			return p.store.UpdateJobStatus(ctx, job.ID, "cancelled")
		}

		// Simulated transient failure on first building stage attempt.
		if stage.status == "building" && job.RetryCount == 0 && rand.Intn(10) == 0 {
			reason := "simulated compile failure"
			_, _ = p.store.AppendLogLine(ctx, job.ID, "error", reason)
			current, err := p.store.GetBuildJobByID(ctx, job.ID)
			if err != nil {
				return err
			}
			if current.RetryCount+1 < current.MaxRetries {
				return p.store.FailJob(ctx, job.ID, reason, true)
			}
			return p.store.FailJob(ctx, job.ID, reason, false)
		}

		for _, line := range stage.lines {
			if cancelled, err := p.store.IsJobCancelled(ctx, job.ID); err != nil {
				return err
			} else if cancelled {
				_, _ = p.store.AppendLogLine(ctx, job.ID, "warn", "Build cancelled by user")
				return p.store.UpdateJobStatus(ctx, job.ID, "cancelled")
			}
			if _, err := p.store.AppendLogLine(ctx, job.ID, "info", line); err != nil {
				return err
			}
		}
		time.Sleep(stage.sleep)

		if cancelled, err := p.store.IsJobCancelled(ctx, job.ID); err != nil {
			return err
		} else if cancelled {
			_, _ = p.store.AppendLogLine(ctx, job.ID, "warn", "Build cancelled by user")
			return p.store.UpdateJobStatus(ctx, job.ID, "cancelled")
		}
	}

	_, _ = p.store.AppendLogLine(ctx, job.ID, "info", "Deployment successful")
	return p.store.UpdateJobStatus(ctx, job.ID, "success")
}

func randomDuration(minSec, maxSec int) time.Duration {
	if maxSec <= minSec {
		return time.Duration(minSec) * time.Second
	}
	seconds := minSec + rand.Intn(maxSec-minSec+1)
	return time.Duration(seconds) * time.Second
}
