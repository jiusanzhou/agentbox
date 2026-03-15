"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { clientFetch } from "@/lib/api";

const LEVEL_LABELS: Record<string, string> = {
  junior: "🌱 Junior",
  mid: "🌿 Mid",
  senior: "🌳 Senior",
  expert: "⭐ Expert",
};

const DIFFICULTY_COLORS: Record<string, string> = {
  basic: "bg-gray-100 text-gray-700",
  intermediate: "bg-blue-100 text-blue-700",
  advanced: "bg-orange-100 text-orange-700",
  expert: "bg-purple-100 text-purple-700",
};

const RUN_STATUS_COLORS: Record<string, string> = {
  pending: "bg-yellow-100 text-yellow-700",
  running: "bg-blue-100 text-blue-700",
  completed: "bg-green-100 text-green-700",
  failed: "bg-red-100 text-red-700",
};

function formatTime(ts: string): string {
  return new Date(ts).toLocaleString();
}

export default function AgentDetailPage() {
  const params = useParams();
  const router = useRouter();
  const [agent, setAgent] = useState<any>(null);
  const [manifest, setManifest] = useState<any>(null);
  const [runs, setRuns] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [hireOpen, setHireOpen] = useState(false);
  const [hireMessage, setHireMessage] = useState("");
  const [hireExecutor, setHireExecutor] = useState("");
  const [hiring, setHiring] = useState(false);
  const [hireError, setHireError] = useState("");
  const [dnaTab, setDnaTab] = useState<"identity" | "soul" | "tools">("identity");

  useEffect(() => {
    loadAgent();
    loadRuns();
  }, [params.id]);

  async function loadAgent() {
    try {
      const resp = await clientFetch(`/api/v1/registry/agents/${params.id}`);
      const data = await resp.json();
      setAgent(data);

      try {
        const mResp = await clientFetch(`/api/v1/registry/agents/${params.id}/manifest`);
        const m = await mResp.json();
        setManifest(m);
      } catch {
        // manifest endpoint might not have data
      }
    } catch (e) {
      console.error("Failed to load agent:", e);
    } finally {
      setLoading(false);
    }
  }

  async function loadRuns() {
    try {
      const resp = await fetch("/api/runs");
      const allRuns = await resp.json();
      if (Array.isArray(allRuns)) {
        // Filter runs by agent name (best-effort match)
        const id = String(params.id);
        const filtered = allRuns.filter((r: any) =>
          r.agent_dna_id === id || r.name?.toLowerCase().includes(id.toLowerCase())
        );
        setRuns(filtered.slice(0, 5));
      }
    } catch {
      // ignore
    }
  }

  async function handleHire() {
    setHiring(true);
    setHireError("");
    try {
      const body: any = { message: hireMessage };
      if (hireExecutor) body.executor = hireExecutor;

      const resp = await clientFetch(`/api/v1/registry/agents/${params.id}/hire`, {
        method: "POST",
        body: JSON.stringify(body),
      });

      if (!resp.ok) {
        const err = await resp.json();
        setHireError(err.error || "Failed to hire agent");
        return;
      }

      const run = await resp.json();
      setHireOpen(false);
      router.push(`/chat?session=${run.id}`);
    } catch {
      setHireError("Failed to hire agent. Is the API running?");
    } finally {
      setHiring(false);
    }
  }

  if (loading) {
    return <div className="text-center py-12 text-muted-foreground">Loading...</div>;
  }

  if (!agent) {
    return <div className="text-center py-12 text-muted-foreground">Agent not found</div>;
  }

  const m = manifest || {};
  const exp = m.experience;
  const persona = m.persona;
  const adapters = m.adapters;
  const pricing = m.marketplace?.pricing ?? {
    model: agent.manifest?.pricing_model,
    base: agent.manifest?.price_per_unit
      ? `${agent.manifest.currency || "USD"} ${agent.manifest.price_per_unit}/${agent.manifest.pricing_model?.replace("per_", "")}`
      : undefined,
  };
  const identity = agent.identity || {};
  const soul = agent.soul;
  const tags = m.marketplace?.tags || agent.manifest?.tags || [];

  const pricingLabel =
    pricing?.model === "free"
      ? "Free"
      : pricing?.base || pricing?.model || agent.manifest?.pricing_model || "Free";

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-4">
          {m.emoji && <span className="text-5xl">{m.emoji}</span>}
          <div>
            <h1 className="text-3xl font-bold">{identity.name || m.name}</h1>
            <p className="text-muted-foreground">
              v{agent.version} · by {agent.manifest?.author || m.author || "unknown"}
            </p>
            {exp?.level && (
              <span className="text-sm font-medium">
                {LEVEL_LABELS[exp.level] || exp.level} · {exp.packs || 0} experience packs
              </span>
            )}
          </div>
        </div>
        <div className="flex flex-col items-end gap-2">
          <span className="text-lg font-bold">{pricingLabel}</span>
          <Button onClick={() => setHireOpen(true)} size="lg">
            🚀 Hire This Agent
          </Button>
          {pricing?.trial && (
            <span className="text-xs text-muted-foreground">{pricing.trial} free trial runs</span>
          )}
        </div>
      </div>

      {/* Description */}
      <Card>
        <CardContent className="pt-6">
          <p className="text-lg">{identity.description || m.description}</p>
        </CardContent>
      </Card>

      {/* Tags */}
      {tags.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {tags.map((t: string) => (
            <Badge key={t} variant="secondary">{t}</Badge>
          ))}
        </div>
      )}

      {/* DNA Viewer */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Agent DNA</CardTitle>
            <div className="flex gap-1">
              {(["identity", "soul", "tools"] as const).map((tab) => (
                <button
                  key={tab}
                  onClick={() => setDnaTab(tab)}
                  className={`px-3 py-1 text-xs rounded-md transition-colors ${
                    dnaTab === tab
                      ? "bg-primary text-primary-foreground"
                      : "text-muted-foreground hover:bg-accent"
                  }`}
                >
                  {tab === "identity" ? "AGENT.md" : tab === "soul" ? "SOUL.md" : "TOOLS"}
                </button>
              ))}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {dnaTab === "identity" && (
            <div className="space-y-3 text-sm">
              {identity.role && (
                <div>
                  <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Role</span>
                  <p className="mt-1">{identity.role}</p>
                </div>
              )}
              {identity.capabilities && identity.capabilities.length > 0 && (
                <div>
                  <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Capabilities</span>
                  <ul className="mt-1 space-y-1">
                    {identity.capabilities.map((c: string, i: number) => (
                      <li key={i} className="flex items-start gap-2">
                        <span className="text-muted-foreground mt-0.5">•</span>
                        <span>{c}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {identity.constraints && identity.constraints.length > 0 && (
                <div>
                  <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Constraints</span>
                  <ul className="mt-1 space-y-1">
                    {identity.constraints.map((c: string, i: number) => (
                      <li key={i} className="flex items-start gap-2 text-muted-foreground">
                        <span className="mt-0.5">•</span>
                        <span>{c}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {identity.workflow && identity.workflow.length > 0 && (
                <div>
                  <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Workflow</span>
                  <ol className="mt-1 space-y-1">
                    {identity.workflow.map((step: string, i: number) => (
                      <li key={i} className="flex items-start gap-2">
                        <span className="text-muted-foreground font-mono text-xs mt-0.5">{i + 1}.</span>
                        <span>{step}</span>
                      </li>
                    ))}
                  </ol>
                </div>
              )}
              {identity.guidelines && identity.guidelines.length > 0 && (
                <div>
                  <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Guidelines</span>
                  <ul className="mt-1 space-y-1">
                    {identity.guidelines.map((g: string, i: number) => (
                      <li key={i} className="flex items-start gap-2">
                        <span className="text-muted-foreground mt-0.5">→</span>
                        <span>{g}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {!identity.role && !identity.capabilities?.length && (
                <p className="text-muted-foreground italic">No identity details available.</p>
              )}
            </div>
          )}

          {dnaTab === "soul" && (
            <div className="space-y-3 text-sm">
              {soul ? (
                <>
                  {soul.personality && (
                    <div>
                      <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Personality</span>
                      <p className="mt-1">{soul.personality}</p>
                    </div>
                  )}
                  {soul.voice && (
                    <div>
                      <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Voice</span>
                      <p className="mt-1">{soul.voice}</p>
                    </div>
                  )}
                  {soul.tone && (
                    <div>
                      <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Tone</span>
                      <p className="mt-1">{soul.tone}</p>
                    </div>
                  )}
                  {soul.values && soul.values.length > 0 && (
                    <div>
                      <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Values</span>
                      <ul className="mt-1 space-y-1">
                        {soul.values.map((v: string, i: number) => (
                          <li key={i} className="flex items-start gap-2">
                            <span className="text-muted-foreground mt-0.5">❤</span>
                            <span>{v}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                  {soul.communication_style && (
                    <div>
                      <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Communication Style</span>
                      <p className="mt-1">{soul.communication_style}</p>
                    </div>
                  )}
                  {/* Enriched persona from manifest */}
                  {persona && (
                    <div>
                      <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Principles</span>
                      <ul className="mt-1 space-y-1">
                        {persona.principles?.map((p: string, i: number) => (
                          <li key={i} className="flex items-start gap-2 text-muted-foreground">
                            <span className="mt-0.5">•</span>
                            <span>{p}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </>
              ) : (
                <p className="text-muted-foreground italic">No soul data available.</p>
              )}
            </div>
          )}

          {dnaTab === "tools" && (
            <div className="space-y-4 text-sm">
              {adapters ? (
                <>
                  {adapters.frameworks && adapters.frameworks.length > 0 && (
                    <div>
                      <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Frameworks</span>
                      <div className="flex gap-2 mt-2 flex-wrap">
                        {adapters.frameworks.map((fw: any) => (
                          <Badge key={fw.name} variant={fw.native ? "default" : "outline"}>
                            {fw.name} {fw.version || ""} {fw.native ? "(native)" : ""}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}
                  {adapters.tools && (
                    <div className="grid grid-cols-3 gap-4">
                      {adapters.tools.required && adapters.tools.required.length > 0 && (
                        <div>
                          <span className="font-medium text-red-600 text-xs uppercase">Required</span>
                          <ul className="mt-1 space-y-1">
                            {adapters.tools.required.map((t: any) => (
                              <li key={t.name} className="text-muted-foreground">
                                {t.name}
                                {t.reason && <span className="text-xs block opacity-70">— {t.reason}</span>}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {adapters.tools.recommended && adapters.tools.recommended.length > 0 && (
                        <div>
                          <span className="font-medium text-yellow-600 text-xs uppercase">Recommended</span>
                          <ul className="mt-1 space-y-1">
                            {adapters.tools.recommended.map((t: any) => (
                              <li key={t.name} className="text-muted-foreground">{t.name}</li>
                            ))}
                          </ul>
                        </div>
                      )}
                      {adapters.tools.optional && adapters.tools.optional.length > 0 && (
                        <div>
                          <span className="font-medium text-gray-500 text-xs uppercase">Optional</span>
                          <ul className="mt-1 space-y-1">
                            {adapters.tools.optional.map((t: any) => (
                              <li key={t.name} className="text-muted-foreground">{t.name}</li>
                            ))}
                          </ul>
                        </div>
                      )}
                    </div>
                  )}
                  {adapters.agent_apps && adapters.agent_apps.length > 0 && (
                    <div>
                      <span className="font-medium text-muted-foreground text-xs uppercase tracking-wide">Agent Apps</span>
                      <div className="flex gap-2 mt-2 flex-wrap">
                        {adapters.agent_apps.map((app: any) => (
                          <Badge key={app.name} variant="outline">
                            {app.name} ({app.role})
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}
                </>
              ) : (
                <p className="text-muted-foreground italic">No tool information available.</p>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Experience */}
      {exp && exp.highlights && exp.highlights.length > 0 && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>
                Experience
                {exp.domains && (
                  <span className="text-sm font-normal text-muted-foreground ml-2">
                    {exp.domains.join(" · ")}
                  </span>
                )}
              </CardTitle>
              <Button
                variant="outline"
                size="sm"
                onClick={() => router.push(`/agents/${params.id}/experience`)}
              >
                Manage Experience
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {exp.highlights.map((h: any) => (
                <div key={h.id} className="flex items-start justify-between p-3 rounded-lg bg-muted/50">
                  <div>
                    <p className="font-medium text-sm">{h.summary}</p>
                    <p className="text-xs text-muted-foreground mt-1">{h.id}</p>
                  </div>
                  {h.difficulty && (
                    <Badge className={`text-xs ${DIFFICULTY_COLORS[h.difficulty] || ""}`}>
                      {h.difficulty}
                    </Badge>
                  )}
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Run history */}
      {runs.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Run History</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {runs.map((run: any) => (
              <div
                key={run.id}
                className="flex items-center justify-between p-3 rounded-lg border border-border hover:bg-accent/30 cursor-pointer transition-colors"
                onClick={() => router.push(`/runs/${run.id}`)}
              >
                <div>
                  <p className="text-sm font-mono">{run.id.slice(0, 8)}</p>
                  <p className="text-xs text-muted-foreground">{formatTime(run.created_at)}</p>
                </div>
                <Badge className={`text-xs ${RUN_STATUS_COLORS[run.status] || ""}`}>{run.status}</Badge>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Model & Stats */}
      <div className="grid grid-cols-2 gap-4">
        {m.model && (
          <Card>
            <CardHeader>
              <CardTitle>Model Requirements</CardTitle>
            </CardHeader>
            <CardContent className="text-sm space-y-2">
              {m.model.minimum && <div><span className="font-medium">Minimum:</span> {m.model.minimum}</div>}
              {m.model.recommended && <div><span className="font-medium">Recommended:</span> {m.model.recommended}</div>}
              {m.model.context_window && <div><span className="font-medium">Context:</span> {m.model.context_window}</div>}
            </CardContent>
          </Card>
        )}

        <Card>
          <CardHeader>
            <CardTitle>Stats</CardTitle>
          </CardHeader>
          <CardContent className="text-sm space-y-2">
            <div><span className="font-medium">Downloads:</span> {agent.downloads || 0}</div>
            <div><span className="font-medium">Rating:</span> {agent.rating ? `⭐ ${agent.rating.toFixed(1)}` : "No ratings yet"}</div>
            <div><span className="font-medium">Runtime:</span> {agent.manifest?.runtime || "—"}</div>
            <div><span className="font-medium">Status:</span> <Badge variant="outline">{agent.status}</Badge></div>
            {agent.repo_url && (
              <div className="truncate">
                <span className="font-medium">Repo:</span>{" "}
                <span className="font-mono text-xs text-muted-foreground">{agent.repo_url}</span>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Hire Dialog */}
      <Dialog open={hireOpen} onOpenChange={setHireOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Hire {identity.name || m.name}</DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-2">
            {/* Pricing summary */}
            <div className="rounded-lg bg-muted/50 p-4 text-sm space-y-1">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Pricing</span>
                <span className="font-medium">{pricingLabel}</span>
              </div>
              {agent.manifest?.runtime && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Runtime</span>
                  <span className="font-mono">{agent.manifest.runtime}</span>
                </div>
              )}
              {pricing?.trial && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Trial runs</span>
                  <span className="text-blue-600">{pricing.trial} free</span>
                </div>
              )}
            </div>

            {/* Task description */}
            <div className="space-y-2">
              <Label htmlFor="hire-message">Task Description (optional)</Label>
              <Textarea
                id="hire-message"
                placeholder="Describe what you'd like this agent to do..."
                value={hireMessage}
                onChange={(e) => setHireMessage(e.target.value)}
                className="min-h-[100px]"
              />
              <p className="text-xs text-muted-foreground">
                You can also send the first message in the chat after hiring.
              </p>
            </div>

            {hireError && (
              <p className="text-sm text-red-500">{hireError}</p>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setHireOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleHire} disabled={hiring}>
              {hiring ? "Starting..." : "🚀 Hire & Launch"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
