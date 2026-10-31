"use client";

import { ArrowDown2, ArrowUp2 } from "iconsax-react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { AppIcon } from "@/components/ui/icon";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useSectionsUi } from "@/lib/store/sections-ui";

function Chevrons({ children }: { children: ReactNode }) {
  // Two stacked chevrons with a small gap (overlapping them reads as a diamond/X).
  return <span className="flex flex-col items-center -space-y-1.5">{children}</span>;
}

function ToggleButton({
  label,
  disabled,
  onClick,
  children,
}: {
  label: string;
  disabled: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button variant="ghost" size="icon-sm" aria-label={label} disabled={disabled} onClick={onClick}>
            <Chevrons>{children}</Chevrons>
          </Button>
        }
      />
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

/** Expand all / Collapse all, as double-chevron icon buttons for the tab bar. */
export function SectionToggles() {
  const ids = useSectionsUi((s) => s.ids);
  const open = useSectionsUi((s) => s.open);
  const expandAll = useSectionsUi((s) => s.expandAll);
  const collapseAll = useSectionsUi((s) => s.collapseAll);

  const allOpen = ids.every((id) => open.includes(id));
  return (
    <div className="flex items-center gap-0.5">
      {/* Expand: chevrons point away from each other. Collapse: toward each other. */}
      <ToggleButton label="Expand all" disabled={ids.length === 0 || allOpen} onClick={expandAll}>
        <AppIcon icon={ArrowUp2} size={14} />
        <AppIcon icon={ArrowDown2} size={14} />
      </ToggleButton>
      <ToggleButton label="Collapse all" disabled={open.length === 0} onClick={collapseAll}>
        <AppIcon icon={ArrowDown2} size={14} />
        <AppIcon icon={ArrowUp2} size={14} />
      </ToggleButton>
    </div>
  );
}
