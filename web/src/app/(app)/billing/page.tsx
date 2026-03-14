"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { clientFetch } from "@/lib/api";

interface UsageSummary {
  user_id: string;
  agent_id: string;
  period: string;
  total_runs: number;
  total_tokens: number;
  total_cost_micros: number;
  by_model?: { model: string; runs: number; input_tokens: number; output_tokens: number; cost_micros: number }[];
}

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

interface Revenue {
  total_gross: number;
  total_earnings: number;
  total_paid: number;
  pending: number;
  share: { author_percent: number; platform_percent: number };
  payouts: any[];
}

function formatCost(micros: number): string {
  return `$${(micros / 1_000_000).toFixed(4)}`;
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

export default function BillingPage() {
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [subs, setSubs] = useState<Subscription[]>([]);
  const [revenue, setRevenue] = useState<Revenue | null>(null);
  const [tab, setTab] = useState<"usage" | "subscriptions" | "revenue">("usage");

  useEffect(() => {
    clientFetch("/api/v1/billing/usage/summary").then(r => r.json()).then(setUsage).catch(() => {});
    clientFetch("/api/v1/billing/subscriptions").then(r => r.json()).then(d => setSubs(d || [])).catch(() => {});
    clientFetch("/api/v1/billing/revenue").then(r => r.json()).then(setRevenue).catch(() => {});
  }, []);

  const STATUS_COLORS: Record<string, string> = {
    active: "bg-green-100 text-green-700",
    trialing: "bg-blue-100 text-blue-700",
    past_due: "bg-yellow-100 text-yellow-700",
    canceled: "bg-gray-100 text-gray-500",
    expired: "bg-red-100 text-red-700",
  };

  async function handleCancel(subId: string) {
    await clientFetch(`/api/v1/billing/subscriptions/${subId}/cancel`, { method: "POST" });
    setSubs(prev => prev.map(s => s.id === subId ? { ...s, status: "canceled" } : s));
  }

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Billing & Usage</h1>
        <p className="text-muted-foreground">Monitor your agent usage, subscriptions, and earnings.</p>
      </div>

      {/* Tab nav */}
      <div className="flex gap-2 border-b pb-2">
        {(["usage", "subscriptions", "revenue"] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm rounded-t-md ${tab === t ? "bg-primary text-primary-foreground" : "hover:bg-muted"}`}
          >
            {t === "usage" ? "📊 Usage" : t === "subscriptions" ? "📋 Subscriptions" : "💰 Revenue"}
          </button>
        ))}
      </div>

      {/* Usage Tab */}
      {tab === "usage" && (
        <div className="space-y-4">
          <div className="grid grid-cols-3 gap-4">
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Total Runs</CardTitle></CardHeader>
              <CardContent><p className="text-3xl font-bold">{usage?.total_runs ?? 0}</p></CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Total Tokens</CardTitle></CardHeader>
              <CardContent><p className="text-3xl font-bold">{formatTokens(usage?.total_tokens ?? 0)}</p></CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Total Cost</CardTitle></CardHeader>
              <CardContent><p className="text-3xl font-bold">{formatCost(usage?.total_cost_micros ?? 0)}</p></CardContent>
            </Card>
          </div>

          {usage?.by_model && usage.by_model.length > 0 && (
            <Card>
              <CardHeader><CardTitle>By Model</CardTitle></CardHeader>
              <CardContent>
                <table className="w-full text-sm">
                  <thead><tr className="border-b">
                    <th className="text-left py-2">Model</th>
                    <th className="text-right">Runs</th>
                    <th className="text-right">Input</th>
                    <th className="text-right">Output</th>
                    <th className="text-right">Cost</th>
                  </tr></thead>
                  <tbody>
                    {usage.by_model.map(m => (
                      <tr key={m.model} className="border-b">
                        <td className="py-2 font-mono">{m.model}</td>
                        <td className="text-right">{m.runs}</td>
                        <td className="text-right">{formatTokens(m.input_tokens)}</td>
                        <td className="text-right">{formatTokens(m.output_tokens)}</td>
                        <td className="text-right">{formatCost(m.cost_micros)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* Subscriptions Tab */}
      {tab === "subscriptions" && (
        <div className="space-y-3">
          {subs.length === 0 && (
            <p className="text-muted-foreground">No subscriptions yet. Browse agents to subscribe.</p>
          )}
          {subs.map(sub => (
            <Card key={sub.id}>
              <CardContent className="pt-4 flex items-center justify-between">
                <div>
                  <p className="font-medium">{sub.agent_id}</p>
                  <p className="text-xs text-muted-foreground">
                    {sub.pricing_model} · Since {new Date(sub.created_at).toLocaleDateString()}
                    {sub.trial_ends_at && ` · Trial ends ${new Date(sub.trial_ends_at).toLocaleDateString()}`}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <Badge className={STATUS_COLORS[sub.status] || ""}>{sub.status}</Badge>
                  {(sub.status === "active" || sub.status === "trialing") && (
                    <Button variant="outline" size="sm" onClick={() => handleCancel(sub.id)}>Cancel</Button>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Revenue Tab */}
      {tab === "revenue" && (
        <div className="space-y-4">
          <div className="grid grid-cols-4 gap-4">
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Gross Revenue</CardTitle></CardHeader>
              <CardContent><p className="text-2xl font-bold">{formatCost(revenue?.total_gross ?? 0)}</p></CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Your Earnings ({revenue?.share.author_percent ?? 70}%)</CardTitle></CardHeader>
              <CardContent><p className="text-2xl font-bold text-green-600">{formatCost(revenue?.total_earnings ?? 0)}</p></CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Paid Out</CardTitle></CardHeader>
              <CardContent><p className="text-2xl font-bold">{formatCost(revenue?.total_paid ?? 0)}</p></CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Pending</CardTitle></CardHeader>
              <CardContent><p className="text-2xl font-bold text-orange-500">{formatCost(revenue?.pending ?? 0)}</p></CardContent>
            </Card>
          </div>

          {revenue?.payouts && revenue.payouts.length > 0 && (
            <Card>
              <CardHeader><CardTitle>Payout History</CardTitle></CardHeader>
              <CardContent>
                <table className="w-full text-sm">
                  <thead><tr className="border-b">
                    <th className="text-left py-2">Period</th>
                    <th className="text-right">Gross</th>
                    <th className="text-right">Platform Fee</th>
                    <th className="text-right">Earnings</th>
                    <th className="text-right">Status</th>
                  </tr></thead>
                  <tbody>
                    {revenue.payouts.map((p: any) => (
                      <tr key={p.id} className="border-b">
                        <td className="py-2">{p.period}</td>
                        <td className="text-right">{formatCost(p.gross_revenue)}</td>
                        <td className="text-right">{formatCost(p.platform_fee)}</td>
                        <td className="text-right font-medium text-green-600">{formatCost(p.author_earnings)}</td>
                        <td className="text-right">
                          <Badge variant="outline" className={p.status === "paid" ? "text-green-700" : "text-orange-500"}>
                            {p.status}
                          </Badge>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          )}
        </div>
      )}
    </div>
  );
}
