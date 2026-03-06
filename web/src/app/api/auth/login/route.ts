import { proxyRequest } from "@/lib/api";
import { NextResponse } from "next/server";

export async function POST(request: Request) {
  try {
    const body = await request.json();
    const res = await proxyRequest("/api/v1/authlogin", {
      method: "POST",
      body: JSON.stringify(body),
    });
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "Login service unavailable" },
      { status: 502 }
    );
  }
}
