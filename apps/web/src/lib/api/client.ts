"use client";

import { createClient } from "@/lib/supabase/client";

// Typed fetch wrapper: attaches the Supabase access token to every Go API
// call. On 401 the session is dead — send the user back to login.
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const supabase = createClient();
  const {
    data: { session },
  } = await supabase.auth.getSession();
  const headers = new Headers(init.headers);
  if (session?.access_token) {
    headers.set("Authorization", `Bearer ${session.access_token}`);
  }
  const res = await fetch(path, { ...init, headers });
  if (res.status === 401) {
    window.location.href = "/login";
  }
  return res;
}

export async function apiJSON<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await apiFetch(path, init);
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new Error(body?.error ?? `request failed (${res.status})`);
  }
  return res.json() as Promise<T>;
}
