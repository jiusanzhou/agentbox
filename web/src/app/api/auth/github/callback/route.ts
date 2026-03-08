import { proxyRequest } from "@/lib/api";
import { NextRequest, NextResponse } from "next/server";

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const code = searchParams.get("code");
    if (!code) {
      return NextResponse.json({ error: "missing code" }, { status: 400 });
    }

    const res = await proxyRequest(`/api/v1/auth/github/callback?code=${encodeURIComponent(code)}`);
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "GitHub callback failed" },
      { status: 502 }
    );
  }
}
