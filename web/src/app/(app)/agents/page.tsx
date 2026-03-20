"use client";

import { useState, useEffect, useMemo } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { clientFetch } from "@/lib/api";

const OPENAGENT_REGISTRY_URL =
  "https://raw.githubusercontent.com/openagent-spec/registry/main/index.json";

interface RegistryEntry {
  id: string;
  name: string;
  emoji: string;
  description: string;
  category: string;
  url: string;
}

const LEVEL_BADGES: Record<string, { label: string; color: string }> = {
  junior: { label: "🌱 Junior", color: "bg-green-100 text-green-800" },
  mid: { label: "🌿 Mid", color: "bg-emerald-100 text-emerald-800" },
  senior: { label: "🌳 Senior", color: "bg-blue-100 text-blue-800" },
  expert: { label: "⭐ Expert", color: "bg-purple-100 text-purple-800" },
};

type TabType = "my" | "registry";

export default function AgentsPage() {
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<TabType>("my");

  // My Agents state
  const [agents, setAgents] = useState<any[]>([]);
  const [search, setSearch] = useState("");
  const [framework, setFramework] = useState("");
  const [level, setLevel] = useState("");
  const [runtime, setRuntime] = useState("");
  const [tag, setTag] = useState("");
  const [loading, setLoading] = useState(true);

  // Registry state
  const [registryAgents, setRegistryAgents] = useState<RegistryEntry[]>([]);
  const [registrySearch, setRegistrySearch] = useState("");
  const [registryLoading, setRegistryLoading] = useState(false);
  const [registryError, setRegistryError] = useState("");

  useEffect(() => {
    loadAgents();
  }, []);

  useEffect(() => {
    if (activeTab === "registry" && registryAgents.length === 0 && !registryLoading) {
      loadRegistryAgents();
    }
  }, [activeTab]);

  async function loadAgents() {
    try {
      const params = new URLSearchParams();
      if (search) params.set("q", search);
      if (framework) params.set("framework", framework);
      if (level) params.set("level", level);
      if (runtime) params.set("runtime", runtime);
      if (tag) params.set("tag", tag);

      const resp = await clientFetch(`/api/v1/registry/agents/search?${params}`);
      const data = await resp.json();
      setAgents(data || []);
    } catch (e) {
      console.error("Failed to load agents:", e);
    } finally {
      setLoading(false);
    }
  }

  async function loadRegistryAgents() {
    setRegistryLoading(true);
    setRegistryError("");
    try {
      const resp = await fetch(OPENAGENT_REGISTRY_URL);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data: RegistryEntry[] = await resp.json();
      setRegistryAgents(data || []);
    } catch (e) {
      console.error("Failed to load registry agents:", e);
      setRegistryError("Failed to load OpenAgent Registry. Please try again later.");
    } finally {
      setRegistryLoading(false);
    }
  }

  useEffect(() => {
    const timer = setTimeout(() => {
      setLoading(true);
      loadAgents();
    }, 300);
    return () => clearTimeout(timer);
  }, [search, framework, level, runtime, tag]);

  const frameworks = useMemo(() => {
    const fws = new Set<string>();
    agents.forEach((a) => {
      if (a.manifest?.adapters?.frameworks) {
        a.manifest.adapters.frameworks.forEach((fw: any) => fws.add(fw.name));
      }
    });
    return Array.from(fws);
  }, [agents]);

  const runtimes = useMemo(() => {
    const rts = new Set<string>();
    agents.forEach((a) => {
      if (a.manifest?.runtime) rts.add(a.manifest.runtime);
    });
    return Array.from(rts);
  }, [agents]);

  const allTags = useMemo(() => {
    const ts = new Set<string>();
    agents.forEach((a) => {
      const tags = a.manifest?.marketplace?.tags || a.manifest?.tags || [];
      tags.forEach((t: string) => ts.add(t));
    });
    return Array.from(ts).slice(0, 12);
  }, [agents]);

  const filteredRegistryAgents = useMemo(() => {
    if (!registrySearch.trim()) return registryAgents;
    const q = registrySearch.toLowerCase();
    return registryAgents.filter(
      (a) =>
        a.name.toLowerCase().includes(q) ||
        a.description.toLowerCase().includes(q) ||
        a.category.toLowerCase().includes(q) ||
        a.id.toLowerCase().includes(q)
    );
  }, [registryAgents, registrySearch]);

  const registryCategories = useMemo(() => {
    const cats = new Set<string>();
    registryAgents.forEach((a) => {
      if (a.category) cats.add(a.category);
    });
    return Array.from(cats);
  }, [registryAgents]);

  const hasFilters = search || framework || level || runtime || tag;

  function clearFilters() {
    setSearch("");
    setFramework("");
    setLevel("");
    setRuntime("");
    setTag("");
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Agent Marketplace</h1>
          <p className="text-muted-foreground mt-1">
            Hire AI agents with real experience. The more they work, the better they get.
          </p>
        </div>
        <Link href="/agents/my">
          <Button variant="outline">My Agents &rarr;</Button>
        </Link>
      </div>

      {/* Tab Toggle */}
      <div className="flex gap-1 bg-muted/50 p-1 rounded-lg w-fit">
        <button
          onClick={() => setActiveTab("my")}
          className={`px-3 py-1 text-sm rounded-md transition-colors ${
            activeTab === "my"
              ? "bg-primary text-primary-foreground"
              : "text-muted-foreground hover:bg-accent"
          }`}
        >
          My Agents
        </button>
        <button
          onClick={() => setActiveTab("registry")}
          className={`px-3 py-1 text-sm rounded-md transition-colors ${
            activeTab === "registry"
              ? "bg-primary text-primary-foreground"
              : "text-muted-foreground hover:bg-accent"
          }`}
        >
          OpenAgent Registry
        </button>
      </div>

      {/* My Agents Tab */}
      {activeTab === "my" && (
        <>
          {/* Filters */}
          <div className="space-y-3">
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
                className="border border-border rounded-md px-3 py-2 text-sm bg-background"
              >
                <option value="">All Frameworks</option>
                {frameworks.map((fw) => (
                  <option key={fw} value={fw}>{fw}</option>
                ))}
              </select>
              <select
                value={level}
                onChange={(e) => setLevel(e.target.value)}
                className="border border-border rounded-md px-3 py-2 text-sm bg-background"
              >
                <option value="">All Levels</option>
                <option value="junior">🌱 Junior</option>
                <option value="mid">🌿 Mid</option>
                <option value="senior">🌳 Senior</option>
                <option value="expert">⭐ Expert</option>
              </select>
              <select
                value={runtime}
                onChange={(e) => setRuntime(e.target.value)}
                className="border border-border rounded-md px-3 py-2 text-sm bg-background"
              >
                <option value="">All Runtimes</option>
                {runtimes.map((rt) => (
                  <option key={rt} value={rt}>{rt}</option>
                ))}
              </select>
              {hasFilters && (
                <button
                  onClick={clearFilters}
                  className="text-sm text-muted-foreground hover:text-foreground transition-colors px-2"
                >
                  Clear filters &times;
                </button>
              )}
            </div>

            {/* Tag pills */}
            {allTags.length > 0 && (
              <div className="flex gap-2 flex-wrap">
                {allTags.map((t) => (
                  <button
                    key={t}
                    onClick={() => setTag(tag === t ? "" : t)}
                    className={`text-xs px-3 py-1 rounded-full border transition-colors ${
                      tag === t
                        ? "bg-primary text-primary-foreground border-primary"
                        : "border-border text-muted-foreground hover:text-foreground hover:border-foreground"
                    }`}
                  >
                    {t}
                  </button>
                ))}
              </div>
            )}
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
        </>
      )}

      {/* OpenAgent Registry Tab */}
      {activeTab === "registry" && (
        <>
          <div className="flex gap-3 flex-wrap">
            <Input
              placeholder="Search registry agents..."
              value={registrySearch}
              onChange={(e) => setRegistrySearch(e.target.value)}
              className="max-w-xs"
            />
            {registryCategories.length > 0 && (
              <div className="flex gap-2 flex-wrap items-center">
                {registryCategories.map((cat) => (
                  <button
                    key={cat}
                    onClick={() =>
                      setRegistrySearch(registrySearch === cat ? "" : cat)
                    }
                    className={`text-xs px-3 py-1 rounded-full border transition-colors ${
                      registrySearch === cat
                        ? "bg-primary text-primary-foreground border-primary"
                        : "border-border text-muted-foreground hover:text-foreground hover:border-foreground"
                    }`}
                  >
                    {cat}
                  </button>
                ))}
              </div>
            )}
          </div>

          {registryLoading ? (
            <div className="text-center py-12 text-muted-foreground">
              Loading OpenAgent Registry...
            </div>
          ) : registryError ? (
            <div className="text-center py-12 space-y-3">
              <p className="text-muted-foreground">{registryError}</p>
              <Button variant="outline" onClick={loadRegistryAgents}>
                Retry
              </Button>
            </div>
          ) : filteredRegistryAgents.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              {registrySearch
                ? "No registry agents match your search."
                : "No agents found in the OpenAgent Registry."}
            </div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {filteredRegistryAgents.map((entry) => (
                <RegistryAgentCard
                  key={entry.id}
                  entry={entry}
                  onClick={() => {
                    const q = new URLSearchParams({
                      name: entry.name,
                      emoji: entry.emoji,
                      description: entry.description,
                      category: entry.category,
                      url: entry.url,
                    });
                    router.push(`/agents/registry/${entry.id}?${q.toString()}`);
                  }}
                />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}

/* ---------- My Agents Card (existing) ---------- */

function AgentCard({ agent, onClick }: { agent: any; onClick: () => void }) {
  const manifest = agent.manifest;
  const exp = manifest?.experience;
  const levelBadge = exp?.level ? LEVEL_BADGES[exp.level] : null;
  const pricing = manifest?.marketplace?.pricing;
  const tags = manifest?.marketplace?.tags || manifest?.tags || [];

  const pricingLabel =
    pricing?.model === "free"
      ? "Free"
      : pricing?.base || pricing?.model || "Free";

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
        {tags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {tags.slice(0, 4).map((t: string) => (
              <Badge key={t} variant="secondary" className="text-xs">{t}</Badge>
            ))}
          </div>
        )}

        {/* Footer: stats + pricing + runtime */}
        <div className="flex items-center justify-between text-xs text-muted-foreground pt-2 border-t">
          <div className="flex gap-3">
            {exp?.packs != null && <span>{exp.packs} exp packs</span>}
            {agent.downloads > 0 && <span>{agent.downloads} installs</span>}
            {agent.rating > 0 && <span>⭐ {agent.rating.toFixed(1)}</span>}
          </div>
          <div className="flex items-center gap-2">
            {manifest?.runtime && (
              <span className="font-mono opacity-70">{manifest.runtime}</span>
            )}
            <span className="font-medium">{pricingLabel}</span>
          </div>
        </div>

        {/* Frameworks */}
        {manifest?.adapters?.frameworks && manifest.adapters.frameworks.length > 0 && (
          <div className="flex gap-1 flex-wrap">
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

/* ---------- Registry Agent Card ---------- */

function RegistryAgentCard({
  entry,
  onClick,
}: {
  entry: RegistryEntry;
  onClick: () => void;
}) {
  return (
    <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={onClick}>
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2">
            {entry.emoji && <span className="text-2xl">{entry.emoji}</span>}
            <div>
              <CardTitle className="text-lg">{entry.name}</CardTitle>
              <p className="text-xs text-muted-foreground">{entry.id}</p>
            </div>
          </div>
          <Badge variant="secondary" className="text-xs capitalize">
            {entry.category}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm text-muted-foreground line-clamp-3">
          {entry.description}
        </p>
        <div className="flex items-center justify-between text-xs text-muted-foreground pt-2 border-t">
          <span className="capitalize">{entry.category}</span>
          <span className="font-medium text-blue-600">OpenAgent Registry</span>
        </div>
      </CardContent>
    </Card>
  );
}
