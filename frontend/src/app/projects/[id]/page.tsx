"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ApiError, BuildJob, Project, api } from "@/lib/api";
import { AppShell, Button, Card, StatusBadge } from "@/components/ui";

export default function ProjectDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [builds, setBuilds] = useState<BuildJob[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [triggering, setTriggering] = useState(false);

  const load = async () => {
    try {
      const [projectRes, buildsRes] = await Promise.all([
        api.getProject(params.id),
        api.listBuilds(params.id),
      ]);
      setProject(projectRes);
      setBuilds(buildsRes.builds);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        router.replace("/login");
        return;
      }
      setError(err instanceof Error ? err.message : "Failed to load project");
    }
  };

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [params.id]);

  const triggerBuild = async () => {
    setTriggering(true);
    setError(null);
    try {
      const job = await api.triggerBuild(params.id);
      router.push(`/builds/${job.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to trigger build");
    } finally {
      setTriggering(false);
    }
  };

  if (!project && !error) {
    return <div className="flex min-h-screen items-center justify-center text-slate-600">Loading project...</div>;
  }

  return (
    <AppShell title={project?.name || "Project"}>
      {project && (
        <Card className="mb-6">
          <dl className="grid gap-3 text-sm sm:grid-cols-2">
            <Item label="Repository" value={project.repo_url} />
            <Item label="Branch" value={project.branch} />
            <Item label="Git provider" value={project.git_provider} />
            <Item label="Runtime" value={project.runtime_type} />
            <Item label="Environment" value={project.environment} />
          </dl>
          <div className="mt-6">
            <Button onClick={triggerBuild} disabled={triggering}>
              {triggering ? "Starting build..." : "Build / Deploy"}
            </Button>
          </div>
        </Card>
      )}

      {error && <p className="mb-4 text-sm text-red-600">{error}</p>}

      <Card>
        <h2 className="mb-4 text-lg font-medium">Recent Builds</h2>
        {builds.length === 0 ? (
          <p className="text-sm text-slate-500">No builds yet.</p>
        ) : (
          <div className="space-y-3">
            {builds.map((build) => (
              <Link
                key={build.id}
                href={`/builds/${build.id}`}
                className="flex items-center justify-between rounded-lg border border-slate-200 px-4 py-3 hover:bg-slate-50"
              >
                <span className="font-mono text-sm">{build.id.slice(0, 8)}</span>
                <StatusBadge status={build.status} />
              </Link>
            ))}
          </div>
        )}
      </Card>
    </AppShell>
  );
}

function Item({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-slate-500">{label}</dt>
      <dd className="font-medium text-slate-900">{value}</dd>
    </div>
  );
}
