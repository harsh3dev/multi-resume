"use client";

import { motion } from "framer-motion";
import type { ReactNode } from "react";

import { useMotion } from "@/lib/motion";

// A template re-mounts on every navigation, giving each page a short fade +
// 10px rise. Reduced motion: opacity only.
export default function AppTemplate({ children }: { children: ReactNode }) {
  const { reduced, transition } = useMotion();
  return (
    <motion.div
      className="h-full"
      initial={{ opacity: 0, y: reduced ? 0 : 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={transition}
    >
      {children}
    </motion.div>
  );
}
