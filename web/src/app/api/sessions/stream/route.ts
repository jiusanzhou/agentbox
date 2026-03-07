import { NextRequest } from "next/server";

const API_BASE = process.env.ABOX_API_URL || "http://localhost:8080";

export async function POST(request: NextRequest) {
  const body = await request.json();
  const authHeader = request.headers.get("Authorization");
  const apiKey = request.headers.get("x-api-key");
  const baseUrl = request.headers.get("x-base-url");
  const model = request.headers.get("x-model");

  const res = await fetch(`${API_BASE}/api/v1/stream`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(authHeader ? { Authorization: authHeader } : {}),
      ...(apiKey ? { "x-api-key": apiKey } : {}),
      ...(baseUrl ? { "x-base-url": baseUrl } : {}),
      ...(model ? { "x-model": model } : {}),
    },
    body: JSON.stringify(body),
  });

  return new Response(res.body, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    },
  });
}
