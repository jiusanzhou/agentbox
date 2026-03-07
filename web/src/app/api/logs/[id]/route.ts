import { NextRequest } from "next/server";

const API_BASE = process.env.ABOX_API_URL || "http://localhost:8080";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const authHeader = request.headers.get("Authorization");

  const res = await fetch(`${API_BASE}/api/v1/logs/${id}`, {
    headers: {
      ...(authHeader ? { Authorization: authHeader } : {}),
    },
  });

  return new Response(res.body, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    },
  });
}
