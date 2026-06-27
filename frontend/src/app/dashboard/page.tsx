"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ApiError, Project, User, api } from "@/lib/api";
import { AppShell, Button, Card, StatusBadge } from "@/components/ui";

export default function DashboardPage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const [me, list] = await Promise.all([api.me(), api.listProjects()]);
        if (!active) return;
        setUser(me);
        setProjects(list.projects);
      } catch (err) {
        if (!active) return;
        if (err instanceof ApiError && err.status === 401) {
          router.replace("/login");
          return;
        }
        setError(err instanceof Error ? err.message : "Failed to load dashboard");
      } finally {
        if (active) setLoading(false);
      }
    };
    load();
    return () => {
      active = false;
    };
  }, [router]);

  const logout = async () => {
    await api.logout();
    router.replace("/login");
  };

  if (loading) {
    return <div className="flex min-h-screen items-center justify-center text-slate-600">Loading dashboard...</div>;
  }

  return (
    <AppShell
      title="Dashboard"
      actions={
        user && (
          <div className="flex items-center gap-3">
            <span className="hidden text-sm text-slate-600 sm:inline">{user.email}</span>
            <Button variant="secondary" onClick={logout}>
              Log out
            </Button>
          </div>
        )
      }
    >
      {error && <p className="mb-4 text-sm text-red-600">{error}</p>}

      <div className="mb-6">
        <Link href="/projects/new">
          <Button>New Project</Button>
        </Link>
      </div>

      {projects.length === 0 ? (
        <Card>
          <p className="text-slate-600">No projects yet. Create your first project to trigger a build.</p>
        </Card>
      ) : (
        <div className="grid gap-4">
          {projects.map((project) => (
            <Card key={project.id}>
              <div className="flex flex-wrap items-center justify-between gap-4">
                <div>
                  <Link href={`/projects/${project.id}`} className="text-lg font-medium text-slate-900 hover:underline">
                    {project.name}
                  </Link>
                  <p className="mt-1 text-sm text-slate-500">
                    {project.git_provider} · {project.runtime_type} · {project.environment}
                  </p>
                </div>
                <StatusBadge status={project.environment} />
              </div>
            </Card>
          ))}
        </div>
      )}
    </AppShell>
  );
}
