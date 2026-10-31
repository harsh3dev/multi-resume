import { ArrowRight2 } from "iconsax-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { AppIcon } from "@/components/ui/icon";
import { cn } from "@/lib/utils";

export interface Crumb {
  label: string;
  /** Omit on the current page (the last crumb). */
  href?: string;
  /** Replaces the label, e.g. with a dropdown that switches to a sibling. */
  node?: ReactNode;
}

/** Breadcrumb trail: links for ancestors, plain text for the current page. */
export function Breadcrumbs({ items, className }: { items: Crumb[]; className?: string }) {
  return (
    <nav aria-label="Breadcrumb" className={cn("min-w-0", className)}>
      <ol className="flex min-w-0 items-center gap-1 text-sm">
        {items.map((item, i) => {
          const last = i === items.length - 1;
          return (
            <li
              key={`${i}-${item.label}`}
              aria-current={last ? "page" : undefined}
              className={cn("flex items-center gap-1", last ? "min-w-0" : "shrink-0")}
            >
              {item.node ? (
                item.node
              ) : item.href && !last ? (
                <Link
                  href={item.href}
                  className="rounded-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
                >
                  {item.label}
                </Link>
              ) : (
                <span aria-current={last ? "page" : undefined} className={cn("truncate", last ? "font-medium text-foreground" : "text-muted-foreground")}>
                  {item.label}
                </span>
              )}
              {last ? null : <AppIcon icon={ArrowRight2} size={14} className="text-muted-foreground/60" />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
