"use client";

import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useAuth } from "@/lib/auth";
import { clientFetch } from "@/lib/api";

// ─── Types ────────────────────────────────────────────────────────────────────

interface QuotaResponse {
  quota: {
    user_id: string;
    plan: string;
    compute_limit: number;
    token_limit: number;
    storage_limit: number;
    api_call_limit: number;
  };
  usage: {
    compute_seconds: number;
    token_count: number;
    storage_bytes: number;
    api_calls: number;
  };
  remaining: {
    compute_seconds: number;
    token_count: number;
    storage_bytes: number;
    api_calls: number;
  };
}

interface Subscription {
  id: string;
  plan_id: string;
  status: string;
  current_period_end: string;
  cancel_at_period_end?: boolean;
}

interface UsageRecord {
  id: string;
  created_at: string;
  amount: number;
  currency?: string;
  status: string;
  invoice_pdf?: string;
  description?: string;
}

// ─── Plan definitions ─────────────────────────────────────────────────────────

interface PlanDef {
  id: string;
  name: string;
  price: number | null;
  computeMin: number;
  tokens: string;
  storage: string;
  apiCalls: string;
  seats?: number;
  highlight?: boolean;
}

const PLANS: PlanDef[] = [
  {
    id: "free",
    name: "Free",
    price: null,
    computeMin: 100,
    tokens: "100K",
    storage: "100 MB",
    apiCalls: "1,000",
  },
  {
    id: "pro",
    name: "Pro",
    price: 29,
    computeMin: 2000,
    tokens: "2M",
    storage: "10 GB",
    apiCalls: "Unlimited",
    highlight: true,
  },
  {
    id: "team",
    name: "Team",
    price: 99,
    computeMin: 10000,
    tokens: "10M",
    storage: "100 GB",
    apiCalls: "Unlimited",
    seats: 5,
  },
];

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes >= 1_073_741_824) return `${(bytes / 1_073_741_824).toFixed(1)} GB`;
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
  return String(n);
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  } catch {
    return iso;
  }
}

function computeUsedMinutes(seconds: number): number {
  return Math.round(seconds / 60);
}

function pct(used: number, limit: number): number {
  if (limit <= 0) return 0;
  return Math.min(100, Math.round((used / limit) * 100));
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function UsageBar({ label, used, limit, formatUsed, formatLimit }: {
  label: string;
  used: number;
  limit: number;
  formatUsed: (n: number) => string;
  formatLimit: (n: number) => string;
}) {
  const percent = pct(used, limit);
  const isWarning = percent >= 75;
  const isDanger = percent >= 90;

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-sm">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium tabular-nums">
          {formatUsed(used)}
          <span className="text-muted-foreground font-normal"> / {formatLimit(limit)}</span>
        </span>
      </div>
      <div className="h-2 w-full rounded-full bg-muted overflow-hidden">
        <div
          className={`h-full rounded-full transition-all ${
            isDanger
              ? "bg-red-500"
              : isWarning
              ? "bg-amber-500"
              : "bg-primary"
          }`}
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    active: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
    paid: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
    trialing: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
    past_due: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
    pending: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
    canceled: "bg-muted text-muted-foreground",
    failed: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
  };
  return (
    <Badge className={`capitalize text-xs font-medium ${map[status] ?? "bg-muted text-muted-foreground"}`}>
      {status.replace("_", " ")}
    </Badge>
  );
}

