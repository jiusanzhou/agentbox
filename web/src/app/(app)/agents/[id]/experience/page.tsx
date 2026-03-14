"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { clientFetch } from "@/lib/api";

const DIFFICULTY_COLORS: Record<string, string> = {
  basic: "bg-gray-100 text-gray-700",
  intermediate: "bg-blue-100 text-blue-700",
  advanced: "bg-orange-100 text-orange-700",
  expert: "bg-purple-100 text-purple-700",
};

export default function ExperienceReviewPage() {
  const params = useParams();
  const [memory, setMemory] = useState("");
  const [packs, setPacks] = useState<any[]>([]);
  const [sanitizeInput, setSanitizeInput] = useState("");
  const [sanitizeResult, setSanitizeResult] = useState<any>(null);
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState<string[]>([]);

  async function handleExtract() {
    if (!memory.trim()) return;

    // Call sanitize preview for the whole memory
    const resp = await clientFetch("/api/v1/experience/sanitize", {
      method: "POST",
      body: JSON.stringify({ text: memory }),
    });
    const result = await resp.json();

    // Simple client-side extraction (split by ### headings)
    const sections = memory.split(/^### /m).filter(Boolean);
    const extracted = sections.map((section, i) => {
      const lines = section.split("\n");
      const title = lines[0]?.trim() || `Section ${i + 1}`;
      const detail = lines.slice(1).join("\n").trim();
      return {
        id: `exp-review-${i + 1}`,
        summary: title,
        detail: detail,
        domain: "",
        difficulty: "",
        tags: [],
        sanitized: result.changed,
      };
    }).filter(p => p.detail.length > 20);

    setPacks(extracted);
  }

  async function handleSanitizePreview() {
    if (!sanitizeInput.trim()) return;
    const resp = await clientFetch("/api/v1/experience/sanitize", {
      method: "POST",
      body: JSON.stringify({ text: sanitizeInput }),
    });
    const result = await resp.json();
    setSanitizeResult(result);
  }

  async function handleSubmitPack(pack: any) {
    setSubmitting(true);
    try {
      const resp = await clientFetch(`/api/v1/registry/agents/${params.id}/experience`, {
        method: "POST",
        body: JSON.stringify({
          packs: [{
            id: pack.id,
            domain: pack.domain || "General",
            summary: pack.summary.slice(0, 200),
            detail: pack.detail,
            difficulty: pack.difficulty || "intermediate",
            tags: pack.tags,
          }],
        }),
      });
      if (resp.ok) {
        setSubmitted(prev => [...prev, pack.id]);
      }
    } catch (e) {
      console.error("Failed to submit:", e);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Experience Review</h1>
        <p className="text-muted-foreground">
          Extract, sanitize, and publish experience packs from memory.
        </p>
      </div>

      {/* Sanitize Preview Tool */}
      <Card>
        <CardHeader>
          <CardTitle>🔒 Sanitize Preview</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <textarea
            value={sanitizeInput}
            onChange={(e) => setSanitizeInput(e.target.value)}
            placeholder="Paste text to check for PII..."
            className="w-full h-24 p-3 border rounded-md text-sm font-mono"
          />
          <Button onClick={handleSanitizePreview} size="sm">Check</Button>
          {sanitizeResult && (
            <div className="space-y-2">
              {sanitizeResult.changed ? (
                <>
                  <div className="p-3 bg-red-50 rounded text-sm">
                    <span className="font-medium text-red-600">Redactions: </span>
                    {sanitizeResult.redactions?.join(", ") || "none"}
                  </div>
                  <div className="p-3 bg-green-50 rounded text-sm font-mono whitespace-pre-wrap">
                    {sanitizeResult.sanitized}
                  </div>
                </>
              ) : (
                <div className="p-3 bg-green-50 rounded text-sm text-green-700">
                  ✅ No PII detected. Text is clean.
                </div>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Memory Extraction */}
      <Card>
        <CardHeader>
          <CardTitle>📝 Extract from Memory</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <textarea
            value={memory}
            onChange={(e) => setMemory(e.target.value)}
            placeholder="Paste MEMORY.md content here..."
            className="w-full h-48 p-3 border rounded-md text-sm font-mono"
          />
          <Button onClick={handleExtract}>Extract Experiences</Button>
        </CardContent>
      </Card>

      {/* Extracted Packs Review */}
      {packs.length > 0 && (
        <div className="space-y-4">
          <h2 className="text-xl font-bold">Review & Publish ({packs.length} packs)</h2>
          {packs.map((pack) => (
            <Card key={pack.id} className={submitted.includes(pack.id) ? "opacity-50" : ""}>
              <CardContent className="pt-6 space-y-3">
                <div className="flex items-start justify-between">
                  <div>
                    <h3 className="font-medium">{pack.summary}</h3>
                    <p className="text-xs text-muted-foreground">{pack.id}</p>
                  </div>
                  {submitted.includes(pack.id) ? (
                    <Badge className="bg-green-100 text-green-700">Published</Badge>
                  ) : (
                    <Button
                      size="sm"
                      onClick={() => handleSubmitPack(pack)}
                      disabled={submitting}
                    >
                      Publish
                    </Button>
                  )}
                </div>

                <div className="p-3 bg-muted/50 rounded text-sm font-mono whitespace-pre-wrap max-h-48 overflow-auto">
                  {pack.detail}
                </div>

                <div className="flex gap-2">
                  <input
                    placeholder="Domain..."
                    value={pack.domain}
                    onChange={(e) => {
                      const updated = [...packs];
                      const idx = updated.indexOf(pack);
                      updated[idx] = { ...pack, domain: e.target.value };
                      setPacks(updated);
                    }}
                    className="border rounded px-2 py-1 text-sm flex-1"
                  />
                  <select
                    value={pack.difficulty}
                    onChange={(e) => {
                      const updated = [...packs];
                      const idx = updated.indexOf(pack);
                      updated[idx] = { ...pack, difficulty: e.target.value };
                      setPacks(updated);
                    }}
                    className="border rounded px-2 py-1 text-sm"
                  >
                    <option value="">Difficulty...</option>
                    <option value="basic">Basic</option>
                    <option value="intermediate">Intermediate</option>
                    <option value="advanced">Advanced</option>
                    <option value="expert">Expert</option>
                  </select>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
