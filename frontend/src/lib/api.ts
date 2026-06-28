export type User = {
  id: string;
  email: string;
  name: string;
  avatar_url: string;
  created_at: string;
  updated_at: string;
};

export type Project = {
  id: string;
  user_id: string;
  name: string;
  git_provider: string;
  repo_url: string;
  branch: string;
  runtime_type: string;
  environment: string;
  created_by: string;
  created_at: string;
  updated_at: string;
};

export type BuildJob = {
  id: string;
  project_id: string;
  status: string;
  retry_count: number;
  max_retries: number;
  failure_reason?: string | null;
  cancelled_at?: string | null;
  queued_at: string;
  started_at?: string | null;
  building_at?: string | null;
  scanning_at?: string | null;
  deploying_at?: string | null;
  finished_at?: string | null;
  created_by: string;
  created_at: string;
  updated_at: string;
};

export type LogLine = {
  seq: number;
  logged_at: string;
  level: string;
  message: string;
};

export class ApiError extends Error {
  code: string;
  status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

function apiBase() {
  if (typeof window === "undefined") {
    return process.env.API_INTERNAL_URL || process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  }
  return process.env.NEXT_PUBLIC_API_URL || "";
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const base = apiBase();
  const url = base ? `${base}${path}` : path;
  const res = await fetch(url, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers || {}),
    },
    cache: "no-store",
  });

  if (!res.ok) {
    let code = "request_failed";
    let message = res.statusText;
    try {
      const body = await res.json();
      code = body?.error?.code || code;
      message = body?.error?.message || message;
    } catch {
      // ignore parse errors
    }
    throw new ApiError(res.status, code, message);
  }

  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

export const api = {
  me: () => request<User>("/api/v1/me"),
  logout: () => request<void>("/api/v1/auth/logout", { method: "POST" }),
  listProjects: () => request<{ projects: Project[] }>("/api/v1/projects"),
  createProject: (body: {
    name: string;
    git_provider: string;
    repo_url: string;
    branch: string;
    runtime_type: string;
    environment: string;
  }) => request<Project>("/api/v1/projects", { method: "POST", body: JSON.stringify(body) }),
  getProject: (id: string) => request<Project>(`/api/v1/projects/${id}`),
  listBuilds: (projectId: string) => request<{ builds: BuildJob[] }>(`/api/v1/projects/${projectId}/builds`),
  triggerBuild: (projectId: string) =>
    request<BuildJob>(`/api/v1/projects/${projectId}/builds`, { method: "POST" }),
  getBuild: (id: string) => request<BuildJob>(`/api/v1/builds/${id}`),
  getLogs: (id: string, afterSeq = 0) =>
    request<{ logs: LogLine[] }>(`/api/v1/builds/${id}/logs?after_seq=${afterSeq}`),
  cancelBuild: (id: string) => request<BuildJob>(`/api/v1/builds/${id}/cancel`, { method: "POST" }),
  loginUrl: (provider: string) => `/api/v1/auth/${provider}/login`,
};

export const BUILD_STAGES = ["queued", "building", "scanning", "deploying", "success"] as const;

export function statusColor(status: string) {
  switch (status) {
    case "success":
      return "bg-emerald-100 text-emerald-800";
    case "failed":
      return "bg-red-100 text-red-800";
    case "cancelled":
      return "bg-slate-100 text-slate-700";
    case "queued":
      return "bg-amber-100 text-amber-800";
    default:
      return "bg-blue-100 text-blue-800";
  }
}

export function isTerminal(status: string) {
  return status === "success" || status === "failed" || status === "cancelled";
}
