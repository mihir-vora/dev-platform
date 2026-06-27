"use client";

import { useEffect, useRef, useState } from "react";
import { BuildJob, LogLine, api, isTerminal } from "@/lib/api";
import { BuildTimeline, Button, Card, StatusBadge } from "@/components/ui";

export function BuildDetailClient({ initialBuild }: { initialBuild: BuildJob }) {
  const [build, setBuild] = useState(initialBuild);
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const logRef = useRef<HTMLDivElement>(null);
  const afterSeqRef = useRef(0);

  useEffect(() => {
    let active = true;
    const loadInitial = async () => {
      try {
        const res = await api.getLogs(build.id);
        if (!active) return;
        setLogs(res.logs);
        afterSeqRef.current = res.logs.at(-1)?.seq || 0;
      } catch (err) {
        if (active) setError(err instanceof Error ? err.message : "Failed to load logs");
      }
    };
    loadInitial();
    return () => {
      active = false;
    };
  }, [build.id]);

  useEffect(() => {
    const base = process.env.NEXT_PUBLIC_API_URL || "";
    const url = `${base}/api/v1/builds/${build.id}/logs/stream?after_seq=${afterSeqRef.current}`;
    const source = new EventSource(url, { withCredentials: true });

    source.onmessage = (event) => {
      try {
        const line = JSON.parse(event.data) as LogLine;
        setLogs((prev) => {
          if (prev.some((item) => item.seq === line.seq)) return prev;
          return [...prev, line];
        });
        afterSeqRef.current = line.seq;
      } catch {
        // ignore malformed events
      }
    };

    source.onerror = () => {
      source.close();
    };

    return () => source.close();
  }, [build.id]);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs]);

  useEffect(() => {
    if (isTerminal(build.status)) return;
    const timer = setInterval(async () => {
      try {
        const next = await api.getBuild(build.id);
        setBuild(next);
      } catch {
        // ignore polling errors
      }
    }, 2000);
    return () => clearInterval(timer);
  }, [build.id, build.status]);

  const handleCancel = async () => {
    setCancelling(true);
    setError(null);
    try {
      const next = await api.cancelBuild(build.id);
      setBuild(next);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel build");
    } finally {
      setCancelling(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="space-y-2">
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-medium">Build {build.id.slice(0, 8)}</h2>
              <StatusBadge status={build.status} />
            </div>
            <BuildTimeline status={build.status} />
            {build.failure_reason && (
              <p className="text-sm text-red-700">Failure: {build.failure_reason}</p>
            )}
            <p className="text-sm text-slate-500">
              Retries: {build.retry_count}/{build.max_retries}
            </p>
          </div>
          {!isTerminal(build.status) && (
            <Button variant="danger" onClick={handleCancel} disabled={cancelling}>
              {cancelling ? "Cancelling..." : "Cancel Build"}
            </Button>
          )}
        </div>
      </Card>

      {error && <p className="text-sm text-red-600">{error}</p>}

      <Card className="p-0">
        <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium">Build Logs</div>
        <div ref={logRef} className="max-h-[32rem] overflow-auto bg-slate-950 p-4 font-mono text-xs text-slate-100">
          {logs.length === 0 ? (
            <p className="text-slate-400">Waiting for logs...</p>
          ) : (
            logs.map((line) => (
              <div key={line.seq} className="whitespace-pre-wrap py-0.5">
                <span className="text-slate-500">{new Date(line.logged_at).toLocaleTimeString()} </span>
                <span className="text-slate-400">[{line.level}] </span>
                <span>{line.message}</span>
              </div>
            ))
          )}
        </div>
      </Card>
    </div>
  );
}
