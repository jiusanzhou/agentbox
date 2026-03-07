"use client";

import { useState, useEffect, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { useAuth } from "@/lib/auth";
import { clientFetch } from "@/lib/api";

// --- Types ---

interface SanitizedConfig {
  debug: boolean;
  addr: string;
  session_ttl: string;
  cleanup_interval: string;
  rate_limit: { requests_per_minute: number; burst_size: number };
  auth: { enabled: boolean; jwt_secret: string };
  channels: ChannelInfo[];
}

interface ChannelInfo {
  type: string;
  name?: string;
  config: Record<string, string>;
}

interface RuntimeInfo {
  name: string;
  image: string;
  env_keys: string[];
}

// --- Channel type form fields ---

const channelFields: Record<string, { label: string; key: string; placeholder: string }[]> = {
  telegram: [{ label: "Bot Token", key: "token", placeholder: "123456:ABC-DEF..." }],
  discord: [{ label: "Bot Token", key: "token", placeholder: "Discord bot token" }],
  slack: [
    { label: "Bot Token", key: "bot_token", placeholder: "xoxb-..." },
    { label: "App Token", key: "app_token", placeholder: "xapp-..." },
  ],
  wecom: [
    { label: "Corp ID", key: "corp_id", placeholder: "ww..." },
    { label: "Agent ID", key: "agent_id", placeholder: "1000002" },
    { label: "Secret", key: "secret", placeholder: "App secret" },
    { label: "Token", key: "token", placeholder: "Callback token" },
    { label: "Encoding AES Key", key: "encoding_aes_key", placeholder: "AES key" },
  ],
  webhook: [{ label: "Path", key: "path", placeholder: "/webhook/my-hook" }],
};

// --- Component ---

export default function SettingsPage() {
  const { user } = useAuth();
  const [apiKey, setApiKey] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);

  // AI Provider settings
  const [aiApiKey, setAiApiKey] = useState("");
  const [aiBaseUrl, setAiBaseUrl] = useState("");
  const [aiModel, setAiModel] = useState("");
  const [aiSaved, setAiSaved] = useState(false);

  // Admin state
  const [adminConfig, setAdminConfig] = useState<SanitizedConfig | null>(null);
  const [runtimes, setRuntimes] = useState<RuntimeInfo[]>([]);
  const [adminLoading, setAdminLoading] = useState(false);

  // Rate limit form
  const [rpm, setRpm] = useState(60);
  const [burst, setBurst] = useState(10);
  const [rateSaved, setRateSaved] = useState(false);

  // Session form
  const [sessionTTL, setSessionTTL] = useState("1h");
  const [cleanupInterval, setCleanupInterval] = useState("5m");
  const [sessionSaved, setSessionSaved] = useState(false);

  // New channel form
  const [newChannelType, setNewChannelType] = useState("");
  const [newChannelFields, setNewChannelFields] = useState<Record<string, string>>({});
  const [channelAdding, setChannelAdding] = useState(false);

  // Load saved AI settings from localStorage
  useEffect(() => {
    const saved = localStorage.getItem("abox_ai_settings");
    if (saved) {
      try {
        const s = JSON.parse(saved);
        setAiApiKey(s.apiKey || "");
        setAiBaseUrl(s.baseUrl || "");
        setAiModel(s.model || "");
      } catch {}
    }
  }, []);

  const generateApiKey = async () => {
    setGenerating(true);
    try {
      const res = await clientFetch("/api/auth/apikey", { method: "POST" });
      const data = await res.json();
      if (data.api_key) setApiKey(data.api_key);
    } catch {
      setApiKey("Failed to generate key");
    } finally {
      setGenerating(false);
    }
  };

  const saveAiSettings = () => {
    localStorage.setItem(
      "abox_ai_settings",
      JSON.stringify({ apiKey: aiApiKey, baseUrl: aiBaseUrl, model: aiModel })
    );
    setAiSaved(true);
    setTimeout(() => setAiSaved(false), 2000);
  };

  const clearAiSettings = () => {
    localStorage.removeItem("abox_ai_settings");
    setAiApiKey("");
    setAiBaseUrl("");
    setAiModel("");
  };

  // --- Admin data loading ---

  const loadAdminConfig = useCallback(async () => {
    setAdminLoading(true);
    try {
      const [cfgRes, rtRes] = await Promise.all([
        clientFetch("/api/admin/config"),
        clientFetch("/api/admin/runtimes"),
      ]);
      if (cfgRes.ok) {
        const cfg: SanitizedConfig = await cfgRes.json();
        setAdminConfig(cfg);
        setRpm(cfg.rate_limit?.requests_per_minute || 60);
        setBurst(cfg.rate_limit?.burst_size || 10);
        setSessionTTL(cfg.session_ttl || "1h");
        setCleanupInterval(cfg.cleanup_interval || "5m");
      }
      if (rtRes.ok) {
        const rt = await rtRes.json();
        setRuntimes(Array.isArray(rt) ? rt : []);
      }
    } catch {
      // ignore
    } finally {
      setAdminLoading(false);
    }
  }, []);

  const saveRateLimit = async () => {
    try {
      const res = await clientFetch("/api/admin/config", {
        method: "PUT",
        body: JSON.stringify({
          rate_limit: { requests_per_minute: rpm, burst_size: burst },
        }),
      });
      if (res.ok) {
        setRateSaved(true);
        setTimeout(() => setRateSaved(false), 2000);
      }
    } catch {}
  };

  const saveSessionSettings = async () => {
    try {
      const res = await clientFetch("/api/admin/config", {
        method: "PUT",
        body: JSON.stringify({
          session_ttl: sessionTTL,
          cleanup_interval: cleanupInterval,
        }),
      });
      if (res.ok) {
        setSessionSaved(true);
        setTimeout(() => setSessionSaved(false), 2000);
      }
    } catch {}
  };

  const removeChannel = async (index: number) => {
    try {
      await clientFetch(`/api/admin/config/channels/${index}`, {
        method: "DELETE",
      });
      loadAdminConfig();
    } catch {}
  };

  const addChannel = async () => {
    if (!newChannelType) return;
    setChannelAdding(true);
    try {
      const res = await clientFetch("/api/admin/config/channels", {
        method: "POST",
        body: JSON.stringify({
          type: newChannelType,
          config: JSON.stringify(newChannelFields),
        }),
      });
      if (res.ok) {
        setNewChannelType("");
        setNewChannelFields({});
        loadAdminConfig();
      }
    } catch {}
    setChannelAdding(false);
  };

  // --- Render ---

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage your account and server configuration
        </p>
      </div>

      <Tabs defaultValue="personal" onValueChange={(v) => {
        if (v === "admin" && !adminConfig) loadAdminConfig();
      }}>
        <TabsList>
          <TabsTrigger value="personal">Personal</TabsTrigger>
          <TabsTrigger value="admin">Admin</TabsTrigger>
        </TabsList>

        {/* === Personal Tab === */}
        <TabsContent value="personal">
          <div className="space-y-6 mt-4">
            {/* Profile */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Profile</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>Name</Label>
                  <Input value={user?.name || ""} readOnly className="bg-muted" />
                </div>
                <div className="space-y-2">
                  <Label>Email</Label>
                  <Input value={user?.email || ""} readOnly className="bg-muted" />
                </div>
              </CardContent>
            </Card>

            {/* AI Provider Settings */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base">AI Provider</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  Configure your AI model provider. The API key is injected into the sandbox agent as{" "}
                  <code className="text-xs bg-muted px-1 py-0.5 rounded">x-api-key</code> header
                  and <code className="text-xs bg-muted px-1 py-0.5 rounded">ANTHROPIC_API_KEY</code> env var.
                </p>
                <div className="space-y-2">
                  <Label>API Key</Label>
                  <Input
                    type="password"
                    placeholder="sk-ant-... or your provider key"
                    value={aiApiKey}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setAiApiKey(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label>Base URL <span className="text-muted-foreground font-normal">(optional)</span></Label>
                  <Input
                    placeholder="https://api.anthropic.com or custom endpoint"
                    value={aiBaseUrl}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setAiBaseUrl(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    Leave empty for default. Set for proxies or alternative providers.
                  </p>
                </div>
                <div className="space-y-2">
                  <Label>Model <span className="text-muted-foreground font-normal">(optional)</span></Label>
                  <Input
                    placeholder="claude-sonnet-4-20250514"
                    value={aiModel}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setAiModel(e.target.value)}
                  />
                </div>
                <div className="flex gap-2">
                  <Button size="sm" onClick={saveAiSettings}>
                    {aiSaved ? "Saved" : "Save"}
                  </Button>
                  <Button size="sm" variant="outline" onClick={clearAiSettings}>
                    Clear
                  </Button>
                </div>
              </CardContent>
            </Card>

            {/* ABox API Key */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base">ABox API Key</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  Use your ABox API key to authenticate requests from external applications.
                </p>
                {apiKey ? (
                  <div className="space-y-2">
                    <code className="block rounded-md bg-muted px-3 py-2 text-sm font-mono break-all">
                      {apiKey}
                    </code>
                    <p className="text-xs text-muted-foreground">
                      Copy this key now. It won&apos;t be shown again.
                    </p>
                  </div>
                ) : (
                  <Button variant="outline" size="sm" onClick={generateApiKey} disabled={generating}>
                    {generating ? "Generating..." : "Generate API Key"}
                  </Button>
                )}
              </CardContent>
            </Card>

            {/* Plan */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Plan</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium">Free</p>
                    <p className="text-sm text-muted-foreground">
                      Basic access with limited runs
                    </p>
                  </div>
                  <Button variant="outline" size="sm" disabled>
                    Upgrade
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* === Admin Tab === */}
        <TabsContent value="admin">
          <div className="space-y-6 mt-4">
            {adminLoading && (
              <p className="text-sm text-muted-foreground">Loading configuration...</p>
            )}

            {/* IM Channels */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base">IM Channels</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                {adminConfig?.channels && adminConfig.channels.length > 0 ? (
                  <div className="space-y-2">
                    {adminConfig.channels.map((ch, i) => (
                      <div key={i} className="flex items-center gap-2 py-2 px-3 rounded-md bg-muted/50">
                        <Badge variant="secondary">{ch.type}</Badge>
                        <span className="text-sm text-muted-foreground flex-1 truncate">
                          {channelSummary(ch)}
                        </span>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
                          onClick={() => removeChannel(i)}
                        >
                          x
                        </Button>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">No channels configured.</p>
                )}

                <Separator />

                <div className="space-y-3">
                  <Label>Add channel</Label>
                  <select
                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                    value={newChannelType}
                    onChange={(e) => {
                      setNewChannelType(e.target.value);
                      setNewChannelFields({});
                    }}
                  >
                    <option value="">Select type...</option>
                    <option value="telegram">Telegram</option>
                    <option value="discord">Discord</option>
                    <option value="slack">Slack</option>
                    <option value="wecom">WeCom</option>
                    <option value="webhook">Webhook</option>
                  </select>

                  {newChannelType && channelFields[newChannelType]?.map((f) => (
                    <div key={f.key} className="space-y-1">
                      <Label className="text-sm">{f.label}</Label>
                      <Input
                        placeholder={f.placeholder}
                        value={newChannelFields[f.key] || ""}
                        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                          setNewChannelFields((prev) => ({ ...prev, [f.key]: e.target.value }))
                        }
                      />
                    </div>
                  ))}

                  {newChannelType && (
                    <Button size="sm" onClick={addChannel} disabled={channelAdding}>
                      {channelAdding ? "Adding..." : "Add Channel"}
                    </Button>
                  )}
                </div>
              </CardContent>
            </Card>

            {/* Rate Limiting */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Rate Limiting</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>Requests per minute</Label>
                  <Input
                    type="number"
                    value={rpm}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setRpm(Number(e.target.value))}
                  />
                </div>
                <div className="space-y-2">
                  <Label>Burst size</Label>
                  <Input
                    type="number"
                    value={burst}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setBurst(Number(e.target.value))}
                  />
                </div>
                <Button size="sm" onClick={saveRateLimit}>
                  {rateSaved ? "Saved" : "Save"}
                </Button>
              </CardContent>
            </Card>

            {/* Session Settings */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Sessions</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>Session TTL</Label>
                  <Input
                    placeholder="1h"
                    value={sessionTTL}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSessionTTL(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    Duration before idle sessions are cleaned up (e.g. 30m, 1h, 2h).
                  </p>
                </div>
                <div className="space-y-2">
                  <Label>Cleanup interval</Label>
                  <Input
                    placeholder="5m"
                    value={cleanupInterval}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setCleanupInterval(e.target.value)}
                  />
                </div>
                <Button size="sm" onClick={saveSessionSettings}>
                  {sessionSaved ? "Saved" : "Save"}
                </Button>
              </CardContent>
            </Card>

            {/* Agent Runtimes */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Agent Runtimes</CardTitle>
              </CardHeader>
              <CardContent>
                {runtimes.length > 0 ? (
                  <div className="space-y-2">
                    {runtimes.map((rt) => (
                      <div key={rt.name} className="flex items-center gap-2 py-1.5">
                        <Badge variant="outline">{rt.name}</Badge>
                        <span className="text-sm text-muted-foreground">
                          {rt.image || "custom"}
                        </span>
                        {rt.env_keys?.length > 0 && (
                          <span className="text-xs text-muted-foreground">
                            {rt.env_keys.join(", ")}
                          </span>
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">No runtimes registered.</p>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function channelSummary(ch: ChannelInfo): string {
  const cfg = ch.config || {};
  switch (ch.type) {
    case "telegram":
    case "discord":
      return cfg.token ? `Token: ${cfg.token}` : "";
    case "slack":
      return cfg.bot_token ? `Bot: ${cfg.bot_token}` : "";
    case "webhook":
      return cfg.path ? `Path: ${cfg.path}` : "";
    case "wecom":
      return cfg.corp_id ? `Corp: ${cfg.corp_id}` : "";
    default:
      return JSON.stringify(cfg).slice(0, 50);
  }
}
