import { proxyRequestWithAuth } from "@/lib/api";
import { NextResponse } from "next/server";

export async function GET(request: Request) {
  try {
    const res = await proxyRequestWithAuth("/api/v1/admin/config", request);
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "Failed to fetch config" },
      { status: 502 }
    );
  }
}

export async function PUT(request: Request) {
  try {
    const body = await request.json();
    const res = await proxyRequestWithAuth("/api/v1/admin/config", request, {
      method: "PUT",
      body: JSON.stringify(body),
    });
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "Failed to update config" },
      { status: 502 }
    );
  }
}
