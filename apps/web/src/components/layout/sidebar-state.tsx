"use client";

import { SidebarLeft } from "@/icons";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { IconButton } from "@/components/ui/components/actions/icon-button";

export const SIDEBAR_COOKIE = "sidebar_state";
const MOBILE_QUERY = "(max-width: 767px)";

interface SidebarCtx {
  /** Desktop: icon rail instead of the full sidebar. */
  collapsed: boolean;
  /** Mobile: the drawer is open. */
  mobileOpen: boolean;
  setMobileOpen: (open: boolean) => void;
  toggle: () => void;
}

const Ctx = createContext<SidebarCtx | null>(null);

export function useSidebar() {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useSidebar must be used inside <SidebarProvider>");
  return ctx;
}

/**
 * Sidebar state. `defaultCollapsed` comes from a cookie read on the server, so
 * the first paint already has the right width (no expand → collapse flash).
 */
export function SidebarProvider({
  defaultCollapsed,
  children,
}: {
  defaultCollapsed: boolean;
  children: ReactNode;
}) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed);
  const [mobileOpen, setMobileOpen] = useState(false);

  const toggle = useCallback(() => {
    if (window.matchMedia(MOBILE_QUERY).matches) {
      setMobileOpen((o) => !o);
      return;
    }
    setCollapsed((c) => {
      const next = !c;
      document.cookie = `${SIDEBAR_COOKIE}=${next ? "collapsed" : "open"}; path=/; max-age=31536000; SameSite=Lax`;
      return next;
    });
  }, []);

  // ⌘/Ctrl + \ — not ⌘B, which is Bold in the rich-text editor.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "\\") {
        e.preventDefault();
        toggle();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [toggle]);

  const value = useMemo(
    () => ({ collapsed, mobileOpen, setMobileOpen, toggle }),
    [collapsed, mobileOpen, toggle],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function SidebarTrigger() {
  const { toggle, collapsed } = useSidebar();
  return (
    <IconButton
      icon={SidebarLeft}
      label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
      shortcut=" ⌘\"
      onClick={toggle}
    />
  );
}
