"use client";

import { useEffect, useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { clientFetch } from "@/lib/api";

interface Integration {
  id: string;
  user_id: string;
  type: string;
  name: string;
  config: Record<string, string>;
  session_id: string;
  enabled: boolean;
  status: string;
  error?: string;
  created_at: string;
  updated_at: string;
}

const CHANNEL_TYPES = [
  { value: "telegram", label: "Telegram", icon: "🤖" },
  { value: "discord", label: "Discord", icon: "💬" },
  { value: "slack", label: "Slack", icon: "📱" },
  { value: "wecom", label: "WeChat Work", icon: "💼" },
  { value: "webhook", label: "Webhook", icon: "🔗" },
];

const CHANNEL_FIELDS: Record<string, { key: string; label: string; type?: string }[]> = {
  telegram: [{ key: "token", label: "Bot Token" }],
  discord: [
    { key: "token", label: "Bot Token" },
    { key: "guild_id", label: "Guild ID (optional)" },
  ],
  slack: [
    { key: "token", label: "Bot Token (xoxb-...)" },
    { key: "app_token", label: "App Token (xapp-...)" },
  ],
  wecom: [
    { key: "corp_id", label: "Corp ID" },
    { key: "agent_id", label: "Agent ID" },
    { key: "secret", label: "Secret" },
    { key: "token", label: "Token" },
    { key: "encoding_aes_key", label: "Encoding AES Key" },
  ],
  webhook: [{ key: "secret", label: "HMAC Secret (optional)" }],
};

export default function IntegrationsPage() {
  const [integrations, setIntegrations] = useState<Integration[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [selectedType, setSelectedType] = useState("");
  const [formName, setFormName] = useState("");
  const [formConfig, setFormConfig] = useState<Record<string, string>>({});
  const [formEnabled, setFormEnabled] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await clientFetch("/api/integrations");
      if (res.ok) setIntegrations(await res.json());
    } catch { /* ignore */ }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    if (!selectedType) return;
    setSaving(true);
    try {
      const res = await clientFetch("/api/integrations", {
        method: "POST",
        body: JSON.stringify({
          type: selectedType,
          name: formName || `My ${CHANNEL_TYPES.find((t) => t.value === selectedType)?.label}`,
          config: formConfig,
          enabled: formEnabled,
        }),
      });
      if (res.ok) {
        setOpen(false);
        setSelectedType("");
        setFormName("");
        setFormConfig({});
        load();
      }
    } catch { /* ignore */ }
    setSaving(false);
  };

  const toggleEnabled = async (intg: Integration) => {
    await clientFetch(`/api/integrations/${intg.id}`, {
      method: "PUT",
      body: JSON.stringify({ enabled: !intg.enabled }),
    });
    load();
  };

  const remove = async (id: string) => {
    await clientFetch(`/api/integrations/${id}`, { method: "DELETE" });
    load();
  };

  const test = async (id: string) => {
    const res = await clientFetch(`/api/integrations/${id}/test`, { method: "POST" });
    const data = await res.json();
    if (data.status === "ok") {
      alert("Connection successful!");
    } else {
      alert("Connection failed: " + (data.error || "unknown error"));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Integrations</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Connect your IM channels to agent sessions
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button size="sm">+ Add New</Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Add Integration</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              {!selectedType ? (
                <div className="grid grid-cols-2 gap-2">
                  {CHANNEL_TYPES.map((ct) => (
                    <button
                      key={ct.value}
                      onClick={() => setSelectedType(ct.value)}
                      className="flex items-center gap-2 rounded-md border p-3 text-sm hover:bg-accent transition-colors text-left"
                    >
                      <span className="text-lg">{ct.icon}</span>
                      {ct.label}
                    </button>
                  ))}
                </div>
              ) : (
                <>
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <button onClick={() => setSelectedType("")} className="hover:text-foreground">&larr;</button>
                    <span>{CHANNEL_TYPES.find((t) => t.value === selectedType)?.icon}</span>
                    <span>{CHANNEL_TYPES.find((t) => t.value === selectedType)?.label}</span>
                  </div>
                  <div className="space-y-3">
                    <div>
                      <Label>Name</Label>
                      <Input
                        placeholder={`My ${CHANNEL_TYPES.find((t) => t.value === selectedType)?.label}`}
                        value={formName}
                        onChange={(e) => setFormName(e.target.value)}
                      />
                    </div>
                    {CHANNEL_FIELDS[selectedType]?.map((field) => (
                      <div key={field.key}>
                        <Label>{field.label}</Label>
                        <Input
                          type={field.type || "text"}
                          value={formConfig[field.key] || ""}
                          onChange={(e) =>
                            setFormConfig((prev) => ({ ...prev, [field.key]: e.target.value }))
                          }
                        />
                      </div>
                    ))}
                    {selectedType === "webhook" && (
                      <p className="text-xs text-muted-foreground">
                        A unique webhook URL will be generated after creation.
                      </p>
                    )}
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id="enabled"
                        checked={formEnabled}
                        onChange={(e) => setFormEnabled(e.target.checked)}
                        className="rounded"
                      />
                      <Label htmlFor="enabled">Enable immediately</Label>
                    </div>
                  </div>
                  <div className="flex justify-end gap-2">
                    <Button variant="outline" onClick={() => setOpen(false)}>Cancel</Button>
                    <Button onClick={handleCreate} disabled={saving}>
                      {saving ? "Creating..." : "Create"}
                    </Button>
                  </div>
                </>
              )}
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {loading ? (
        <div className="py-12 text-center text-sm text-muted-foreground">Loading...</div>
      ) : integrations.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          No integrations yet. Add one to connect an IM channel.
        </div>
      ) : (
        <div className="space-y-3">
          {integrations.map((intg) => {
            const ct = CHANNEL_TYPES.find((t) => t.value === intg.type);
            return (
              <Card key={intg.id}>
                <CardContent className="flex items-center justify-between py-4 px-5">
                  <div className="space-y-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-lg">{ct?.icon || "?"}</span>
                      <span className="font-medium text-sm">{intg.name || ct?.label}</span>
                      <Badge
                        variant={intg.status === "connected" ? "default" : "secondary"}
                        className="text-[10px]"
                      >
                        {intg.status}
                      </Badge>
                    </div>
                    <div className="text-xs text-muted-foreground flex items-center gap-2">
                      <span>{intg.type}</span>
                      {intg.session_id && <span>· Session: {intg.session_id}</span>}
                      {intg.type === "webhook" && (
                        <span>· URL: /api/v1/hook/{intg.id}</span>
                      )}
                    </div>
                    {intg.error && (
                      <p className="text-xs text-destructive">{intg.error}</p>
                    )}
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <Button size="sm" variant="outline" onClick={() => test(intg.id)}>
                      Test
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => toggleEnabled(intg)}>
                      {intg.enabled ? "Disable" : "Enable"}
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => remove(intg.id)}>
                      Remove
                    </Button>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
