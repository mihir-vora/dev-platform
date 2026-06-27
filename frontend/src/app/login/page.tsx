import Link from "next/link";
import { api } from "@/lib/api";

export default function LoginPage() {
  const providers = [
    { id: "github", label: "Continue with GitHub" },
    { id: "google", label: "Continue with Google" },
    { id: "gitlab", label: "Continue with GitLab" },
  ];

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950 px-6">
      <div className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-8 shadow-xl">
        <div className="mb-8 space-y-2 text-center">
          <h1 className="text-2xl font-semibold text-white">Dev Platform</h1>
          <p className="text-sm text-slate-400">Sign in to manage projects and deployments.</p>
        </div>
        <div className="space-y-3">
          {providers.map((provider) => (
            <Link
              key={provider.id}
              href={api.loginUrl(provider.id)}
              className="flex w-full items-center justify-center rounded-lg border border-slate-700 bg-slate-800 px-4 py-3 text-sm font-medium text-white transition hover:bg-slate-700"
            >
              {provider.label}
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
