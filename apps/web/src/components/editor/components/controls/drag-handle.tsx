"use client";

import { HambergerMenu } from "@/icons";
import type { ComponentProps } from "react";

import { AppIcon } from "@/icons";
import { cn } from "@/lib/utils/utils";

/**
 * Full-height drag strip for the left edge of a sortable card. The whole
 * strip is the handle (spread dnd-kit's attributes/listeners onto it); the
 * grip icon is only a hint. No tooltip: the grip and cursor say it already;
 * `label` is the accessible name.
 */
export function DragHandle({
  label,
  className,
  ...props
}: ComponentProps<"button"> & { label: string }) {
  return (
    <button
      type="button"
      aria-label={label}
      className={cn(
        "flex w-7 shrink-0 cursor-grab touch-none items-center justify-center self-stretch rounded-l-md text-muted-foreground outline-none transition-colors",
        "hover:bg-muted hover:text-foreground focus-visible:bg-muted focus-visible:text-foreground active:cursor-grabbing",
        className,
      )}
      {...props}
    >
      <AppIcon icon={HambergerMenu} />
    </button>
  );
}
