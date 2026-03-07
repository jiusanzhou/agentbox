import { NextRequest } from "next/server";

const API_BASE = process.env.ABOX_API_URL || "http://localhost:8080";

export async function POST(request: NextRequest) {
  const formData = await request.formData();
  const authHeader = request.headers.get("Authorization");

  const res = await fetch(`${API_BASE}/api/v1/upload`, {
    method: "POST",
    headers: {
      ...(authHeader ? { Authorization: authHeader } : {}),
    },
    body: formData,
  });

  return new Response(res.body, {
    status: res.status,
    headers: { "Content-Type": "application/json" },
  });
}
