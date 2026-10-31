"use client";

import { useEffect, useState } from "react";

import { apiJSON } from "@/lib/api/client";

export type SupportedFont = { value: string; label: string };

// The allowlist lives in the backend (internal/fonts) and is the single
// source for the Style tab's font select. Fetched once per page load.
let cached: Promise<SupportedFont[]> | null = null;

function loadFonts(): Promise<SupportedFont[]> {
  if (!cached) {
    cached = apiJSON<{ fonts: SupportedFont[] }>("/api/meta/fonts")
      .then((r) => r.fonts)
      .catch(() => {
        cached = null; // allow a retry on the next mount
        return [];
      });
  }
  return cached;
}

/** null while loading; an empty array if the request failed. */
export function useSupportedFonts(): SupportedFont[] | null {
  const [fonts, setFonts] = useState<SupportedFont[] | null>(null);
  useEffect(() => {
    let live = true;
    void loadFonts().then((f) => live && setFonts(f));
    return () => {
      live = false;
    };
  }, []);
  return fonts;
}
