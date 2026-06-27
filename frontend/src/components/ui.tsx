import Link from "next/link";
import { ReactNode } from "react";

export function AppShell({
  title,
  children,
  actions,
}: {
  title: string;
  children: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="min-h-screen bg-slate-50 text-slate-900">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-6">
            <Link href="/dashboard" className="text-lg font-semibold text-slate-900">
              Dev Platform
            </Link>
            <nav className="hidden gap-4 text-sm text-slate-600 sm:flex">
              <Link href="/dashboard" className="hover:text-slate-900">
                Dashboard
              </Link>
              <Link href="/projects/new" className="hover:text-slate-900">
                New Project
              </Link>
            </nav>
          </div>
          {actions}
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-6 py-8">
        <div className="mb-6 flex items-center justify-between gap-4">
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        </div>
        {children}
      </main>
    </div>
  );
}

export function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    success: "bg-emerald-100 text-emerald-800",
    failed: "bg-red-100 text-red-800",
    cancelled: "bg-slate-100 text-slate-700",
    queued: "bg-amber-100 text-amber-800",
    building: "bg-blue-100 text-blue-800",
    scanning: "bg-indigo-100 text-indigo-800",
    deploying: "bg-violet-100 text-violet-800",
  };
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${colors[status] || "bg-slate-100 text-slate-700"}`}>
      {status}
    </span>
  );
}

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`rounded-xl border border-slate-200 bg-white p-6 shadow-sm ${className}`}>{children}</div>;
}

export function Button({
  children,
  variant = "primary",
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" | "danger" }) {
  const styles = {
    primary: "bg-slate-900 text-white hover:bg-slate-800",
    secondary: "border border-slate-300 bg-white text-slate-800 hover:bg-slate-50",
    danger: "bg-red-600 text-white hover:bg-red-700",
  };
  return (
    <button
      className={`inline-flex items-center justify-center rounded-lg px-4 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-50 ${styles[variant]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
}

export function BuildTimeline({ status }: { status: string }) {
  const stages = ["queued", "building", "scanning", "deploying", "success"];
  const currentIndex = stages.indexOf(status === "failed" || status === "cancelled" ? "deploying" : status);

  return (
    <ol className="flex flex-wrap gap-3">
      {stages.map((stage, index) => {
        const active = index <= currentIndex || status === "success";
        const failed = (status === "failed" || status === "cancelled") && index === currentIndex + 1;
        return (
          <li
            key={stage}
            className={`rounded-full px-3 py-1 text-xs font-medium ${
              failed
                ? "bg-red-100 text-red-800"
                : active
                  ? "bg-slate-900 text-white"
                  : "bg-slate-100 text-slate-500"
            }`}
          >
            {stage}
          </li>
        );
      })}
    </ol>
  );
}
