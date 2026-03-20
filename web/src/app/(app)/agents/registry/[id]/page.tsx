"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { clientFetch } from "@/lib/api";

export default function RegistryAgentDetailPage() {
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();

  const agentId = params.id as string;
  const name = searchParams.get("name") || agentId;
  const emoji = searchParams.get("emoji") || "";
  const description = searchParams.get("description") || "";
  const category = searchParams.get("category") || "";
  const yamlUrl = searchParams.get("url") || "";

  const [yamlContent, setYamlContent] = useState<string>("");
  const [yamlLoading, setYamlLoading] = useState(false);
  const [yamlError, setYamlError] = useState("");

  const [installing, setInstalling] = useState(false);
  const [installStatus, setInstallStatus] = useState<"idle" | "success" | "error">("idle");
  const [installMessage, setInstallMessage] = useState("");

  const [running, setRunning] = useState(false);

  // Derive the GitHub page URL from the raw content URL
  const githubPageUrl = yamlUrl
    ? yamlUrl
        .replace("raw.githubusercontent.com", "github.com")
        .replace("/main/", "/blob/main/")
    : "";

  useEffect(() => {
    if (yamlUrl) {
      fetchYaml();
    }
  }, [yamlUrl]);

  async function fetchYaml() {
    setYamlLoading(true);
    setYamlError("");
    try {
      const resp = await fetch(yamlUrl);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const text = await resp.text();
      setYamlContent(text);
    } catch (e) {
      console.error("Failed to fetch agent YAML:", e);
      setYamlError("Failed to load agent definition from GitHub.");
    } finally {
      setYamlLoading(false);
    }
  }

  async function handleInstall() {
    setInstalling(true);
    setInstallStatus("idle");
    setInstallMessage("");
    try {
      const body = {
        slug: agentId,
        identity: {
          name: name,
          description: description,
        },
        manifest: {
          version: "1.0.0",
          author: "openagent-registry",
          runtime: "claude",
        },
        repo_url: githubPageUrl || yamlUrl,
      };

      const resp = await clientFetch("/api/v1/registry/agents", {
        method: "POST",
        body: JSON.stringify(body),
      });

      if (!resp.ok) {
        const err = await resp.json().catch(() => ({ error: "Unknown error" }));
        throw new Error(err.error || `HTTP ${resp.status}`);
      }

      setInstallStatus("success");
      setInstallMessage("Agent installed successfully.");
    } catch (e: any) {
      setInstallStatus("error");
      setInstallMessage(e.message || "Failed to install agent.");
    } finally {
      setInstalling(false);
    }
  }

  async function handleRun() {
    setRunning(true);
    try {
      // Install first if not already installed
      if (installStatus !== "success") {
        const installBody = {
          slug: agentId,
          identity: {
            name: name,
            description: description,
          },
          manifest: {
            version: "1.0.0",
            author: "openagent-registry",
            runtime: "claude",
          },
          repo_url: githubPageUrl || yamlUrl,
        };

        const installResp = await clientFetch("/api/v1/registry/agents", {
          method: "POST",
          body: JSON.stringify(installBody),
        });

        // Ignore 409 conflict (already installed)
        if (!installResp.ok && installResp.status !== 409) {
          const err = await installResp.json().catch(() => ({ error: "Unknown error" }));
          throw new Error(err.error || "Failed to install agent before running.");
        }
      }

      // Hire the agent
      const hireResp = await clientFetch(`/api/v1/registry/agents/${agentId}/hire`, {
        method: "POST",
        body: JSON.stringify({ message: "" }),
      });

      if (!hireResp.ok) {
        const err = await hireResp.json().catch(() => ({ error: "Unknown error" }));
        throw new Error(err.error || "Failed to run agent.");
      }

      const run = await hireResp.json();
      router.push(`/chat?session=${run.id}`);
    } catch (e: any) {
      setInstallStatus("error");
      setInstallMessage(e.message || "Failed to run agent.");
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Back link */}
      <Link
        href="/agents"
        className="text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        &larr; Back to Marketplace
      </Link>

      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-4">
          {emoji && <span className="text-5xl">{emoji}</span>}
          <div>
            <h1 className="text-3xl font-bold">{name}</h1>
            <p className="text-muted-foreground">
              {agentId} &middot; OpenAgent Registry
            </p>
            {category && (
              <Badge variant="secondary" className="mt-1 capitalize">
                {category}
              </Badge>
            )}
          </div>
        </div>
        <div className="flex flex-col items-end gap-2">
          <span className="text-lg font-bold">Free</span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              onClick={handleInstall}
              disabled={installing || installStatus === "success"}
            >
              {installing
                ? "Installing..."
                : installStatus === "success"
                ? "Installed"
                : "Install"}
            </Button>
            <Button onClick={handleRun} disabled={running} size="lg">
              {running ? "Starting..." : "Run Agent"}
            </Button>
          </div>
        </div>
      </div>

      {/* Status message */}
      {installMessage && (
        <div
          className={`text-sm px-4 py-3 rounded-lg ${
            installStatus === "success"
              ? "bg-green-50 text-green-700 border border-green-200"
              : "bg-red-50 text-red-700 border border-red-200"
          }`}
        >
          {installMessage}
        </div>
      )}

      {/* Description */}
      <Card>
        <CardContent className="pt-6">
          <p className="text-lg">{description}</p>
        </CardContent>
      </Card>

      {/* Links */}
      <div className="flex gap-3">
        {githubPageUrl && (
          <a
            href={githubPageUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm text-blue-600 hover:underline"
          >
            View on GitHub &rarr;
          </a>
        )}
        {yamlUrl && (
          <a
            href={yamlUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm text-muted-foreground hover:underline"
          >
            Raw YAML
          </a>
        )}
      </div>

      {/* YAML Preview */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Agent Definition (agent.yaml)</CardTitle>
            {yamlError && (
              <Button variant="outline" size="sm" onClick={fetchYaml}>
                Retry
              </Button>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {yamlLoading ? (
            <div className="text-center py-8 text-muted-foreground">
              Loading agent definition...
            </div>
          ) : yamlError ? (
            <div className="text-center py-8 text-muted-foreground">{yamlError}</div>
          ) : yamlContent ? (
            <pre className="text-sm bg-muted/50 p-4 rounded-lg overflow-x-auto max-h-[600px] overflow-y-auto whitespace-pre-wrap break-words font-mono">
              {yamlContent}
            </pre>
          ) : (
            <p className="text-muted-foreground italic">
              No YAML URL available for this agent.
            </p>
          )}
        </CardContent>
      </Card>

      {/* Metadata */}
      <Card>
        <CardHeader>
          <CardTitle>Details</CardTitle>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <div>
            <span className="font-medium">ID:</span>{" "}
            <span className="font-mono text-muted-foreground">{agentId}</span>
          </div>
          {category && (
            <div>
              <span className="font-medium">Category:</span>{" "}
              <span className="capitalize">{category}</span>
            </div>
          )}
          <div>
            <span className="font-medium">Source:</span>{" "}
            <span>OpenAgent Registry (GitHub)</span>
          </div>
          <div>
            <span className="font-medium">Runtime:</span>{" "}
            <span className="font-mono">claude</span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
