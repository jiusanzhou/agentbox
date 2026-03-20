"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth";
import { clientFetch } from "@/lib/api";

// ─── API response interfaces ───────────────────────────────────────────────

interface UsageSummary {
  user_id: string;
  period: string;
  compute_seconds: number;
  token_count: number;
  storage_bytes: number;
  api_calls: number;
  estimated_cost: number;
}

interface UsageRecord {
  id: string;
  user_id: string;
  type: "compute" | "tokens" | "storage" | "api";
  amount: number;
  unit: string;
  run_id?: string;
  session_id?: string;
  description?: string;
  created_at: string;
}

interface QuotaInfo {
  user_id: string;
  plan: string;
  compute_limit: number;
  token_limit: number;
  storage_limit: number;
  api_call_limit: number;
}

interface QuotaResponse {
  quota: QuotaInfo;
  usage: UsageSummary;
  remaining: {
    compute_seconds: number;
    token_count: number;
    storage_bytes: number;
    api_calls: number;
  };
}

interface DailyEntry {
  date: string;
  compute_seconds: number;
  token_count: number;
  api_calls: number;
}

interface DashboardResponse {
  period: string;
  daily: DailyEntry[];
}

// ─── Helpers ───────────────────────────────────────────────────────────────

type ChartView = "compute" | "tokens" | "api_calls";

function fmtNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function fmtBytes(bytes: number): string {
  if (bytes >= 1_073_741_824) return `${(bytes / 1_073_741_824).toFixed(1)} GB`;
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`;
  if (bytes >= 1_024) return `${(bytes / 1_024).toFixed(1)} KB`;
  return `${bytes} B`;
}

function fmtSeconds(sec: number): string {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m`;
  return `${sec}s`;
}

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
}

function pct(used: number, limit: number): number {
  if (!limit) return 0;
  return Math.min(100, Math.round((used / limit) * 100));
}

function getGreeting(): string {
  const h = new Date().getHours();
  if (h < 12) return "Good morning";
  if (h < 18) return "Good afternoon";
  return "Good evening";
}

// ─── Sub-components ────────────────────────────────────────────────────────

function Skeleton({ className = "", style }: { className?: string; style?: React.CSSProperties }) {
  return <div className={`animate-pulse rounded bg-muted ${className}`} style={style} />;
}

function StatCard({
  label,
  value,
  loading,
}: {
  label: string;
  value: string;
  loading: boolean;
}) {
  return (
    <div className="rounded-xl border border-border p-6 transition-colors hover:border-foreground/20">
      {loading ? (
        <>
          <Skeleton className="h-9 w-20" />
          <Skeleton className="mt-2 h-4 w-24" />
        </>
      ) : (
        <>
          <p className="text-3xl font-bold tracking-tight tabular-nums">
            {value}
          </p>
          <p className="mt-1 text-sm text-muted-foreground">{label}</p>
        </>
      )}
    </div>
  );
}

