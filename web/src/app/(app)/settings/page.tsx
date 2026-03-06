"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { useAuth } from "@/lib/auth";
import { clientFetch } from "@/lib/api";

export default function SettingsPage() {
  const { user } = useAuth();
  const [apiKey, setApiKey] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);

  // AI Provider settings
  const [aiApiKey, setAiApiKey] = useState("");
  const [aiBaseUrl, setAiBaseUrl] = useState("");
  const [aiModel, setAiModel] = useState("");
  const [aiSaved, setAiSaved] = useState(false);

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
      const res = await clientFetch("/api/auth/apikey", {
        method: "POST",
      });
      const data = await res.json();
      if (data.api_key) {
        setApiKey(data.api_key);
      }
    } catch {
      setApiKey("Failed to generate key");
    } finally {
      setGenerating(false);
    }
  };

  const saveAiSettings = () => {
    const settings = {
      apiKey: aiApiKey,
      baseUrl: aiBaseUrl,
      model: aiModel,
    };
    localStorage.setItem("abox_ai_settings", JSON.stringify(settings));
    setAiSaved(true);
    setTimeout(() => setAiSaved(false), 2000);
  };

  const clearAiSettings = () => {
    localStorage.removeItem("abox_ai_settings");
    setAiApiKey("");
    setAiBaseUrl("");
    setAiModel("");
  };

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage your account and preferences
        </p>
      </div>

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
              {aiSaved ? "✓ Saved" : "Save"}
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
            <Button
              variant="outline"
              size="sm"
              onClick={generateApiKey}
              disabled={generating}
            >
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

      <Separator />

      {/* Connected IMs */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Connected Channels</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Connect messaging platforms to interact with agents via chat.
            Coming soon.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
