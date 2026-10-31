"use client";

import Avvvatars from "avvvatars-react";
import { useLayoutEffect, useRef } from "react";

import { cn } from "@/lib/utils";

/**
 * Deterministic per-user avatar (avvvatars-react): colour and shape (or
 * letter) come from the value, so one email always looks the same.
 *
 * The library styles itself with goober, which injects CSS at runtime. It does
 * render the colours it chose as `color` attributes, so the same look is also
 * applied inline here and never depends on that injection. The wrapper's own
 * size/shape covers the moment before that runs.
 */
export function UserAvatar({
  value,
  variant = "shape",
  letter,
  size = 32,
  radius = 8,
  className,
}: {
  value: string;
  /** "shape": a generated geometric glyph (default). "character": the initial. */
  variant?: "shape" | "character";
  /** Only used by the "character" variant. */
  letter?: string;
  size?: number;
  radius?: number;
  className?: string;
}) {
  const ref = useRef<HTMLSpanElement>(null);

  useLayoutEffect(() => {
    const box = ref.current?.firstElementChild as HTMLElement | null;
    const label = box?.firstElementChild as HTMLElement | null;
    if (!box || !label) return;
    Object.assign(box.style, {
      width: `${size}px`,
      height: `${size}px`,
      borderRadius: `${radius}px`,
      backgroundColor: `#${box.getAttribute("color") ?? "EEEDFD"}`,
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      boxSizing: "border-box",
    });
    const color = `#${label.getAttribute("color") ?? "060A23"}`;
    if (label.tagName === "P") {
      // character variant: the initial
      Object.assign(label.style, {
        margin: "0",
        padding: "0",
        fontSize: `${Math.round(size * 0.375)}px`,
        lineHeight: "0",
        fontWeight: "500",
        textTransform: "uppercase",
        color,
      });
    } else {
      // shape variant: an inline SVG that fills with currentColor
      Object.assign(label.style, {
        display: "inline-flex",
        alignItems: "center",
        verticalAlign: "middle",
        color,
      });
    }
  }, [value, variant, letter, size, radius]);

  return (
    <span
      ref={ref}
      aria-hidden
      style={{ width: size, height: size, borderRadius: radius }}
      className={cn("inline-flex shrink-0 items-center justify-center overflow-hidden bg-muted", className)}
    >
      <Avvvatars value={value} style={variant} displayValue={letter} size={size} radius={radius} />
    </span>
  );
}
