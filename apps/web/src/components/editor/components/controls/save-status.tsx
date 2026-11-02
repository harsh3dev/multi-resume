"use client";

import { Danger, TickCircle, Warning2 } from "@/icons";

import { AppIcon } from "@/icons";
import { selectNeedsSave, useEditor } from "@/lib/store/editor";
import { cn } from "@/lib/utils/utils";

const STATES = {
  dirty: { label: "Unsynced changes", icon: Warning2, className: "text-warning" },
  clean: { label: "Synced", icon: TickCircle, className: "text-success" },
  error: { label: "Sync failed", icon: Danger, className: "text-destructive" },
} as const;

/**
 * What the user sees. Internally edits autosave and only the PDF can be out of
 * date; for the user a preview that doesn't match their edits is "unsynced",
 * and it clears when they press Sync PDF.
 */
export function SaveStatus() {
  const status = useEditor((s) => s.status);
  const needsSave = useEditor(selectNeedsSave);
  const busy = status === "saving" || status === "compiling";
  const key = status === "error" ? "error" : needsSave ? "dirty" : "clean";
  const state = busy ? null : STATES[key];

  return (
    <span
      role="status"
      aria-live="polite"
      data-status={busy ? status : key}
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 text-xs font-medium",
        busy ? "text-muted-foreground" : state?.className,
      )}
    >
      {busy ? (
        <>
          <span
            aria-hidden
            className="size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent motion-reduce:animate-none"
          />
          <span className="max-sm:sr-only">{status === "compiling" ? "Syncing…" : "Saving…"}</span>
        </>
      ) : state ? (
        <>
          <AppIcon icon={state.icon} variant="Bulk" />
          <span className="max-sm:sr-only">{state.label}</span>
        </>
      ) : null}
    </span>
  );
}
