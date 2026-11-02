"use client";

import { useReducedMotion, type Transition } from "framer-motion";

/** Every animation is ≤250 ms with a bounce-free spring. */
export const SPRING: Transition = { type: "spring", bounce: 0, duration: 0.25 };
export const FADE: Transition = { duration: 0.15, ease: "easeOut" };

/**
 * The one hook gating JS animation. When the user prefers reduced motion,
 * variants collapse to an opacity-only change with no movement or scale.
 */
export function useMotion() {
  const reduced = useReducedMotion() ?? false;
  return { reduced, transition: reduced ? FADE : SPRING };
}
