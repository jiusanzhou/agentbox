import { proxyRequest } from "@/lib/api";
import { NextResponse } from "next/server";

export async function GET() {
  try {
    const res = await proxyRequest("/api/v1/auth/github");
    // The Go backend returns a redirect; forward it
    const location = res.headers.get("Location");
    if (location) {
      return NextResponse.redirect(location);
    }
    // Fallback: proxy the response
    const data = await res.text();
    return new NextResponse(data, { status: res.status });
  } catch {
    return NextResponse.json(
      { error: "GitHub auth unavailable" },
      { status: 502 }
    );
  }
}
