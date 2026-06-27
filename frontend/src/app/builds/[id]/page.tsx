"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ApiError, BuildJob, api } from "@/lib/api";
import { AppShell } from "@/components/ui";
import { BuildDetailClient } from "@/components/BuildDetailClient";

export default function BuildDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const [build, setBuild] = useState<BuildJob | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const job = await api.getBuild(params.id);
        if (active) setBuild(job);
      } catch (err) {
        if (!active) return;
        if (err instanceof ApiError && err.status === 401) {
          router.replace("/login");
          return;
        }
        setError(err instanceof Error ? err.message : "Failed to load build");
      }
    };
    load();
    return () => {
      active = false;
    };
  }, [params.id, router]);

  if (!build && !error) {
    return <div className="flex min-h-screen items-center justify-center text-slate-600">Loading build...</div>;
  }

  return (
    <AppShell title="Build Details">
      <div className="mb-4">
        <Link href={build ? `/projects/${build.project_id}` : "/dashboard"} className="text-sm text-slate-600 hover:text-slate-900">
          ← Back to project
        </Link>
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
      {build && <BuildDetailClient initialBuild={build} />}
    </AppShell>
  );
}
