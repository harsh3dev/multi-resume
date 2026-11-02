"use client";

import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from "react";

import { Tabs, TabsList, TabsTrigger } from "@/components/ui/components/navigation/tabs";
import { cn } from "@/lib/utils/utils";

const MIN = 25;
const MAX = 75;
const STEP = 5;

function useNarrow(breakpoint = 768) {
  const [narrow, setNarrow] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia(`(max-width: ${breakpoint - 1}px)`);
    const update = () => setNarrow(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, [breakpoint]);
  return narrow;
}

/**
 * Two-pane workspace with a draggable, keyboard-accessible divider. Below
 * `md` it shows one pane at a time behind an Edit / Preview switch.
 */
export function Workspace({
  edit,
  preview,
  divider,
}: {
  edit: ReactNode;
  preview: ReactNode;
  /** Rendered centred on the divider (conversion indicator). */
  divider?: ReactNode;
}) {
  const narrow = useNarrow();
  const [pane, setPane] = useState<"edit" | "preview">("edit");
  const [pct, setPct] = useState(50);
  const [dragging, setDragging] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  const clamp = (v: number) => Math.min(MAX, Math.max(MIN, v));

  const onPointerMove = useCallback((e: PointerEvent<HTMLDivElement>) => {
    if (!dragging || !rootRef.current) return;
    const rect = rootRef.current.getBoundingClientRect();
    setPct(clamp(((e.clientX - rect.left) / rect.width) * 100));
  }, [dragging]);

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (e.key === "ArrowLeft") setPct((p) => clamp(p - STEP));
    else if (e.key === "ArrowRight") setPct((p) => clamp(p + STEP));
    else return;
    e.preventDefault();
  }

  if (narrow) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex shrink-0 justify-center border-b border-border p-2">
          <Tabs value={pane} onValueChange={(v) => setPane(v as "edit" | "preview")}>
            <TabsList>
              <TabsTrigger value="edit" className="px-4">Edit</TabsTrigger>
              <TabsTrigger value="preview" className="px-4">Preview</TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
        {/* Both stay mounted so Source edits and the PDF survive a switch. */}
        <div className={cn("min-h-0 flex-1 overflow-hidden", pane !== "edit" && "hidden")}>{edit}</div>
        <div className={cn("min-h-0 flex-1 overflow-hidden", pane !== "preview" && "hidden")}>{preview}</div>
      </div>
    );
  }

  return (
    <div ref={rootRef} className="relative flex min-h-0 flex-1">
      <div style={{ width: `${pct}%` }} className="min-w-0 overflow-hidden">
        {edit}
      </div>
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize panels"
        aria-valuemin={MIN}
        aria-valuemax={MAX}
        aria-valuenow={Math.round(pct)}
        tabIndex={0}
        onKeyDown={onKeyDown}
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture(e.pointerId);
          setDragging(true);
        }}
        onPointerMove={onPointerMove}
        onPointerUp={() => setDragging(false)}
        onPointerCancel={() => setDragging(false)}
        className={cn(
          "group relative z-10 w-px shrink-0 cursor-col-resize touch-none bg-border outline-none transition-colors",
          "before:absolute before:inset-y-0 before:-left-1.5 before:-right-1.5 before:content-['']",
          "hover:bg-ring focus-visible:bg-ring",
          dragging && "bg-ring",
        )}
      >
        {divider ? (
          <div className="pointer-events-none absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2">
            {divider}
          </div>
        ) : null}
      </div>
      <div className="min-w-0 flex-1 overflow-hidden">{preview}</div>
    </div>
  );
}
