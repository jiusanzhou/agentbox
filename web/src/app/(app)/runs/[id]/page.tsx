"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import type { Run } from "@/lib/types";
import { StatusBadge, formatTime, formatDuration } from "@/lib/run-utils";

export default function RunDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [run, setRun] = useState<Run | null>(null);
  const [loading, setLoading] = useState(true);
  const [cancelling, setCancelling] = useState(false);

  useEffect(() => {
    fetch(`/api/runs/${id}`)
      .then((r) => r.json())
      .then(setRun)
      .catch(() => setRun(null))
      .finally(() => setLoading(false));
  }, [id]);

  const handleCancel = async () => {
    setCancelling(true);
    try {
      await fetch(`/api/runs/${id}`, { method: "DELETE" });
      const res = await fetch(`/api/runs/${id}`);
      setRun(await res.json());
    } catch {
      // ignore
    } finally {
      setCancelling(false);
    }
  };

  if (loading) {
    return <div className="text-sm text-muted-foreground py-8">Loading...</div>;
  }

  if (!run) {
    return <div className="text-sm text-muted-foreground py-8">Run not found.</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-semibold tracking-tight">{run.name}</h1>
            <StatusBadge status={run.status} />
          </div>
          <p className="font-mono text-xs text-muted-foreground">{run.id}</p>
        </div>
        <div className="flex items-center gap-2">
          {run.status === "running" && (
            <Button
              variant="destructive"
              onClick={handleCancel}
              disabled={cancelling}
            >
              {cancelling ? "Cancelling..." : "Cancel Run"}
            </Button>
          )}
          <Button variant="outline" onClick={() => router.push("/runs")}>
            Back
          </Button>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <InfoCard label="Status" value={run.status} />
        <InfoCard label="Created" value={formatTime(run.created_at)} />
        <InfoCard label="Started" value={run.started_at ? formatTime(run.started_at) : "–"} />
        <InfoCard
          label="Duration"
          value={
            run.completed_at && run.started_at
              ? formatDuration(run.started_at, run.completed_at)
              : run.status === "running"
              ? "In progress..."
              : "–"
          }
        />
      </div>

      <Separator />

      {run.agent_file && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Agent File</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto rounded-md bg-muted p-4 text-sm font-mono whitespace-pre-wrap">
              {run.agent_file}
            </pre>
          </CardContent>
        </Card>
      )}

      {run.output && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Output</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto rounded-md bg-muted p-4 text-sm font-mono whitespace-pre-wrap text-emerald-400">
              {run.output}
            </pre>
          </CardContent>
        </Card>
      )}

      {run.error && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Error</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto rounded-md bg-muted p-4 text-sm font-mono whitespace-pre-wrap text-red-400">
              {run.error}
            </pre>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function InfoCard({ label, value }: { label: string; value: string }) {
  return (
    <Card>
      <CardContent className="pt-4">
        <p className="text-xs text-muted-foreground mb-1">{label}</p>
        <p className="text-sm font-medium">{value}</p>
      </CardContent>
    </Card>
  );
}
