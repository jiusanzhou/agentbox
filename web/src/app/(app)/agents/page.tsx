"use client";

import { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { clientFetch } from "@/lib/api";
import type { AgentDNA } from "@/lib/types";

const LEVEL_BADGES: Record<string, { label: string; color: string }> = {
  junior: { label: "🌱 Junior", color: "bg-green-100 text-green-800" },
  mid: { label: "🌿 Mid", color: "bg-emerald-100 text-emerald-800" },
  senior: { label: "🌳 Senior", color: "bg-blue-100 text-blue-800" },
  expert: { label: "⭐ Expert", color: "bg-purple-100 text-purple-800" },
};

export default function AgentsPage() {
  const router = useRouter();
  const [agents, setAgents] = useState<any[]>([]);
  const [search, setSearch] = useState("");
  const [framework, setFramework] = useState("");
  const [level, setLevel] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadAgents();
  }, []);

  async function loadAgents() {
    try {
      const params = new URLSearchParams();
      if (search) params.set("q", search);
      if (framework) params.set("framework", framework);
      if (level) params.set("level", level);

      const resp = await clientFetch(`/api/v1/registry/agents/search?${params}`);
      const data = await resp.json();
      setAgents(data || []);
    } catch (e) {
      console.error("Failed to load agents:", e);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    const timer = setTimeout(loadAgents, 300);
    return () => clearTimeout(timer);
  }, [search, framework, level]);

  const frameworks = useMemo(() => {
    const fws = new Set<string>();
    agents.forEach((a) => {
      if (a.manifest?.adapters?.frameworks) {
        a.manifest.adapters.frameworks.forEach((fw: any) => fws.add(fw.name));
      }
    });
    return Array.from(fws);
  }, [agents]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Agent Marketplace</h1>
        <p className="text-muted-foreground mt-1">
          Hire AI agents with real experience. The more they work, the better they get.
        </p>
      </div>

      {/* Filters */}
      <div className="flex gap-3 flex-wrap">
        <Input
          placeholder="Search agents..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-xs"
        />
        <select
          value={framework}
          onChange={(e) => setFramework(e.target.value)}
          className="border rounded-md px-3 py-2 text-sm"
        >
          <option value="">All Frameworks</option>
          {frameworks.map((fw) => (
            <option key={fw} value={fw}>{fw}</option>
          ))}
        </select>
        <select
          value={level}
          onChange={(e) => setLevel(e.target.value)}
          className="border rounded-md px-3 py-2 text-sm"
        >
          <option value="">All Levels</option>
          <option value="junior">🌱 Junior</option>
          <option value="mid">🌿 Mid</option>
          <option value="senior">🌳 Senior</option>
          <option value="expert">⭐ Expert</option>
        </select>
      </div>

      {/* Agent Grid */}
      {loading ? (
        <div className="text-center py-12 text-muted-foreground">Loading agents...</div>
      ) : agents.length === 0 ? (
        <div className="text-center py-12 text-muted-foreground">
          No agents found. Be the first to publish one!
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {agents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              onClick={() => router.push(`/agents/${agent.slug || agent.id}`)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function AgentCard({ agent, onClick }: { agent: any; onClick: () => void }) {
  const manifest = agent.manifest;
  const exp = manifest?.experience;
  const levelBadge = exp?.level ? LEVEL_BADGES[exp.level] : null;
  const pricing = manifest?.marketplace?.pricing;

  return (
    <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={onClick}>
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2">
            {manifest?.emoji && <span className="text-2xl">{manifest.emoji}</span>}
            <div>
              <CardTitle className="text-lg">{agent.name}</CardTitle>
              <p className="text-xs text-muted-foreground">
                v{agent.version} · by {manifest?.author || "unknown"}
              </p>
            </div>
          </div>
          {levelBadge && (
            <span className={`text-xs px-2 py-1 rounded-full ${levelBadge.color}`}>
              {levelBadge.label}
            </span>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm text-muted-foreground line-clamp-2">
          {agent.description || manifest?.description}
        </p>

        {/* Experience domains */}
        {exp?.domains && exp.domains.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {exp.domains.slice(0, 3).map((d: string) => (
              <Badge key={d} variant="outline" className="text-xs">
                {d}
              </Badge>
            ))}
            {exp.domains.length > 3 && (
              <Badge variant="outline" className="text-xs">+{exp.domains.length - 3}</Badge>
            )}
          </div>
        )}

        {/* Tags */}
        {manifest?.marketplace?.tags && (
          <div className="flex flex-wrap gap-1">
            {manifest.marketplace.tags.slice(0, 4).map((t: string) => (
              <Badge key={t} variant="secondary" className="text-xs">{t}</Badge>
            ))}
          </div>
        )}

        {/* Footer: stats + pricing */}
        <div className="flex items-center justify-between text-xs text-muted-foreground pt-2 border-t">
          <div className="flex gap-3">
            {exp?.packs != null && <span>{exp.packs} exp packs</span>}
            {agent.downloads > 0 && <span>{agent.downloads} installs</span>}
            {agent.rating > 0 && <span>⭐ {agent.rating.toFixed(1)}</span>}
          </div>
          {pricing && (
            <span className="font-medium">
              {pricing.model === "free" ? "Free" : pricing.base || pricing.model}
            </span>
          )}
        </div>

        {/* Frameworks */}
        {manifest?.adapters?.frameworks && (
          <div className="flex gap-1">
            {manifest.adapters.frameworks.map((fw: any) => (
              <Badge key={fw.name} variant="outline" className="text-[10px]">
                {fw.name}
              </Badge>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
