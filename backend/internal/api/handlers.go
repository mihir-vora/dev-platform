package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/dev-platform/backend/internal/auth"
	"github.com/dev-platform/backend/internal/config"
	"github.com/dev-platform/backend/internal/db"
	"github.com/dev-platform/backend/internal/validate"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	cfg   *config.Config
	store *db.Store
	pool  *pgxpool.Pool
}

func NewHandler(cfg *config.Config, store *db.Store) *Handler {
	return &Handler{cfg: cfg, store: store, pool: store.Pool()}
}

func (h *Handler) RegisterRoutes(r chi.Router, authService *auth.Service) {
	r.Route("/api/v1", func(r chi.Router) {
		authService.RegisterRoutes(r)

		r.Group(func(r chi.Router) {
			r.Use(authService.Middleware)
			r.Get("/me", h.getMe)
			r.Post("/projects", h.createProject)
			r.Get("/projects", h.listProjects)
			r.Get("/projects/{id}", h.getProject)
			r.Get("/projects/{id}/builds", h.listProjectBuilds)
			r.Post("/projects/{id}/builds", h.triggerBuild)
			r.Get("/builds/{id}", h.getBuild)
			r.Get("/builds/{id}/logs", h.getBuildLogs)
			r.Get("/builds/{id}/logs/stream", h.streamBuildLogs)
			r.Post("/builds/{id}/cancel", h.cancelBuild)
		})
	})
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	writeJSON(w, http.StatusOK, user)
}

type createProjectRequest struct {
	Name        string `json:"name"`
	GitProvider string `json:"git_provider"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	RuntimeType string `json:"runtime_type"`
	Environment string `json:"environment"`
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	for _, check := range []struct {
		value   string
		allowed []string
		field   string
	}{
		{req.GitProvider, []string{"github", "gitlab"}, "git_provider"},
		{req.RuntimeType, []string{"go", "node", "python", "static"}, "runtime_type"},
		{req.Environment, []string{"dev", "staging", "prod"}, "environment"},
	} {
		if err := validate.OneOf(check.value, check.allowed...); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", check.field+" is invalid")
			return
		}
	}
	if err := validate.RepoURL(req.RepoURL, req.GitProvider, h.cfg.GitLabBaseURL); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	project, err := h.store.CreateProject(r.Context(), db.CreateProjectInput{
		Name:        req.Name,
		GitProvider: req.GitProvider,
		RepoURL:     req.RepoURL,
		Branch:      req.Branch,
		RuntimeType: req.RuntimeType,
		Environment: req.Environment,
		UserID:      user.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create project")
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	projects, err := h.store.ListProjects(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list projects")
		return
	}
	if projects == nil {
		projects = []db.Project{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (h *Handler) getProject(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid project id")
		return
	}
	project, err := h.store.GetProjectForUser(r.Context(), projectID, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get project")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) listProjectBuilds(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid project id")
		return
	}
	jobs, err := h.store.ListBuildJobsForProject(r.Context(), projectID, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list builds")
		return
	}
	if jobs == nil {
		jobs = []db.BuildJob{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"builds": jobs})
}

func (h *Handler) triggerBuild(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid project id")
		return
	}
	job, err := h.store.CreateBuildJob(r.Context(), projectID, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create build job")
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (h *Handler) getBuild(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid build id")
		return
	}
	job, err := h.store.GetBuildJobForUser(r.Context(), jobID, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "build not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get build")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) getBuildLogs(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid build id")
		return
	}
	if _, err := h.store.GetBuildJobForUser(r.Context(), jobID, user.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "build not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to authorize build")
		return
	}

	afterSeq := int64(0)
	if v := r.URL.Query().Get("after_seq"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			afterSeq = parsed
		}
	}
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}

	lines, err := h.store.ListLogLines(r.Context(), jobID, afterSeq, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get logs")
		return
	}
	if lines == nil {
		lines = []db.LogLine{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": lines})
}

func (h *Handler) streamBuildLogs(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid build id")
		return
	}
	job, err := h.store.GetBuildJobForUser(r.Context(), jobID, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "build not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to authorize build")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	afterSeq := int64(0)
	if v := r.URL.Query().Get("after_seq"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			afterSeq = parsed
		}
	}

	ctx := r.Context()
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to subscribe to logs")
		return
	}
	defer conn.Release()

	channel := "build_log_" + jobID.String()
	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{channel}.Sanitize()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to listen for logs")
		return
	}

	notifyCh := make(chan struct{}, 1)
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				time.Sleep(500 * time.Millisecond)
				continue
			}
			select {
			case notifyCh <- struct{}{}:
			default:
			}
		}
	}()

	sendLines := func() error {
		lines, err := h.store.ListLogLines(ctx, jobID, afterSeq, 100)
		if err != nil {
			return err
		}
		for _, line := range lines {
			payload, _ := json.Marshal(line)
			if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
				return err
			}
			flusher.Flush()
			afterSeq = line.Seq
		}
		return nil
	}

	if err := sendLines(); err != nil {
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notifyCh:
			if err := sendLines(); err != nil {
				return
			}
		case <-ticker.C:
			current, err := h.store.GetBuildJobByID(ctx, jobID)
			if err == nil {
				job = current
			}
			if err := sendLines(); err != nil {
				return
			}
			if isTerminal(job.Status) {
				return
			}
		}
	}
}

func (h *Handler) cancelBuild(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid build id")
		return
	}
	job, err := h.store.CancelBuildJob(r.Context(), jobID, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "build not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to cancel build")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func isTerminal(status string) bool {
	switch status {
	case "success", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
