"use client";

import { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
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
  const [registryCategory, setRegistryCategory] = useState("");

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

  const allTags = useMemo(() => {
    const ts = new Set<string>();
    agents.forEach((a) => {
      const tags = a.manifest?.marketplace?.tags || a.manifest?.tags || [];
      tags.forEach((t: string) => ts.add(t));
    });
    return Array.from(ts).slice(0, 12);
  }, [agents]);

  const filteredRegistryAgents = useMemo(() => {
    let filtered = registryAgents;
    if (registryCategory) {
      filtered = filtered.filter((a) => a.category === registryCategory);
    }
    if (registrySearch.trim()) {
      const q = registrySearch.toLowerCase();
      filtered = filtered.filter(
        (a) =>
          a.name.toLowerCase().includes(q) ||
          a.description.toLowerCase().includes(q) ||
          a.category.toLowerCase().includes(q) ||
          a.id.toLowerCase().includes(q)
      );
    }
    return filtered;
  }, [registryAgents, registrySearch, registryCategory]);

  const registryCategories = useMemo(() => {
    const cats = new Set<string>();
    registryAgents.forEach((a) => {
      if (a.category) cats.add(a.category);
    });
    return Array.from(cats);
  }, [registryAgents]);

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="animate-fade-in">
        <h1 className="text-2xl font-bold tracking-tight">Agents</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Browse, install, and manage AI agents
        </p>
      </div>

      {/* Tabs */}
      <div className="animate-fade-in animate-delay-100 flex items-center gap-6 border-b border-border">
        <button
          onClick={() => setActiveTab("my")}
          className={`relative pb-3 text-sm font-medium transition-colors ${
            activeTab === "my"
              ? "text-foreground"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          Registry
          {activeTab === "my" && (
            <span className="absolute bottom-0 left-0 right-0 h-px bg-foreground" />
          )}
        </button>
        <button
          onClick={() => setActiveTab("registry")}
          className={`relative pb-3 text-sm font-medium transition-colors ${
            activeTab === "registry"
              ? "text-foreground"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          My Agents
          {activeTab === "registry" && (
            <span className="absolute bottom-0 left-0 right-0 h-px bg-foreground" />
          )}
        </button>
      </div>

      {/* My Agents Tab */}
      {activeTab === "my" && (
        <div className="animate-fade-in space-y-6">
          {/* Search + Filters */}
          <div className="flex items-center gap-3 flex-wrap">
            <div className="relative">
              <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search agents..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9 w-64"
              />
            </div>
          </div>

          {/* Category pills */}
          {allTags.length > 0 && (
            <div className="flex gap-2 flex-wrap">
              <button
                onClick={() => setTag("")}
                className={`text-xs px-3 py-1.5 rounded-full transition-all duration-150 ${
                  !tag
                    ? "bg-foreground text-background"
                    : "border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30"
                }`}
              >
                All
              </button>
              {allTags.map((t) => (
                <button
                  key={t}
                  onClick={() => setTag(tag === t ? "" : t)}
                  className={`text-xs px-3 py-1.5 rounded-full transition-all duration-150 ${
                    tag === t
                      ? "bg-foreground text-background"
                      : "border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30"
                  }`}
                >
                  {t}
                </button>
              ))}
            </div>
          )}

          {/* Agent Grid */}
          {loading ? (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="rounded-xl border border-border p-6">
                  <div className="flex items-center gap-3 mb-3">
                    <div className="h-10 w-10 rounded-lg bg-muted animate-pulse" />
                    <div className="space-y-1.5 flex-1">
                      <div className="h-4 w-24 bg-muted animate-pulse rounded" />
                      <div className="h-3 w-32 bg-muted animate-pulse rounded" />
                    </div>
                  </div>
                  <div className="h-3 w-full bg-muted animate-pulse rounded mt-3" />
                  <div className="h-3 w-2/3 bg-muted animate-pulse rounded mt-2" />
                </div>
              ))}
            </div>
          ) : agents.length === 0 ? (
            <div className="text-center py-20">
              <p className="text-muted-foreground">No agents found</p>
              <p className="text-sm text-muted-foreground/60 mt-1">
                Be the first to publish one
              </p>
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
      )}

      {/* OpenAgent Registry Tab */}
      {activeTab === "registry" && (
        <div className="animate-fade-in space-y-6">
          <div className="flex items-center gap-3 flex-wrap">
            <div className="relative">
              <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search registry..."
                value={registrySearch}
                onChange={(e) => setRegistrySearch(e.target.value)}
                className="pl-9 w-64"
              />
            </div>
          </div>

          {registryCategories.length > 0 && (
            <div className="flex gap-2 flex-wrap">
              <button
                onClick={() => setRegistryCategory("")}
                className={`text-xs px-3 py-1.5 rounded-full transition-all duration-150 ${
                  !registryCategory
                    ? "bg-foreground text-background"
                    : "border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30"
                }`}
              >
                All
              </button>
              {registryCategories.map((cat) => (
                <button
                  key={cat}
                  onClick={() =>
                    setRegistryCategory(registryCategory === cat ? "" : cat)
                  }
                  className={`text-xs px-3 py-1.5 rounded-full capitalize transition-all duration-150 ${
                    registryCategory === cat
                      ? "bg-foreground text-background"
                      : "border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30"
                  }`}
                >
                  {cat}
                </button>
              ))}
            </div>
          )}

          {registryLoading ? (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="rounded-xl border border-border p-6">
                  <div className="h-10 w-10 rounded-lg bg-muted animate-pulse mb-3" />
                  <div className="h-4 w-24 bg-muted animate-pulse rounded" />
                  <div className="h-3 w-full bg-muted animate-pulse rounded mt-3" />
                </div>
              ))}
            </div>
          ) : registryError ? (
            <div className="text-center py-20 space-y-3">
              <p className="text-muted-foreground">{registryError}</p>
              <Button variant="outline" size="sm" onClick={loadRegistryAgents}>
                Retry
              </Button>
            </div>
          ) : filteredRegistryAgents.length === 0 ? (
            <div className="text-center py-20 text-muted-foreground">
              {registrySearch || registryCategory
                ? "No agents match your filters."
                : "No agents found in the registry."}
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
        </div>
      )}
    </div>
  );
}

