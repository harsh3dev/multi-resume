import type { ReactNode } from "react";

import { AppIcon, type IconsaxIcon } from "@/components/ui/icon";
import { cn } from "@/lib/utils";

/**
 * Empty state: 56px tinted icon tile, semibold title, muted xs description,
 * optional action. Used for empty lists and panels.
 */
interface EmptyStateProps {
  icon: IconsaxIcon;
  title: string;
  description?: string;
  action?: ReactNode;
  /** "destructive" tints the icon tile for error screens. */
  tone?: "default" | "destructive";
  className?: string;
}

function EmptyState({ icon, title, description, action, tone = "default", className }: EmptyStateProps) {
  return (
    <div
      data-slot="empty-state"
      className={cn("flex flex-col items-center justify-center gap-2 py-12 text-center", className)}
    >
      <div
        className={cn(
          "flex size-14 items-center justify-center rounded-xl",
          tone === "destructive" ? "bg-destructive/10 text-destructive" : "bg-primary/10 text-primary",
        )}
      >
        <AppIcon icon={icon} size="lg" variant="Bulk" />
      </div>
      <p className="mt-1 text-sm font-semibold">{title}</p>
      {description ? (
        <p className="max-w-72 text-xs text-muted-foreground">{description}</p>
      ) : null}
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}

export { EmptyState };
