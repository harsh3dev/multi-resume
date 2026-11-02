"use client";

import {
  createContext,
  useContext,
  useState,
  type ComponentType,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";

import { AppIcon, type IconsaxIcon } from "@/icons";
import { cn } from "@/lib/utils/utils";

/**
 * The shell owns one header bar. Pages put their title and actions into it
 * with <PageHeader>, which portals into the bar's slot, so there is a single
 * header per screen instead of a shell bar plus a page bar.
 */
const SlotCtx = createContext<HTMLElement | null>(null);
const SetSlotCtx = createContext<((el: HTMLElement | null) => void) | null>(null);

export function HeaderSlotProvider({ children }: { children: ReactNode }) {
  const [el, setEl] = useState<HTMLElement | null>(null);
  return (
    <SetSlotCtx.Provider value={setEl}>
      <SlotCtx.Provider value={el}>{children}</SlotCtx.Provider>
    </SetSlotCtx.Provider>
  );
}

/** Where pages' header content lands. Rendered once, by the shell. */
export function HeaderSlotTarget({ className }: { className?: string }) {
  const setEl = useContext(SetSlotCtx);
  return <div ref={setEl} className={cn("flex min-w-0 flex-1 items-center gap-2", className)} />;
}

export function PageHeader({ children }: { children: ReactNode }) {
  const el = useContext(SlotCtx);
  return el ? createPortal(children, el) : null;
}

/** Gradient icon tile + serif title: the page's h1. */
export function PageTitle({
  icon,
  children,
  className,
}: {
  icon: IconsaxIcon | ComponentType<never>;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex min-w-0 items-center gap-2", className)}>
      <span className="flex shrink-0 items-center justify-center rounded-md bg-gradient-to-br from-light-primary to-primary p-[7px] text-primary-foreground">
        <AppIcon icon={icon as IconsaxIcon} size={14} variant="Bulk" />
      </span>
      <h1 className="truncate font-serif text-xl font-semibold">{children}</h1>
    </div>
  );
}