function SkeletonLine({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded bg-muted ${className}`} />;
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function BillingPage() {
  const { loading: authLoading } = useAuth();

  const [quota, setQuota] = useState<QuotaResponse | null>(null);
  const [subscriptions, setSubscriptions] = useState<Subscription[] | null>(null);
  const [records, setRecords] = useState<UsageRecord[] | null>(null);

  const [quotaLoading, setQuotaLoading] = useState(true);
  const [subsLoading, setSubsLoading] = useState(true);
  const [recordsLoading, setRecordsLoading] = useState(true);

  const [portalLoading, setPortalLoading] = useState(false);
  const [checkoutLoading, setCheckoutLoading] = useState<string | null>(null);

  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) return;

    clientFetch("/api/v1/usage/quota")
      .then((r) => r.json())
      .then((d) => setQuota(d))
      .catch(() => setQuota(null))
      .finally(() => setQuotaLoading(false));

    clientFetch("/api/v1/billing/subscriptions")
      .then((r) => r.json())
      .then((d) => setSubscriptions(Array.isArray(d) ? d : []))
      .catch(() => setSubscriptions([]))
      .finally(() => setSubsLoading(false));

    clientFetch("/api/v1/billing/usage/records")
      .then((r) => r.json())
      .then((d) => setRecords(Array.isArray(d) ? d : []))
      .catch(() => setRecords([]))
      .finally(() => setRecordsLoading(false));
  }, [authLoading]);

  async function handleManageSubscription() {
    setPortalLoading(true);
    setError(null);
    try {
      const res = await clientFetch("/api/v1/billing/portal");
      const data = await res.json();
      if (data?.url) {
        window.location.href = data.url;
      } else {
        setError("Could not open billing portal. Please try again.");
      }
    } catch {
      setError("Could not open billing portal. Please try again.");
    } finally {
      setPortalLoading(false);
    }
  }

  async function handleUpgrade(planId: string) {
    setCheckoutLoading(planId);
    setError(null);
    try {
      const res = await clientFetch("/api/v1/billing/checkout", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ plan_id: planId }),
      });
      const data = await res.json();
      if (data?.url) {
        window.location.href = data.url;
      } else {
        setError("Could not start checkout. Please try again.");
      }
    } catch {
      setError("Could not start checkout. Please try again.");
    } finally {
      setCheckoutLoading(null);
    }
  }

  // Derive current plan from quota or first active subscription
  const currentPlanId = quota?.quota?.plan?.toLowerCase() ?? "free";
  const activeSub = subscriptions?.find(
    (s) => s.status === "active" || s.status === "trialing"
  );
  const renewalDate = activeSub?.current_period_end
    ? formatDate(activeSub.current_period_end)
    : null;
  const subStatus = activeSub?.status ?? (currentPlanId === "free" ? "active" : null);

  // Payment method: mock last4 from subscription metadata if available
  const last4 = (activeSub as any)?.payment_method_last4 ?? null;

  if (authLoading) {
    return (
      <div className="max-w-5xl mx-auto space-y-6 py-4">
        <SkeletonLine className="h-8 w-48" />
        <SkeletonLine className="h-4 w-72" />
        <SkeletonLine className="h-48 w-full" />
      </div>
    );
  }

  return (
    <div className="max-w-5xl mx-auto space-y-8 pb-16">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Billing</h1>
        <p className="text-muted-foreground mt-1">
          Manage your plan, usage, and payment details.
        </p>
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/30 px-4 py-3 text-sm text-red-700 dark:text-red-400">
          {error}
        </div>
      )}

      {/* ── Current Plan ── */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">Current Plan</h2>
        <Card>
          <CardContent className="pt-6">
            {quotaLoading || subsLoading ? (
              <div className="space-y-3">
                <SkeletonLine className="h-6 w-40" />
                <SkeletonLine className="h-4 w-60" />
                <SkeletonLine className="h-2 w-full" />
                <SkeletonLine className="h-2 w-full" />
              </div>
            ) : (
              <div className="space-y-5">
                {/* Plan name + status + renew */}
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-xl font-bold capitalize">
                        {currentPlanId} Plan
                      </span>
                      {subStatus && <StatusBadge status={subStatus} />}
                      {activeSub?.cancel_at_period_end && (
                        <Badge className="bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 text-xs">
                          Cancels at period end
                        </Badge>
                      )}
                    </div>
                    {renewalDate && (
                      <p className="text-sm text-muted-foreground">
                        Renews on {renewalDate}
                      </p>
                    )}
                    {!activeSub && currentPlanId === "free" && (
                      <p className="text-sm text-muted-foreground">
                        Free tier — no renewal required
                      </p>
                    )}
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleManageSubscription}
                    disabled={portalLoading}
                    className="shrink-0"
                  >
                    {portalLoading ? "Opening..." : "Manage Subscription"}
                  </Button>
                </div>

                {/* Usage bars */}
                {quota ? (
                  <div className="space-y-3 pt-1">
                    <UsageBar
                      label="Compute"
                      used={computeUsedMinutes(quota.usage.compute_seconds)}
                      limit={quota.quota.compute_limit}
                      formatUsed={(n) => `${n} min`}
                      formatLimit={(n) => `${n.toLocaleString()} min`}
                    />
                    <UsageBar
                      label="Tokens"
                      used={quota.usage.token_count}
                      limit={quota.quota.token_limit}
                      formatUsed={formatTokens}
                      formatLimit={formatTokens}
                    />
                    <UsageBar
                      label="Storage"
                      used={quota.usage.storage_bytes}
                      limit={quota.quota.storage_limit}
                      formatUsed={formatBytes}
                      formatLimit={formatBytes}
                    />
                    {quota.quota.api_call_limit > 0 && (
                      <UsageBar
                        label="API Calls"
                        used={quota.usage.api_calls}
                        limit={quota.quota.api_call_limit}
                        formatUsed={(n) => n.toLocaleString()}
                        formatLimit={(n) => n.toLocaleString()}
                      />
                    )}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    Usage data unavailable.
                  </p>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      </section>

      {/* ── Plan Comparison ── */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">Plans</h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {PLANS.map((plan) => {
            const isCurrent = plan.id === currentPlanId;
            const isDowngrade =
              PLANS.findIndex((p) => p.id === plan.id) <
              PLANS.findIndex((p) => p.id === currentPlanId);

            return (
              <Card
                key={plan.id}
                className={`relative flex flex-col transition-shadow ${
                  isCurrent
                    ? "border-primary ring-2 ring-primary shadow-md"
                    : plan.highlight
                    ? "border-border shadow-sm"
                    : "border-border"
                }`}
              >
                {isCurrent && (
                  <div className="absolute -top-3 left-1/2 -translate-x-1/2">
                    <span className="rounded-full bg-primary px-3 py-0.5 text-xs font-semibold text-primary-foreground shadow">
                      Current plan
                    </span>
                  </div>
                )}
                {plan.highlight && !isCurrent && (
                  <div className="absolute -top-3 left-1/2 -translate-x-1/2">
                    <span className="rounded-full bg-blue-600 px-3 py-0.5 text-xs font-semibold text-white shadow">
                      Most popular
                    </span>
                  </div>
                )}

                <CardHeader className="pb-2 pt-6">
                  <CardTitle className="text-base">{plan.name}</CardTitle>
                  <div className="mt-1">
                    {plan.price === null ? (
                      <span className="text-3xl font-bold">Free</span>
                    ) : (
                      <>
                        <span className="text-3xl font-bold">${plan.price}</span>
                        <span className="text-sm text-muted-foreground">/mo</span>
                      </>
                    )}
                  </div>
                </CardHeader>

                <CardContent className="flex flex-col flex-1 gap-4">
                  <ul className="space-y-2 text-sm">
                    <li className="flex items-center gap-2">
                      <span className="text-muted-foreground text-xs">Compute</span>
                      <span className="ml-auto font-medium">
                        {plan.computeMin.toLocaleString()} min/mo
                      </span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-muted-foreground text-xs">Tokens</span>
                      <span className="ml-auto font-medium">{plan.tokens}</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-muted-foreground text-xs">Storage</span>
                      <span className="ml-auto font-medium">{plan.storage}</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-muted-foreground text-xs">API calls</span>
                      <span className="ml-auto font-medium">{plan.apiCalls}</span>
                    </li>
                    {plan.seats && (
                      <li className="flex items-center gap-2">
                        <span className="text-muted-foreground text-xs">Team seats</span>
                        <span className="ml-auto font-medium">{plan.seats}</span>
                      </li>
                    )}
                  </ul>

                  <div className="mt-auto pt-2">
                    {isCurrent ? (
                      <Button
                        variant="outline"
                        size="sm"
                        className="w-full"
                        disabled
                      >
                        Current plan
                      </Button>
                    ) : isDowngrade ? (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="w-full text-muted-foreground"
                        onClick={handleManageSubscription}
                        disabled={portalLoading}
                      >
                        Downgrade
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        className="w-full"
                        onClick={() => handleUpgrade(plan.id)}
                        disabled={checkoutLoading === plan.id}
                        variant={plan.highlight ? "default" : "outline"}
                      >
                        {checkoutLoading === plan.id ? "Redirecting..." : "Upgrade"}
                      </Button>
                    )}
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </section>

      {/* ── Payment Method ── */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">Payment Method</h2>
        <Card>
          <CardContent className="pt-6">
            {subsLoading ? (
              <div className="flex items-center justify-between">
                <SkeletonLine className="h-5 w-48" />
                <SkeletonLine className="h-8 w-32" />
              </div>
            ) : (
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-center gap-3">
                  {/* Card icon */}
                  <div className="flex h-10 w-16 items-center justify-center rounded-md border bg-muted">
                    <svg
                      className="h-5 w-8 text-muted-foreground"
                      viewBox="0 0 32 20"
                      fill="none"
                      aria-hidden="true"
                    >
                      <rect width="32" height="20" rx="3" fill="currentColor" opacity="0.15" />
                      <rect x="2" y="7" width="28" height="3" fill="currentColor" opacity="0.4" />
                      <rect x="2" y="13" width="8" height="2" rx="1" fill="currentColor" opacity="0.4" />
                    </svg>
                  </div>
                  <div>
                    {last4 ? (
                      <>
                        <p className="text-sm font-medium">Card ending in {last4}</p>
                        <p className="text-xs text-muted-foreground">Used for subscription billing</p>
                      </>
                    ) : activeSub ? (
                      <>
                        <p className="text-sm font-medium">Payment method on file</p>
                        <p className="text-xs text-muted-foreground">Managed via Stripe</p>
                      </>
                    ) : (
                      <>
                        <p className="text-sm font-medium">No payment method</p>
                        <p className="text-xs text-muted-foreground">Add a card to upgrade your plan</p>
                      </>
                    )}
                  </div>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleManageSubscription}
                  disabled={portalLoading}
                  className="shrink-0"
                >
                  {portalLoading ? "Opening..." : "Update Payment"}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
      </section>

      {/* ── Invoice History ── */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">Invoice History</h2>
        <Card>
          <CardContent className="pt-0">
            {recordsLoading ? (
              <div className="space-y-3 pt-6">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="flex items-center justify-between gap-4">
                    <SkeletonLine className="h-4 w-28" />
                    <SkeletonLine className="h-4 w-16" />
                    <SkeletonLine className="h-5 w-16" />
                    <SkeletonLine className="h-4 w-10" />
                  </div>
                ))}
              </div>
            ) : !records || records.length === 0 ? (
              <div className="py-10 text-center text-sm text-muted-foreground">
                No invoices yet.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b">
                      <th className="py-3 text-left font-medium text-muted-foreground">Date</th>
                      <th className="py-3 text-left font-medium text-muted-foreground hidden sm:table-cell">
                        Description
                      </th>
                      <th className="py-3 text-right font-medium text-muted-foreground">Amount</th>
                      <th className="py-3 text-center font-medium text-muted-foreground">Status</th>
                      <th className="py-3 text-right font-medium text-muted-foreground">Invoice</th>
                    </tr>
                  </thead>
                  <tbody>
                    {records.map((record) => (
                      <tr
                        key={record.id}
                        className="border-b last:border-0 hover:bg-muted/40 transition-colors"
                      >
                        <td className="py-3 text-muted-foreground whitespace-nowrap">
                          {formatDate(record.created_at)}
                        </td>
                        <td className="py-3 text-muted-foreground hidden sm:table-cell max-w-xs truncate">
                          {record.description ?? "Subscription charge"}
                        </td>
                        <td className="py-3 text-right font-medium tabular-nums whitespace-nowrap">
                          {record.currency
                            ? new Intl.NumberFormat("en-US", {
                                style: "currency",
                                currency: record.currency.toUpperCase(),
                              }).format(record.amount / 100)
                            : `$${(record.amount / 100).toFixed(2)}`}
                        </td>
                        <td className="py-3 text-center">
                          <StatusBadge status={record.status} />
                        </td>
                        <td className="py-3 text-right">
                          {record.invoice_pdf ? (
                            <a
                              href={record.invoice_pdf}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-xs font-medium text-primary underline-offset-4 hover:underline"
                            >
                              PDF
                            </a>
                          ) : (
                            <span className="text-xs text-muted-foreground">—</span>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
