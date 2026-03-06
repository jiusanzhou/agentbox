import { proxyRequestWithAuth } from "@/lib/api";
import { NextResponse } from "next/server";

export async function POST(request: Request) {
  try {
    const body = await request.json();
    const res = await proxyRequestWithAuth("/api/v1/sessionmessage", request, {
      method: "POST",
      body: JSON.stringify(body),
    });
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "Failed to send message" },
      { status: 502 }
    );
  }
}
