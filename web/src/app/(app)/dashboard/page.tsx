"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { Run } from "@/lib/types";
import { StatusBadge, formatTime, formatDuration } from "@/lib/run-utils";

export default function DashboardPage() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("/api/runs")
      .then((r) => r.json())
      .then((data) => setRuns(Array.isArray(data) ? data : []))
      .catch(() => setRuns([]))
      .finally(() => setLoading(false));
  }, []);

  const stats = {
    total: runs.length,
    running: runs.filter((r) => r.status === "running").length,
    completed: runs.filter((r) => r.status === "completed").length,
    failed: runs.filter((r) => r.status === "failed").length,
  };

  const recent = runs.slice(0, 5);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Overview of your agent runs
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Total Runs" value={stats.total} loading={loading} />
        <StatCard title="Running" value={stats.running} loading={loading} accent="text-blue-400" />
        <StatCard title="Completed" value={stats.completed} loading={loading} accent="text-emerald-400" />
        <StatCard title="Failed" value={stats.failed} loading={loading} accent="text-red-400" />
      </div>

      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-medium">Recent Runs</h2>
          <Link href="/runs" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
            View all →
          </Link>
        </div>

        {loading ? (
          <div className="text-sm text-muted-foreground">Loading...</div>
        ) : recent.length === 0 ? (
          <Card>
            <CardContent className="py-8 text-center text-sm text-muted-foreground">
              No runs yet.{" "}
              <Link href="/runs/new" className="text-primary hover:underline">
                Create one
              </Link>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-2">
            {recent.map((run) => (
              <Link key={run.id} href={`/runs/${run.id}`}>
                <Card className="hover:bg-accent/50 transition-colors cursor-pointer">
                  <CardContent className="flex items-center justify-between py-3 px-4">
                    <div className="flex items-center gap-4">
                      <StatusBadge status={run.status} />
                      <div>
                        <p className="text-sm font-medium">{run.name}</p>
                        <p className="text-xs text-muted-foreground font-mono">{run.id.slice(0, 8)}</p>
                      </div>
                    </div>
                    <div className="text-right text-xs text-muted-foreground">
                      <p>{formatTime(run.created_at)}</p>
                      {run.completed_at && run.started_at && (
                        <p>{formatDuration(run.started_at, run.completed_at)}</p>
                      )}
                    </div>
                  </CardContent>
                </Card>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function StatCard({
  title,
  value,
  loading,
  accent,
}: {
  title: string;
  value: number;
  loading: boolean;
  accent?: string;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-mt-muted-foreground">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <p className={`text-3xl font-bold tabular-nums ${accent || ""}`}>
          {loading ? "–" : value}
        </p>
      </CardContent>
    </Card>
  );
}
