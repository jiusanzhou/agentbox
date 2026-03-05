import { proxyRequest } from "@/lib/api";
import { NextResponse } from "next/server";

export async function GET() {
  try {
    const res = await proxyRequest("/api/v1/runs");
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "Failed to fetch runs" },
      { status: 502 }
    );
  }
}

export async function POST(request: Request) {
  try {
    const body = await request.json();
    const res = await proxyRequest("/api/v1/run", {
      method: "POST",
      body: JSON.stringify(body),
    });
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "Failed to create run" },
      { status: 502 }
    );
  }
}
