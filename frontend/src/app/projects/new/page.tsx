"use client";

import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { ApiError, api } from "@/lib/api";
import { AppShell, Button, Card } from "@/components/ui";

export default function NewProjectPage() {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const onSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    const form = new FormData(event.currentTarget);
    try {
      const project = await api.createProject({
        name: String(form.get("name") || ""),
        git_provider: String(form.get("git_provider") || "github"),
        repo_url: String(form.get("repo_url") || ""),
        branch: String(form.get("branch") || "main"),
        runtime_type: String(form.get("runtime_type") || "go"),
        environment: String(form.get("environment") || "dev"),
      });
      router.push(`/projects/${project.id}`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        router.replace("/login");
        return;
      }
      setError(err instanceof Error ? err.message : "Failed to create project");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AppShell title="Create Project">
      <Card className="max-w-2xl">
        <form className="space-y-4" onSubmit={onSubmit}>
          <Field label="Project name" name="name" required placeholder="My App" />
          <div className="grid gap-4 sm:grid-cols-2">
            <Select label="Git provider" name="git_provider" options={["github", "gitlab"]} />
            <Select label="Runtime" name="runtime_type" options={["go", "node", "python", "static"]} />
          </div>
          <Field label="Repository URL" name="repo_url" required placeholder="https://github.com/org/repo" />
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Branch" name="branch" defaultValue="main" />
            <Select label="Environment" name="environment" options={["dev", "staging", "prod"]} />
          </div>
          {error && <p className="text-sm text-red-600">{error}</p>}
          <Button type="submit" disabled={submitting}>
            {submitting ? "Creating..." : "Create Project"}
          </Button>
        </form>
      </Card>
    </AppShell>
  );
}

function Field(props: React.InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  const { label, ...inputProps } = props;
  return (
    <label className="block space-y-1 text-sm">
      <span className="font-medium text-slate-700">{label}</span>
      <input
        className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none ring-slate-900 focus:ring-2"
        {...inputProps}
      />
    </label>
  );
}

function Select({
  label,
  name,
  options,
}: {
  label: string;
  name: string;
  options: string[];
}) {
  return (
    <label className="block space-y-1 text-sm">
      <span className="font-medium text-slate-700">{label}</span>
      <select name={name} className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none ring-slate-900 focus:ring-2">
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </label>
  );
}
