import { proxyRequestWithAuth } from "@/lib/api";
import { NextResponse } from "next/server";

export async function POST(request: Request) {
  try {
    const res = await proxyRequestWithAuth("/api/v1/admin/runtimes/install", request, {
      method: "POST",
      body: await request.text(),
    });
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "Failed to start install" },
      { status: 502 }
    );
  }
}
