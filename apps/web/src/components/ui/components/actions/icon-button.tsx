"use client";

import type { ComponentProps } from "react";

import { Button } from "@/components/ui/components/actions/button";
import { AppIcon, type IconsaxIcon } from "@/icons";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/components/actions/tooltip";

type IconButtonProps = Omit<ComponentProps<typeof Button>, "children" | "aria-label"> & {
  icon: IconsaxIcon;
  /** Accessible name and tooltip text. */
  label: string;
  /** Optional shortcut hint shown in the tooltip, e.g. "⌘B". */
  shortcut?: string;
};

/** Icon-only button: always has an accessible label and a tooltip. */
export function IconButton({
  icon,
  label,
  shortcut,
  variant = "ghost",
  size = "icon-sm",
  ...props
}: IconButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button variant={variant} size={size} aria-label={label} {...props}>
            <AppIcon icon={icon} />
          </Button>
        }
      />
      <TooltipContent>
        {label}
        {shortcut ? <span className="text-background/60">{shortcut}</span> : null}
      </TooltipContent>
    </Tooltip>
  );
}
