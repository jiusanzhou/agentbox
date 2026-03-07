import { proxyRequestWithAuth } from "@/lib/api";
import { NextResponse } from "next/server";

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  try {
    const res = await proxyRequestWithAuth(`/api/v1/integrations/${id}/test`, request, {
      method: "POST",
    });
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json({ error: "Failed to test integration" }, { status: 502 });
  }
}
