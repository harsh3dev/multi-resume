"use client";

import { useEffect } from "react";

import { initSentry } from "@/lib/components/sentry";

// Mounts Sentry once on app start (no-op without NEXT_PUBLIC_SENTRY_DSN).
export function SentryInit() {
  useEffect(() => {
    void initSentry();
  }, []);
  return null;
}
