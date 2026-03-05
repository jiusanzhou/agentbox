"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { Run } from "@/lib/types";
import { StatusBadge, formatTime, formatDuration } from "@/lib/run-utils";

export default function RunsPage() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("/api/runs")
      .then((r) => r.json())
      .then((data) => setRuns(Array.isArray(data) ? data : []))
      .catch(() => setRuns([]))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Runs</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Manage your agent workflow runs
          </p>
        </div>
        <Link href="/runs/new">
          <Button>New Run</Button>
        </Link>
      </div>

      {loading ? (
        <div className="text-sm text-muted-foreground py-8 text-center">Loading...</div>
      ) : runs.length === 0 ? (
        <div className="text-sm text-muted-foreground py-8 text-center">
          No runs found.{" "}
          <Link href="/runs/new" className="text-primary hover:underline">
            Create one
          </Link>
        </div>
      ) : (
        <div className="rounded-md border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[100px]">ID</TableHead>
                <TableHead>Name</TableHead>
                <TableHead className="w-[120px]">Status</TableHead>
                <TableHead className="w-[180px]">Created</TableHead>
                <TableHead className="w-[100px]">Duration</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {runs.map((run) => (
                <TableRow key={run.id} className="cursor-pointer hover:bg-accent/50">
                  <TableCell>
                    <Link href={`/runs/${run.id}`} className="font-mono text-xs hover:underline">
                      {run.id.slice(0, 8)}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Link href={`/runs/${run.id}`} className="hover:underline">
                      {run.name}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={run.status} />
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {formatTime(run.created_at)}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {run.completed_at && run.started_at
                      ? formatDuration(run.started_at, run.completed_at)
                      : "–"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
