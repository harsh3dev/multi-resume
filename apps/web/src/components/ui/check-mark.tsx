import type { SVGProps } from "react";

import { cn } from "@/lib/utils";

/**
 * A plain tick. Iconsax has no simple tick (its `Check` is a different glyph,
 * and the TickSquare/TickCircle icons are framed), so selection indicators
 * (checkbox, select and menu items) draw it directly. Uses currentColor.
 */
export function CheckMark({ size = 12, className, ...props }: { size?: number } & SVGProps<SVGSVGElement>) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={3}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      className={cn("shrink-0", className)}
      {...props}
    >
      <path d="M5 12.5 9.5 17 19 7.5" />
    </svg>
  );
}
