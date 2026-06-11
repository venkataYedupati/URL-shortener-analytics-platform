import type { Analytics, Link } from "./types";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

type CreateLinkPayload = {
  target_url: string;
  title?: string;
  custom_code?: string;
  custom_domain?: string;
  expires_at?: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...init?.headers
    },
    ...init
  });

  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error ?? `Request failed with ${response.status}`);
  }

  return response.json() as Promise<T>;
}

export const api = {
  baseURL: API_BASE,

  async listLinks(): Promise<Link[]> {
    const body = await request<{ links: Link[] }>("/v1/overview?limit=50");
    return body.links;
  },

  async createLink(payload: CreateLinkPayload): Promise<Link> {
    return request<Link>("/v1/links", {
      method: "POST",
      body: JSON.stringify(payload)
    });
  },

  async getAnalytics(code: string, hours = 168): Promise<Analytics> {
    return request<Analytics>(`/v1/links/${encodeURIComponent(code)}/analytics?hours=${hours}`);
  },

  qrURL(code: string): string {
    return `${API_BASE}/v1/links/${encodeURIComponent(code)}/qr`;
  }
};
