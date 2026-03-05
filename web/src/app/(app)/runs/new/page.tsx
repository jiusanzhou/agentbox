"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

export default function NewRunPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [agentFile, setAgentFile] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !agentFile.trim()) {
      setError("Name and Agent File are required.");
      return;
    }

    setSubmitting(true);
    setError("");

    try {
      const res = await fetch("/api/runs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: name.trim(),
          agent_file: agentFile,
          config: {},
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        setError(data.error || "Failed to create run");
        return;
      }

      const run = await res.json();
      router.push(`/runs/${run.id}`);
    } catch {
      setError("Failed to create run. Is the API running?");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">New Run</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Create a new agent workflow run
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Run Configuration</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                placeholder="my-agent-run"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="agent-file">Agent File</Label>
              <Textarea
                id="agent-file"
                placeholder="Enter your agent file content (markdown supported)..."
                className="min-h-[300px] font-mono text-sm"
                value={agentFile}
                onChange={(e) => setAgentFile(e.target.value)}
              />
            </div>

            {error && (
              <p className="text-sm text-red-400">{error}</p>
            )}

            <div className="flex items-center gap-3">
              <Button type="submit" disabled={submitting}>
                {submitting ? "Creating..." : "Create Run"}
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={() => router.push("/runs")}
              >
                Cancel
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
