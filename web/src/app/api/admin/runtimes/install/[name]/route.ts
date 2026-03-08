import { NextResponse } from "next/server";

const API_BASE = process.env.ABOX_API_URL || "http://localhost:8080";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ name: string }> }
) {
  const { name } = await params;
  const authHeader = request.headers.get("Authorization");

  const url = `${API_BASE}/api/v1/admin/runtimes/install/${encodeURIComponent(name)}`;

  try {
    const res = await fetch(url, {
      headers: {
        ...(authHeader ? { Authorization: authHeader } : {}),
      },
    });

    if (!res.ok || !res.body) {
      return NextResponse.json(
        { error: "Failed to connect to install stream" },
        { status: res.status || 502 }
      );
    }

    // Proxy the SSE stream
    return new Response(res.body, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      },
    });
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to install stream" },
      { status: 502 }
    );
  }
}
