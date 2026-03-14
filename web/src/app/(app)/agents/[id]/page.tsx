"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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

export default function AgentDetailPage() {
  const params = useParams();
  const router = useRouter();
  const [agent, setAgent] = useState<any>(null);
  const [manifest, setManifest] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [hiring, setHiring] = useState(false);

  useEffect(() => {
    loadAgent();
  }, [params.id]);

  async function loadAgent() {
    try {
      const resp = await clientFetch(`/api/v1/registry/agents/${params.id}`);
      const data = await resp.json();
      setAgent(data);

      // Also fetch manifest format
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

  async function handleHire() {
    setHiring(true);
    try {
      const resp = await clientFetch(`/api/v1/registry/agents/${params.id}/hire`, {
        method: "POST",
        body: JSON.stringify({ message: "" }),
      });
      const run = await resp.json();
      router.push(`/chat?session=${run.id}`);
    } catch (e) {
      console.error("Failed to hire agent:", e);
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
  const pricing = m.marketplace?.pricing;
  const identity = agent.identity || {};

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
          {pricing && (
            <span className="text-lg font-bold">
              {pricing.model === "free" ? "Free" : pricing.base || pricing.model}
            </span>
          )}
          <Button onClick={handleHire} disabled={hiring} size="lg">
            {hiring ? "Starting..." : "🚀 Hire This Agent"}
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

      {/* Persona */}
      {persona && (
        <Card>
          <CardHeader>
            <CardTitle>Persona</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span className="font-medium">Style:</span> {persona.style}
              </div>
              <div>
                <span className="font-medium">Tone:</span> {persona.tone}
              </div>
              {persona.language && (
                <div>
                  <span className="font-medium">Languages:</span> {persona.language.join(", ")}
                </div>
              )}
            </div>
            {persona.principles && persona.principles.length > 0 && (
              <div>
                <span className="font-medium text-sm">Principles:</span>
                <ul className="list-disc list-inside mt-1 text-sm text-muted-foreground">
                  {persona.principles.map((p: string, i: number) => (
                    <li key={i}>{p}</li>
                  ))}
                </ul>
              </div>
            )}
          </CardContent>
        </Card>
      )}

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

      {/* Tool Adapters */}
      {adapters && (
        <Card>
          <CardHeader>
            <CardTitle>Tool Compatibility</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Frameworks */}
            {adapters.frameworks && adapters.frameworks.length > 0 && (
              <div>
                <span className="font-medium text-sm">Frameworks:</span>
                <div className="flex gap-2 mt-1">
                  {adapters.frameworks.map((fw: any) => (
                    <Badge key={fw.name} variant={fw.native ? "default" : "outline"}>
                      {fw.name} {fw.version || ""} {fw.native ? "(native)" : ""}
                    </Badge>
                  ))}
                </div>
              </div>
            )}

            {/* Tools */}
            {adapters.tools && (
              <div className="grid grid-cols-3 gap-4 text-sm">
                {adapters.tools.required && (
                  <div>
                    <span className="font-medium text-red-600">Required:</span>
                    <ul className="mt-1 space-y-1">
                      {adapters.tools.required.map((t: any) => (
                        <li key={t.name} className="text-muted-foreground">
                          {t.name} {t.reason && <span className="text-xs">— {t.reason}</span>}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
                {adapters.tools.recommended && (
                  <div>
                    <span className="font-medium text-yellow-600">Recommended:</span>
                    <ul className="mt-1 space-y-1">
                      {adapters.tools.recommended.map((t: any) => (
                        <li key={t.name} className="text-muted-foreground">
                          {t.name}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
                {adapters.tools.optional && (
                  <div>
                    <span className="font-medium text-gray-500">Optional:</span>
                    <ul className="mt-1 space-y-1">
                      {adapters.tools.optional.map((t: any) => (
                        <li key={t.name} className="text-muted-foreground">{t.name}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            )}

            {/* Agent Apps */}
            {adapters.agent_apps && adapters.agent_apps.length > 0 && (
              <div>
                <span className="font-medium text-sm">Agent Apps:</span>
                <div className="flex gap-2 mt-1">
                  {adapters.agent_apps.map((app: any) => (
                    <Badge key={app.name} variant="outline">
                      {app.name} ({app.role})
                      {app.alternatives && ` / ${app.alternatives.join(", ")}`}
                    </Badge>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Model & Pricing */}
      <div className="grid grid-cols-2 gap-4">
        {m.model && (
          <Card>
            <CardHeader>
              <CardTitle>Model Requirements (Salary)</CardTitle>
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
            <div><span className="font-medium">Status:</span> <Badge variant="outline">{agent.status}</Badge></div>
          </CardContent>
        </Card>
      </div>

      {/* Tags */}
      {m.marketplace?.tags && m.marketplace.tags.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {m.marketplace.tags.map((t: string) => (
            <Badge key={t} variant="secondary">{t}</Badge>
          ))}
        </div>
      )}
    </div>
  );
}
