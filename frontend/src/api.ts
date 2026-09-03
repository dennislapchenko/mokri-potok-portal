// Thin fetch wrapper. The device token lives in localStorage and rides as a
// bearer header (a cookie would be third-party from github.io and iOS blocks it).
export const API = (import.meta.env.VITE_API_URL as string | undefined) || "/api";
const KEY = "potok.token";

export function getToken(): string | null {
  try { return localStorage.getItem(KEY); } catch { return null; }
}
export function setToken(t: string | null) {
  try { t ? localStorage.setItem(KEY, t) : localStorage.removeItem(KEY); } catch { /* private mode */ }
}

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message); }
}

export async function api<T = any>(path: string, opts: { method?: string; body?: any } = {}): Promise<T> {
  const headers: Record<string, string> = {};
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  const tok = getToken();
  if (tok) headers["Authorization"] = "Bearer " + tok;
  const res = await fetch(API + path, {
    method: opts.method || "GET",
    headers,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
  });
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  let data: any = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = null; }
  if (!res.ok) throw new ApiError(res.status, (data && data.error) || res.statusText);
  return data as T;
}

export type House = { id: number; name: string; crest: string; color: string; kind: "house" | "common"; is_steward: number; parcels: string[] };
export type Me = { id: number; name: string; crest: string; color: string; kind: string; is_steward: number; device_id: number };