function UsageChart({
  daily,
  loading,
}: {
  daily: DailyEntry[];
  loading: boolean;
}) {
  const [view, setView] = useState<ChartView>("compute");

  const getValue = (entry: DailyEntry): number => {
    if (view === "compute") return entry.compute_seconds;
    if (view === "tokens") return entry.token_count;
    return entry.api_calls;
  };

  const formatValue = (n: number): string => {
    if (view === "compute") return fmtSeconds(n);
    return fmtNumber(n);
  };

  const maxVal = loading ? 1 : Math.max(...daily.map(getValue), 1);

  const views: { key: ChartView; label: string }[] = [
    { key: "compute", label: "Compute" },
    { key: "tokens", label: "Tokens" },
    { key: "api_calls", label: "API Calls" },
  ];

  return (
    <div className="rounded-xl border border-border p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h3 className="text-sm font-semibold">Daily Usage</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            {new Date().toLocaleDateString("en-US", {
              month: "long",
              year: "numeric",
            })}
          </p>
        </div>
        <div className="flex items-center gap-0.5 rounded-lg border border-border p-0.5">
          {views.map((v) => (
            <button
              key={v.key}
              onClick={() => setView(v.key)}
              className={`px-3 py-1 rounded-md text-xs font-medium transition-all duration-150 ${
                view === v.key
                  ? "bg-foreground text-background"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {v.label}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="flex items-end gap-1 h-44">
          {Array.from({ length: 20 }).map((_, i) => (
            <Skeleton
              key={i}
              className="flex-1"
              style={{ height: `${20 + Math.random() * 60}%` } as React.CSSProperties}
            />
          ))}
        </div>
      ) : daily.length === 0 ? (
        <div className="h-44 flex items-center justify-center text-sm text-muted-foreground">
          No data for this period
        </div>
      ) : (
        <>
          <div className="flex items-end gap-px h-44">
            {daily.map((entry) => {
              const val = getValue(entry);
              const heightPct = maxVal > 0 ? (val / maxVal) * 100 : 0;
              const dayLabel = new Date(entry.date).toLocaleDateString("en-US", {
                month: "numeric",
                day: "numeric",
              });
              return (
                <div
                  key={entry.date}
                  className="group relative flex-1 min-w-[6px] flex flex-col justify-end"
                  style={{ height: "100%" }}
                >
                  <div
                    className="rounded-sm bg-foreground/20 hover:bg-foreground/40 transition-colors cursor-default"
                    style={{ height: `${Math.max(heightPct, val > 0 ? 2 : 0)}%` }}
                  />
                  <div className="absolute bottom-full mb-2 left-1/2 -translate-x-1/2 z-10 hidden group-hover:flex flex-col items-center pointer-events-none">
                    <div className="bg-foreground text-background rounded-lg px-2.5 py-1.5 text-xs whitespace-nowrap">
                      <p className="font-medium">{dayLabel}</p>
                      <p className="opacity-70">{formatValue(val)}</p>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
          <div className="flex justify-between mt-2 text-[10px] text-muted-foreground">
            <span>
              {new Date(daily[0].date).toLocaleDateString("en-US", {
                month: "short",
                day: "numeric",
              })}
            </span>
            <span>
              {new Date(daily[daily.length - 1].date).toLocaleDateString("en-US", {
                month: "short",
                day: "numeric",
              })}
            </span>
          </div>
        </>
      )}
    </div>
  );
}

function RecentRunsTable({
  records,
  loading,
}: {
  records: UsageRecord[];
  loading: boolean;
}) {
  return (
    <div className="rounded-xl border border-border overflow-hidden">
      <div className="flex items-center justify-between px-6 py-4 border-b border-border">
        <h3 className="text-sm font-semibold">Recent Activity</h3>
        <Link
          href="/runs"
          className="text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          View all &rarr;
        </Link>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border bg-muted/30">
              <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground">
                Date
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground">
                Type
              </th>
              <th className="px-6 py-3 text-right text-xs font-medium text-muted-foreground">
                Amount
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground">
                Run ID
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground">
                Description
              </th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <tr key={i} className="border-b border-border last:border-0">
                  {Array.from({ length: 5 }).map((_, j) => (
                    <td key={j} className="px-6 py-3.5">
                      <Skeleton className="h-4 w-20" />
                    </td>
                  ))}
                </tr>
              ))
            ) : records.length === 0 ? (
              <tr>
                <td
                  colSpan={5}
                  className="px-6 py-12 text-center text-sm text-muted-foreground"
                >
                  No activity recorded yet
                </td>
              </tr>
            ) : (
              records.map((record) => (
                <tr
                  key={record.id}
                  className="border-b border-border last:border-0 transition-colors hover:bg-muted/30"
                >
                  <td className="px-6 py-3.5 text-sm text-muted-foreground whitespace-nowrap">
                    {fmtDate(record.created_at)}
                  </td>
                  <td className="px-6 py-3.5 whitespace-nowrap">
                    <span className="inline-flex items-center gap-1.5">
                      <span
                        className={`h-2 w-2 rounded-full ${
                          record.type === "compute"
                            ? "bg-foreground"
                            : record.type === "tokens"
                            ? "bg-foreground/60"
                            : record.type === "api"
                            ? "bg-foreground/40"
                            : "bg-foreground/20"
                        }`}
                      />
                      <span className="text-sm capitalize">{record.type}</span>
                    </span>
                  </td>
                  <td className="px-6 py-3.5 text-right font-mono text-sm tabular-nums whitespace-nowrap">
                    {fmtNumber(record.amount)}{" "}
                    <span className="text-muted-foreground text-xs">{record.unit}</span>
                  </td>
                  <td className="px-6 py-3.5 font-mono text-xs text-muted-foreground whitespace-nowrap">
                    {record.run_id
                      ? record.run_id.slice(0, 8)
                      : record.session_id
                      ? record.session_id.slice(0, 8)
                      : "\u2014"}
                  </td>
                  <td className="px-6 py-3.5 text-sm text-muted-foreground max-w-[240px] truncate">
                    {record.description || "\u2014"}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ─── Page ──────────────────────────────────────────────────────────────────

export default function DashboardPage() {
  const { user } = useAuth();

  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [history, setHistory] = useState<UsageRecord[]>([]);
  const [quotaData, setQuotaData] = useState<QuotaResponse | null>(null);
  const [dashData, setDashData] = useState<DashboardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const [summaryRes, historyRes, quotaRes, dashRes] = await Promise.allSettled([
          clientFetch("/api/v1/usage/summary").then((r) => r.json()),
          clientFetch("/api/v1/usage/history?limit=20").then((r) => r.json()),
          clientFetch("/api/v1/usage/quota").then((r) => r.json()),
          clientFetch("/api/v1/usage/dashboard").then((r) => r.json()),
        ]);

        if (summaryRes.status === "fulfilled") setSummary(summaryRes.value);
        if (historyRes.status === "fulfilled")
          setHistory(Array.isArray(historyRes.value) ? historyRes.value : []);
        if (quotaRes.status === "fulfilled") setQuotaData(quotaRes.value);
        if (dashRes.status === "fulfilled") setDashData(dashRes.value);
      } catch (e) {
        setError("Failed to load usage data. Please try again.");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const quota = quotaData?.quota ?? null;
  const usageSummary = quotaData?.usage ?? summary;

  const computeUsed = usageSummary?.compute_seconds ?? 0;
  const tokensUsed = usageSummary?.token_count ?? 0;
  const storageUsed = usageSummary?.storage_bytes ?? 0;
  const apiUsed = usageSummary?.api_calls ?? 0;

  const daily = dashData?.daily ?? [];

  const currentDate = new Date().toLocaleDateString("en-US", {
    weekday: "long",
    month: "long",
    day: "numeric",
    year: "numeric",
  });

  return (
    <div className="space-y-8">
      {/* Welcome header */}
      <div className="animate-fade-in">
        <h1 className="text-2xl font-bold tracking-tight">
          {getGreeting()}, {user?.name || "there"}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">{currentDate}</p>
      </div>

      {error && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Stats row */}
      <div className="animate-fade-in animate-delay-100 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Compute Time"
          value={loading ? "" : fmtSeconds(computeUsed)}
          loading={loading}
        />
        <StatCard
          label="Tokens Used"
          value={loading ? "" : fmtNumber(tokensUsed)}
          loading={loading}
        />
        <StatCard
          label="Storage"
          value={loading ? "" : fmtBytes(storageUsed)}
          loading={loading}
        />
        <StatCard
          label="API Calls"
          value={loading ? "" : fmtNumber(apiUsed)}
          loading={loading}
        />
      </div>

      {/* Usage chart */}
      <div className="animate-fade-in animate-delay-200">
        <UsageChart daily={daily} loading={loading} />
      </div>

      {/* Activity table */}
      <div className="animate-fade-in animate-delay-300">
        <RecentRunsTable records={history} loading={loading} />
      </div>
    </div>
  );
}
