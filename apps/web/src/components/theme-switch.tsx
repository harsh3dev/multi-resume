"use client";

import { Moon, Sun1 } from "@/icons";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";
import { flushSync } from "react-dom";

import { Button } from "@/components/ui/components/actions/button";
import { AppIcon } from "@/icons";

export function ThemeSwitch() {
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  // Avoid hydration mismatch: the server can't know the resolved theme.
  useEffect(() => setMounted(true), []);

  if (!mounted) {
    return <Button variant="ghost" size="icon" aria-hidden disabled />;
  }

  const isDark = resolvedTheme === "dark";

  function toggle(event: React.MouseEvent<HTMLButtonElement>) {
    const next = isDark ? "light" : "dark";
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    // View Transitions API: snapshot the page, switch theme, then reveal the new
    // theme with a blurred circle growing from the top-left (see globals.css).
    // Unsupported browsers and reduced motion just switch instantly.
    if (reduced || typeof document.startViewTransition !== "function") {
      setTheme(next);
      return;
    }
    // Grow the reveal from the button: its centre, and the radius that reaches
    // the farthest corner of the viewport.
    const rect = event.currentTarget.getBoundingClientRect();
    const x = rect.left + rect.width / 2;
    const y = rect.top + rect.height / 2;
    const root = document.documentElement;
    root.style.setProperty("--vt-x", `${x}px`);
    root.style.setProperty("--vt-y", `${y}px`);
    root.style.setProperty(
      "--vt-end",
      `${Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y)) + 56}px`,
    );
    const transition = document.startViewTransition(() => {
      // The new snapshot is taken as soon as this callback settles, so the DOM
      // must already be in the new theme: apply the class now, then let
      // next-themes (flushed synchronously) agree with it.
      document.documentElement.classList.toggle("dark", next === "dark");
      flushSync(() => setTheme(next));
    });
    void transition.finished.finally(() => {
      root.style.removeProperty("--vt-x");
      root.style.removeProperty("--vt-y");
      root.style.removeProperty("--vt-end");
    });
  }

  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label={isDark ? "Switch to light theme" : "Switch to dark theme"}
      title={isDark ? "Switch to light theme" : "Switch to dark theme"}
      onClick={toggle}
    >
      <AppIcon icon={isDark ? Sun1 : Moon} />
    </Button>
  );
}
