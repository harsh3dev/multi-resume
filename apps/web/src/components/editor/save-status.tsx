"use client";

import { Danger, TickCircle, Warning2 } from "iconsax-react";

import { AppIcon } from "@/components/ui/icon";
import { useEditor } from "@/lib/store/editor";
import { cn } from "@/lib/utils";

const STATES = {
  dirty: { label: "Unsaved changes", icon: Warning2, className: "text-warning" },
  clean: { label: "Saved", icon: TickCircle, className: "text-success" },
  error: { label: "Save failed", icon: Danger, className: "text-destructive" },
} as const;

/** Maps the editor store status onto the spec's four save states. */
export function SaveStatus() {
  const status = useEditor((s) => s.status);
  const busy = status === "saving" || status === "compiling";
  const state = busy ? null : STATES[status as keyof typeof STATES];

  return (
    <span
      role="status"
      aria-live="polite"
      data-status={status}
      className={cn(
        "inline-flex items-center gap-1.5 text-xs font-medium",
        busy ? "text-muted-foreground" : state?.className,
      )}
    >
      {busy ? (
        <>
          <span
            aria-hidden
            className="size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent motion-reduce:animate-none"
          />
          Saving…
        </>
      ) : state ? (
        <>
          <AppIcon icon={state.icon} variant="Bulk" />
          {state.label}
        </>
      ) : null}
    </span>
  );
}
