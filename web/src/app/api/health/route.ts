import { proxyRequest } from "@/lib/api";
import { NextResponse } from "next/server";

export async function GET() {
  try {
    const res = await proxyRequest("/api/v1/health");
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { status: "unreachable" },
      { status: 502 }
    );
  }
}
