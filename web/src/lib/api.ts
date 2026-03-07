const API_BASE = process.env.ABOX_API_URL || "http://localhost:8080";

export async function proxyRequest(
  path: string,
  init?: RequestInit
): Promise<Response> {
  const url = `${API_BASE}${path}`;
  return fetch(url, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
}

export async function proxyRequestWithAuth(
  path: string,
  request: Request,
  init?: RequestInit
): Promise<Response> {
  const authHeader = request.headers.get("Authorization");
  const apiKey = request.headers.get("x-api-key");
  const baseUrl = request.headers.get("x-base-url");
  const model = request.headers.get("x-model");

  const url = `${API_BASE}${path}`;
  return fetch(url, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(authHeader ? { Authorization: authHeader } : {}),
      ...(apiKey ? { "x-api-key": apiKey } : {}),
      ...(baseUrl ? { "x-base-url": baseUrl } : {}),
      ...(model ? { "x-model": model } : {}),
      ...init?.headers,
    },
  });
}

// Get AI provider settings from localStorage
export function getAiSettings(): { apiKey: string; baseUrl: string; model: string } | null {
  if (typeof window === "undefined") return null;
  const saved = localStorage.getItem("abox_ai_settings");
  if (!saved) return null;
  try {
    const s = JSON.parse(saved);
    if (!s.apiKey) return null;
    return s;
  } catch {
    return null;
  }
}

// Client-side fetch wrapper with auth + AI provider headers
export function clientFetch(path: string, init?: RequestInit): Promise<Response> {
  const token = typeof window !== "undefined" ? localStorage.getItem("abox_token") : null;
  const ai = getAiSettings();

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(ai?.apiKey ? { "x-api-key": ai.apiKey } : {}),
    ...(ai?.baseUrl ? { "x-base-url": ai.baseUrl } : {}),
    ...(ai?.model ? { "x-model": ai.model } : {}),
    ...((init?.headers as Record<string, string>) || {}),
  };

  return fetch(path, {
    ...init,
    headers,
  });
}