/* ─── Agent Card ─── */

function AgentCard({ agent, onClick }: { agent: any; onClick: () => void }) {
  const manifest = agent.manifest;
  const exp = manifest?.experience;
  const tags = manifest?.marketplace?.tags || manifest?.tags || [];

  return (
    <div
      onClick={onClick}
      className="group cursor-pointer rounded-xl border border-border p-6 transition-all duration-150 hover:border-foreground/20 hover:scale-[1.01]"
    >
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-3">
          {manifest?.emoji && (
            <span className="text-2xl">{manifest.emoji}</span>
          )}
          <div>
            <h3 className="font-semibold">{agent.name}</h3>
            <p className="text-xs text-muted-foreground">
              v{agent.version} · {manifest?.author || "unknown"}
            </p>
          </div>
        </div>
      </div>

      <p className="text-sm text-muted-foreground line-clamp-2 leading-relaxed">
        {agent.description || manifest?.description}
      </p>

      {tags.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mt-4">
          {tags.slice(0, 3).map((t: string) => (
            <span
              key={t}
              className="text-[11px] px-2 py-0.5 rounded-full bg-muted text-muted-foreground"
            >
              {t}
            </span>
          ))}
          {tags.length > 3 && (
            <span className="text-[11px] px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
              +{tags.length - 3}
            </span>
          )}
        </div>
      )}

      <div className="flex items-center justify-between mt-4 pt-4 border-t border-border">
        <div className="flex gap-3 text-xs text-muted-foreground">
          {agent.downloads > 0 && <span>{agent.downloads} installs</span>}
          {agent.rating > 0 && <span>{agent.rating.toFixed(1)} stars</span>}
        </div>
        <Button
          variant="outline"
          size="sm"
          className="h-7 text-xs opacity-0 group-hover:opacity-100 transition-opacity duration-150"
          onClick={(e) => {
            e.stopPropagation();
          }}
        >
          Install
        </Button>
      </div>
    </div>
  );
}

/* ─── Registry Agent Card ─── */

function RegistryAgentCard({
  entry,
  onClick,
}: {
  entry: RegistryEntry;
  onClick: () => void;
}) {
  return (
    <div
      onClick={onClick}
      className="group cursor-pointer rounded-xl border border-border p-6 transition-all duration-150 hover:border-foreground/20 hover:scale-[1.01]"
    >
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-3">
          {entry.emoji && <span className="text-2xl">{entry.emoji}</span>}
          <div>
            <h3 className="font-semibold">{entry.name}</h3>
            <p className="text-xs text-muted-foreground">{entry.id}</p>
          </div>
        </div>
        <span className="text-[11px] px-2.5 py-0.5 rounded-full bg-muted text-muted-foreground capitalize">
          {entry.category}
        </span>
      </div>

      <p className="text-sm text-muted-foreground line-clamp-2 leading-relaxed">
        {entry.description}
      </p>

      <div className="flex items-center justify-between mt-4 pt-4 border-t border-border">
        <span className="text-xs text-muted-foreground capitalize">
          {entry.category}
        </span>
        <Button
          variant="outline"
          size="sm"
          className="h-7 text-xs opacity-0 group-hover:opacity-100 transition-opacity duration-150"
          onClick={(e) => {
            e.stopPropagation();
          }}
        >
          Install
        </Button>
      </div>
    </div>
  );
}

/* ─── Icons ─── */

function SearchIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="11" cy="11" r="8" />
      <path d="m21 21-4.3-4.3" />
    </svg>
  );
}
