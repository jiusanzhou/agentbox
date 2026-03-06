import { proxyRequestWithAuth } from "@/lib/api";
import { NextResponse } from "next/server";

export async function GET(request: Request) {
  try {
    const res = await proxyRequestWithAuth("/api/v1/authme", request);
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "Auth service unavailable" },
      { status: 502 }
    );
  }
}
