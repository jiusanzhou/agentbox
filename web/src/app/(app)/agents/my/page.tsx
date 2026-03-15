"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { clientFetch } from "@/lib/api";
import type { Run } from "@/lib/types";

function formatCost(micros: number): string {
  return `$${(micros / 1_000_000).toFixed(4)}`;
}

function formatTime(ts: string): string {
  return new Date(ts).toLocaleString();
}

const STATUS_COLORS: Record<string, string> = {
  pending: "bg-yellow-100 text-yellow-700",
  running: "bg-blue-100 text-blue-700",
  completed: "bg-green-100 text-green-700",
  failed: "bg-red-100 text-red-700",
};

const SUB_STATUS_COLORS: Record<string, string> = {
  active: "bg-green-100 text-green-700",
  trialing: "bg-blue-100 text-blue-700",
  past_due: "bg-yellow-100 text-yellow-700",
  canceled: "bg-gray-100 text-gray-500",
  expired: "bg-red-100 text-red-700",
};

interface Subscription {
  id: string;
  agent_id: string;
  pricing_model: string;
  status: string;
  current_period_start: string;
  current_period_end: string;
  trial_ends_at?: string;
  created_at: string;
}

interface UsageSummary {
  total_runs: number;
  total_tokens: number;
  total_cost_micros: number;
}

export default function MyAgentsPage() {
  const router = useRouter();
  const [runs, setRuns] = useState<Run[]>([]);
  const [subs, setSubs] = useState<Subscription[]>([]);
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      fetch("/api/runs").then((r) => r.json()).catch(() => []),
      clientFetch("/api/v1/billing/subscriptions").then((r) => r.json()).catch(() => []),
      clientFetch("/api/v1/billing/usage/summary").then((r) => r.json()).catch(() => null),
    ]).then(([runsData, subsData, usageData]) => {
      setRuns(Array.isArray(runsData) ? runsData : []);
      setSubs(Array.isArray(subsData) ? subsData : []);
      setUsage(usageData);
    }).finally(() => setLoading(false));
  }, []);

  const activeRuns = runs.filter((r) => r.status === "running" || r.status === "pending");
  const recentRuns = runs.filter((r) => r.status === "completed" || r.status === "failed").slice(0, 10);
  const activeSubs = subs.filter((s) => s.status === "active" || s.status === "trialing");

  if (loading) {
    return <div className="text-center py-12 text-muted-foreground">Loading...</div>;
  }

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">My Agents</h1>
          <p className="text-muted-foreground mt-1">Your hired agents, active runs, and spend</p>
        </div>
        <Button onClick={() => router.push("/agents")}>Browse Marketplace</Button>
      </div>

      {/* Summary stats */}
      <div className="grid grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Active Runs</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold text-blue-600">{activeRuns.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Total Runs</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold">{usage?.total_runs ?? runs.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Active Subscriptions</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold text-green-600">{activeSubs.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Total Spend</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold">{formatCost(usage?.total_cost_micros ?? 0)}</p>
          </CardContent>
        </Card>
      </div>

      {/* Active runs */}
      {activeRuns.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <span className="h-2 w-2 rounded-full bg-blue-500 animate-pulse" />
              Active Runs
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {activeRuns.map((run) => (
              <div
                key={run.id}
                className="flex items-center justify-between p-3 rounded-lg border border-border hover:bg-accent/30 cursor-pointer transition-colors"
                onClick={() => router.push(`/runs/${run.id}`)}
              >
                <div>
                  <p className="font-medium">{run.name}</p>
                  <p className="text-xs text-muted-foreground">Started {formatTime(run.started_at || run.created_at)}</p>
                </div>
                <div className="flex items-center gap-2">
                  <Badge className={STATUS_COLORS[run.status] || ""}>{run.status}</Badge>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation();
                      router.push(`/chat?session=${run.id}`);
                    }}
                  >
                    Open Chat
                  </Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Subscriptions */}
      {subs.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Subscriptions</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {subs.map((sub) => (
              <div
                key={sub.id}
                className="flex items-center justify-between p-3 rounded-lg border border-border"
              >
                <div>
                  <p className="font-medium font-mono text-sm">{sub.agent_id}</p>
                  <p className="text-xs text-muted-foreground">
                    {sub.pricing_model} · Since {new Date(sub.created_at).toLocaleDateString()}
                    {sub.trial_ends_at && ` · Trial ends ${new Date(sub.trial_ends_at).toLocaleDateString()}`}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <Badge className={SUB_STATUS_COLORS[sub.status] || ""}>{sub.status}</Badge>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => router.push(`/agents/${sub.agent_id}`)}
                  >
                    View Agent
                  </Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Recent run history */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Recent Runs</CardTitle>
            <Link href="/runs" className="text-sm text-primary hover:underline">
              View all runs →
            </Link>
          </div>
        </CardHeader>
        <CardContent>
          {recentRuns.length === 0 && activeRuns.length === 0 ? (
            <div className="text-center py-8 space-y-3">
              <p className="text-muted-foreground">No runs yet. Hire an agent to get started!</p>
              <Button onClick={() => router.push("/agents")}>Browse Agents</Button>
            </div>
          ) : recentRuns.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">No completed runs yet.</p>
          ) : (
            <div className="space-y-2">
              {recentRuns.map((run) => (
                <div
                  key={run.id}
                  className="flex items-center justify-between p-3 rounded-lg border border-border hover:bg-accent/30 cursor-pointer transition-colors"
                  onClick={() => router.push(`/runs/${run.id}`)}
                >
                  <div>
                    <p className="font-medium text-sm">{run.name}</p>
                    <p className="text-xs text-muted-foreground font-mono">{run.id.slice(0, 8)}</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <p className="text-xs text-muted-foreground">{formatTime(run.created_at)}</p>
                    <Badge className={`text-xs ${STATUS_COLORS[run.status] || ""}`}>{run.status}</Badge>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Billing link */}
      {usage && usage.total_cost_micros > 0 && (
        <div className="text-center">
          <Link href="/billing" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
            View detailed billing & usage →
          </Link>
        </div>
      )}
    </div>
  );
}
