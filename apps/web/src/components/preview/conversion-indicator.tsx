"use client";

import { AnimatePresence, motion } from "framer-motion";

import { useMotion } from "@/lib/motion";
import { useEditor } from "@/lib/store/editor";

// A two-way funnel: streaks race in a straight line through the wide mouth on
// the left, through the narrow waist and out the right mouth. They are grey on
// the input side and turn teal after the waist: source in, rendered output out.
// Many thin lanes across the waist, two streaks per lane, so lines are always
// pouring through. Each lane has a constant y; length varies per lane.
const LANE_COUNT = 9;
const LANES = Array.from({ length: LANE_COUNT }, (_, i) => 15.9 + i * 1.025);
const DELAYS = [0, 0.19, 0.08, 0.33, 0.14, 0.27, 0.04, 0.23, 0.37] as const; // uneven
const CYCLE = 0.42; // seconds to cross the whole funnel: very fast
const PHASES = [0, CYCLE / 2] as const; // two streaks in flight per lane
const STREAK_H = 1;
const streakLength = (lane: number) => 6 + (lane % 3) * 3;
const WAIST_X = [33, 43] as const;
const SPLIT_X = 38; // colour changes here
const VIEW_W = 76;

function Funnel({ animate }: { animate: boolean }) {
  return (
    <svg viewBox="0 3 76 34" width={46} height={21} aria-hidden className="overflow-visible">
      <defs>
        {/* The same moving streak is drawn twice, clipped to each half. */}
        <clipPath id="funnel-in">
          <rect x={0} y={0} width={SPLIT_X} height={40} />
        </clipPath>
        <clipPath id="funnel-out">
          <rect x={SPLIT_X} y={0} width={VIEW_W - SPLIT_X} height={40} />
        </clipPath>
      </defs>

      {/* Left cone (input), right cone (output), and the waist between them. */}
      <path d="M2 5 L33 16 L33 24 L2 35 Z" className="fill-background stroke-border" strokeWidth={1} strokeLinejoin="round" />
      <path d="M74 5 L43 16 L43 24 L74 35 Z" className="fill-background stroke-border" strokeWidth={1} strokeLinejoin="round" />
      {animate ? (
        <motion.rect
          x={WAIST_X[0]}
          y={15}
          width={WAIST_X[1] - WAIST_X[0]}
          height={10}
          rx={2}
          className="fill-primary/25 stroke-primary"
          strokeWidth={1}
          animate={{ opacity: [0.55, 1, 0.55] }}
          transition={{ duration: CYCLE, repeat: Infinity, ease: "easeInOut" }}
        />
      ) : (
        <rect x={WAIST_X[0]} y={15} width={WAIST_X[1] - WAIST_X[0]} height={10} rx={2} className="fill-primary/25 stroke-primary" strokeWidth={1} />
      )}

      {LANES.map((y, i) => {
        const w = streakLength(i);
        return animate ? (
          <g key={i}>
            {PHASES.map((phase) =>
              (["in", "out"] as const).map((side) => (
                <g key={`${phase}-${side}`} clipPath={`url(#funnel-${side})`}>
                  <motion.rect
                    y={y - STREAK_H / 2}
                    width={w}
                    height={STREAK_H}
                    rx={STREAK_H / 2}
                    className={side === "in" ? "fill-muted-foreground" : "fill-primary"}
                    initial={{ x: -w, opacity: 0 }}
                    // Straight and linear: constant y, constant speed, end to end.
                    animate={{ x: [-w, VIEW_W], opacity: [0, 1, 1, 0] }}
                    transition={{
                      duration: CYCLE,
                      times: [0, 0.1, 0.9, 1],
                      repeat: Infinity,
                      ease: "linear",
                      delay: DELAYS[i] + phase,
                    }}
                  />
                </g>
              )),
            )}
          </g>
        ) : (
          <g key={i}>
            <rect x={10 + (i % 4) * 5} y={y - STREAK_H / 2} width={w} height={STREAK_H} rx={STREAK_H / 2} className="fill-muted-foreground" />
            <rect x={46 + (i % 4) * 5} y={y - STREAK_H / 2} width={w} height={STREAK_H} rx={STREAK_H / 2} className="fill-primary" />
          </g>
        );
      })}
    </svg>
  );
}

/**
 * Shown centred on the workspace divider while the PDF regenerates.
 * Reduced motion: the same funnel, static.
 */
export function ConversionIndicator() {
  const status = useEditor((s) => s.status);
  const { reduced } = useMotion();
  const active = status === "saving" || status === "compiling";

  return (
    <AnimatePresence>
      {active ? (
        <motion.div
          key="conversion"
          role="status"
          aria-label="Updating preview"
          initial={{ opacity: 0, scale: 0.85 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.85 }}
          transition={{ type: "spring", bounce: 0, duration: 0.2 }}
          className="rounded-lg border border-border bg-muted px-1.5 py-1 shadow-md"
        >
          <Funnel animate={!reduced} />
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
