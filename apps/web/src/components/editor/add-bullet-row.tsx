"use client";

import { Add } from "iconsax-react";

import { AppIcon } from "@/components/ui/icon";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

/**
 * Insertion point between bullets (and before the first / after the last).
 * `visible` is driven by the list: it is true while the pointer is over a
 * neighbouring bullet or the row itself, so the + shows above and below any
 * hovered block. Always shown on touch devices and for keyboard focus.
 */
export function AddBulletRow({
  onAdd,
  visible,
  onHover,
  alwaysVisible = false,
  label = "Add bullet here",
}: {
  onAdd: () => void;
  visible: boolean;
  onHover: (hovering: boolean) => void;
  alwaysVisible?: boolean;
  label?: string;
}) {
  const show = visible || alwaysVisible;
  return (
    <div
      onMouseEnter={() => onHover(true)}
      onMouseLeave={() => onHover(false)}
      className={cn("group/add relative flex items-center justify-center", alwaysVisible ? "h-10" : "h-4")}
    >
      <div
        aria-hidden
        className={cn(
          "pointer-events-none absolute inset-x-0 top-1/2 h-px -translate-y-1/2 bg-border opacity-0 transition-opacity",
          "group-focus-within/add:opacity-100 pointer-coarse:opacity-100",
          show && "opacity-100",
        )}
      />
      <Tooltip>
        <TooltipTrigger
          render={
            <button
              type="button"
              aria-label={label}
              onClick={onAdd}
              className={cn(
                // z-30: bullet cards are z-10, and the button overlaps their edges
                // (it is taller than the 16px row), so it must sit above them or it is clipped.
                "relative z-30 flex size-7 items-center justify-center rounded-full border border-border bg-background text-muted-foreground opacity-0 shadow-sm outline-none transition-[opacity,colors]",
                "hover:border-primary hover:bg-primary hover:text-primary-foreground focus-visible:opacity-100 focus-visible:ring-3 focus-visible:ring-ring/50 pointer-coarse:opacity-100",
                show && "opacity-100",
              )}
            >
              <AppIcon icon={Add} size="md" />
            </button>
          }
        />
        <TooltipContent>{label}</TooltipContent>
      </Tooltip>
    </div>
  );
}
