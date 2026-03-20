"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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

const TYPE_LABELS: Record<string, string> = {
  compute: "Compute",
  tokens: "Tokens",
  storage: "Storage",
  api: "API",
};

const TYPE_EMOJI: Record<string, string> = {
  compute: "🖥️",
  tokens: "🔤",
  storage: "💾",
  api: "🔌",
};

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
    year: "numeric",
  });
}

function quotaColor(pct: number): string {
  if (pct > 80) return "bg-red-500";
  if (pct > 60) return "bg-yellow-500";
  return "bg-emerald-500";
}

function quotaTextColor(pct: number): string {
  if (pct > 80) return "text-red-500";
  if (pct > 60) return "text-yellow-500";
  return "text-emerald-500";
}

function pct(used: number, limit: number): number {
  if (!limit) return 0;
  return Math.min(100, Math.round((used / limit) * 100));
}

// ─── Sub-components ────────────────────────────────────────────────────────

function SkeletonBar({ className = "" }: { className?: string }) {
  return (
    <div
      className={`animate-pulse rounded bg-muted ${className}`}
    />
  );
}

function QuotaCard({
  title,
  usedLabel,
  limitLabel,
  percent,
  loading,
}: {
  title: string;
  usedLabel: string;
  limitLabel: string;
  percent: number;
  loading: boolean;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {loading ? (
          <>
            <SkeletonBar className="h-7 w-24" />
            <SkeletonBar className="h-2 w-full" />
          </>
        ) : (
          <>
            <div className="flex items-baseline justify-between gap-2">
              <span className={`text-2xl font-bold tabular-nums ${quotaTextColor(percent)}`}>
                {usedLabel}
              </span>
              <span className="text-xs text-muted-foreground whitespace-nowrap">
                / {limitLabel}
              </span>
            </div>
            <div className="h-2 w-full rounded-full bg-muted overflow-hidden">
              <div
                className={`h-full rounded-full transition-all duration-500 ${quotaColor(percent)}`}
                style={{ width: `${percent}%` }}
              />
            </div>
            <p className="text-xs text-muted-foreground">{percent}% used</p>
          </>
        )}
      </CardContent>
    </Card>
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

  const viewBtnClass = (v: ChartView) =>
    `px-3 py-1 rounded-md text-xs font-medium transition-colors ${
      view === v
        ? "bg-primary text-primary-foreground"
        : "text-muted-foreground hover:text-foreground hover:bg-muted"
    }`;

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between flex-wrap gap-2">
          <CardTitle className="text-base font-semibold">
            Daily Usage — {new Date().toLocaleDateString("en-US", { month: "long", year: "numeric" })}
          </CardTitle>
          <div className="flex items-center gap-1 rounded-lg border p-1">
            <button className={viewBtnClass("compute")} onClick={() => setView("compute")}>
              Compute
            </button>
            <button className={viewBtnClass("tokens")} onClick={() => setView("tokens")}>
              Tokens
            </button>
            <button className={viewBtnClass("api_calls")} onClick={() => setView("api_calls")}>
              API Calls
            </button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex items-end gap-1 h-40">
            {Array.from({ length: 20 }).map((_, i) => (
              <SkeletonBar
                key={i}
                className="flex-1"
                style={{ height: `${20 + Math.random() * 60}%` } as React.CSSProperties}
              />
            ))}
          </div>
        ) : daily.length === 0 ? (
          <div className="h-40 flex items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <div className="flex items-end gap-px sm:gap-1 h-40 overflow-x-auto">
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
                  className="group relative flex-1 min-w-[10px] flex flex-col justify-end"
                  style={{ height: "100%" }}
                >
                  <div
                    className="rounded-sm bg-primary/70 hover:bg-primary transition-colors cursor-default"
                    style={{ height: `${Math.max(heightPct, val > 0 ? 2 : 0)}%` }}
                  />
                  {/* Tooltip */}
                  <div className="absolute bottom-full mb-1 left-1/2 -translate-x-1/2 z-10 hidden group-hover:flex flex-col items-center pointer-events-none">
                    <div className="bg-popover border rounded shadow-md px-2 py-1 text-xs whitespace-nowrap">
                      <p className="font-medium">{dayLabel}</p>
                      <p className="text-muted-foreground">{formatValue(val)}</p>
                    </div>
                    <div className="w-px h-1 bg-border" />
                  </div>
                </div>
              );
            })}
          </div>
        )}
        {!loading && daily.length > 0 && (
          <div className="flex justify-between mt-1 text-[10px] text-muted-foreground">
            <span>
              {new Date(daily[0].date).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
            </span>
            <span>
              {new Date(daily[daily.length - 1].date).toLocaleDateString("en-US", {
                month: "short",
                day: "numeric",
              })}
            </span>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function TypeBadge({ type }: { type: string }) {
  const emoji = TYPE_EMOJI[type] ?? "📦";
  const label = TYPE_LABELS[type] ?? type;

  const colorMap: Record<string, string> = {
    compute: "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
    tokens: "bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300",
    storage: "bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300",
    api: "bg-teal-100 text-teal-800 dark:bg-teal-900/30 dark:text-teal-300",
  };

  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${
        colorMap[type] ?? "bg-muted text-muted-foreground"
      }`}
    >
      <span>{emoji}</span>
      {label}
    </span>
  );
}

function ActivityTable({
  records,
  loading,
}: {
  records: UsageRecord[];
  loading: boolean;
}) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base font-semibold">Recent Activity</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40">
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground whitespace-nowrap">
                  Date
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground whitespace-nowrap">
                  Type
                </th>
                <th className="px-4 py-2.5 text-right text-xs font-medium text-muted-foreground whitespace-nowrap">
                  Amount
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground whitespace-nowrap">
                  Run / Session
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Description
                </th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <tr key={i} className="border-b">
                    {Array.from({ length: 5 }).map((_, j) => (
                      <td key={j} className="px-4 py-3">
                        <SkeletonBar className="h-4 w-full max-w-[120px]" />
                      </td>
                    ))}
                  </tr>
                ))
              ) : records.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center text-sm text-muted-foreground">
                    No activity recorded yet
                  </td>
                </tr>
              ) : (
                records.map((record) => (
                  <tr
                    key={record.id}
                    className="border-b last:border-0 hover:bg-muted/30 transition-colors"
                  >
                    <td className="px-4 py-3 text-xs text-muted-foreground whitespace-nowrap">
                      {fmtDate(record.created_at)}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <TypeBadge type={record.type} />
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs whitespace-nowrap">
                      {fmtNumber(record.amount)}{" "}
                      <span className="text-muted-foreground">{record.unit}</span>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-muted-foreground whitespace-nowrap">
                      {record.run_id
                        ? record.run_id.slice(0, 8)
                        : record.session_id
                        ? record.session_id.slice(0, 8)
                        : "—"}
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground max-w-[240px] truncate">
                      {record.description || "—"}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  );
}

function PlanCard({
  quota,
  summary,
  loading,
}: {
  quota: QuotaInfo | null;
  summary: UsageSummary | null;
  loading: boolean;
}) {
  const tierColor = (plan: string) => {
    const p = plan?.toLowerCase() ?? "";
    if (p.includes("pro")) return "bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300";
    if (p.includes("team") || p.includes("enterprise"))
      return "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300";
    return "bg-muted text-muted-foreground";
  };

  const planName = quota?.plan ?? "Free";
  const period = summary?.period ?? "";

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base font-semibold">Current Plan</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading ? (
          <>
            <SkeletonBar className="h-6 w-24" />
            <SkeletonBar className="h-4 w-40" />
          </>
        ) : (
          <>
            <div className="flex items-center gap-2">
              <span className="text-2xl font-bold capitalize">{planName}</span>
              <span
                className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold ${tierColor(
                  planName
                )}`}
              >
                {planName.toUpperCase()}
              </span>
            </div>
            {period && (
              <p className="text-xs text-muted-foreground">
                Billing period:{" "}
                <span className="font-medium text-foreground">{period}</span>
              </p>
            )}
            {summary?.estimated_cost != null && (
              <p className="text-xs text-muted-foreground">
                Estimated cost this period:{" "}
                <span className="font-medium text-foreground">
                  ${summary.estimated_cost.toFixed(2)}
                </span>
              </p>
            )}
            <Link href="/billing">
              <Button size="sm" className="mt-2 w-full sm:w-auto">
                Upgrade Plan
              </Button>
            </Link>
          </>
        )}
      </CardContent>
    </Card>
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

  // Derived quota values
  const quota = quotaData?.quota ?? null;
  const usageSummary = quotaData?.usage ?? summary;

  const computeUsed = usageSummary?.compute_seconds ?? 0;
  const computeLimit = quota?.compute_limit ?? 0;
  const computePct = pct(computeUsed, computeLimit);

  const tokensUsed = usageSummary?.token_count ?? 0;
  const tokensLimit = quota?.token_limit ?? 0;
  const tokensPct = pct(tokensUsed, tokensLimit);

  const storageUsed = usageSummary?.storage_bytes ?? 0;
  const storageLimit = quota?.storage_limit ?? 0;
  const storagePct = pct(storageUsed, storageLimit);

  const apiUsed = usageSummary?.api_calls ?? 0;
  const apiLimit = quota?.api_call_limit ?? 0;
  const apiPct = pct(apiUsed, apiLimit);

  const daily = dashData?.daily ?? [];

  return (
    <div className="space-y-8">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          Usage Dashboard
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          Monitor your resource consumption and quota usage
          {user?.name ? `, ${user.name}` : ""}.
        </p>
      </div>

      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Top stats row */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <QuotaCard
          title="Compute Time"
          usedLabel={fmtSeconds(computeUsed)}
          limitLabel={computeLimit ? fmtSeconds(computeLimit) : "Unlimited"}
          percent={computePct}
          loading={loading}
        />
        <QuotaCard
          title="Tokens Used"
          usedLabel={fmtNumber(tokensUsed)}
          limitLabel={tokensLimit ? fmtNumber(tokensLimit) : "Unlimited"}
          percent={tokensPct}
          loading={loading}
        />
        <QuotaCard
          title="Storage"
          usedLabel={fmtBytes(storageUsed)}
          limitLabel={storageLimit ? fmtBytes(storageLimit) : "Unlimited"}
          percent={storagePct}
          loading={loading}
        />
        <QuotaCard
          title="API Calls"
          usedLabel={fmtNumber(apiUsed)}
          limitLabel={apiLimit ? fmtNumber(apiLimit) : "Unlimited"}
          percent={apiPct}
          loading={loading}
        />
      </div>

      {/* Chart + Plan side-by-side on large screens */}
      <div className="grid gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <UsageChart daily={daily} loading={loading} />
        </div>
        <div>
          <PlanCard quota={quota} summary={usageSummary ?? null} loading={loading} />
        </div>
      </div>

      {/* Activity table */}
      <ActivityTable records={history} loading={loading} />
    </div>
  );
}
