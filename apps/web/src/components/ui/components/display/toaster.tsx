"use client";

import { AnimatePresence, motion } from "framer-motion";
import { CloseCircle, InfoCircle, TickCircle } from "@/icons";
import { useEffect, useState, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";

import { AppIcon, type IconsaxIcon } from "@/icons";
import { useMotion } from "@/lib/components/motion";
import {
  getToastOriginRect,
  setToastOriginElement,
  useToastStore,
  type ToastItem,
} from "@/lib/store/toast";
import { cn } from "@/lib/utils/utils";

const AUTO_DISMISS_MS = 4500;
const ARROW = 12; // px; matches the arrow's size-3

const TYPE_STYLE: Record<ToastItem["type"], { icon: IconsaxIcon; className: string }> = {
  success: { icon: TickCircle, className: "text-success" },
  error: { icon: CloseCircle, className: "text-destructive" },
  info: { icon: InfoCircle, className: "text-primary" },
};

function ToastCard({
  item,
  reduced,
  onDismiss,
}: {
  item: ToastItem;
  reduced: boolean;
  onDismiss: () => void;
}) {
  // Auto-dismiss; the exit animation morphs the card back toward the bell.
  useEffect(() => {
    const timer = setTimeout(onDismiss, AUTO_DISMISS_MS);
    return () => clearTimeout(timer);
  }, [onDismiss]);

  const { icon, className: typeClass } = TYPE_STYLE[item.type];

  return (
    <motion.div
      layout
      role="status"
      initial={reduced ? { opacity: 0 } : { opacity: 0, scale: 0.4, y: -12 }}
      animate={{ opacity: 1, scale: 1, y: 0 }}
      exit={reduced ? { opacity: 0 } : { opacity: 0, scale: 0.4, y: -12 }}
      transition={{ type: "spring", bounce: 0, duration: 0.25 }}
      style={{ transformOrigin: "top right" }}
      className={cn(
        "pointer-events-auto flex w-full items-start gap-2.5 rounded-xl border border-border bg-popover p-3 text-popover-foreground shadow-md",
        typeClass,
      )}
    >
      <AppIcon icon={icon} size="md" variant="Bulk" className="mt-px" />
      <div className="min-w-0 flex-1">
        <p className="text-xs font-medium text-foreground">{item.title}</p>
        {item.description ? (
          <p className="mt-0.5 text-xs text-muted-foreground">{item.description}</p>
        ) : null}
      </div>
      <button
        type="button"
        aria-label="Dismiss notification"
        onClick={onDismiss}
        className="text-muted-foreground transition-colors hover:text-foreground"
      >
        <AppIcon icon={CloseCircle} size="sm" />
      </button>
    </motion.div>
  );
}

export function Toaster() {
  const toasts = useToastStore((s) => s.toasts);
  const dismiss = useToastStore((s) => s.dismiss);
  const { reduced } = useMotion();
  const [mounted, setMounted] = useState(false);
  const [anchor, setAnchor] = useState<DOMRect | null>(null);

  useEffect(() => setMounted(true), []);

  // Anchor the stack to the notification bell when the first toast appears;
  // fall back to the top-right corner outside the editor.
  useEffect(() => {
    if (toasts.length > 0 && !anchor) {
      setAnchor(getToastOriginRect());
    } else if (toasts.length === 0 && anchor) {
      setAnchor(null);
    }
  }, [toasts, anchor]);

  if (!mounted) return null;

  const style: CSSProperties = anchor
    ? { top: anchor.bottom + 8, right: window.innerWidth - anchor.right }
    : { top: 16, right: 16 };

  // z-[100]: above dialogs and their blurred backdrop (z-50), so a toast fired
  // from inside a dialog is never dimmed or blurred by it.
  return createPortal(
    <div style={style} className="pointer-events-none fixed z-[100] flex w-80 flex-col gap-2">
      {/* Pointer toward the bell: a rotated square whose fill hides the card's
          top border, so it reads as one notch. Centred under the bell. */}
      {anchor && toasts.length > 0 ? (
        <span
          aria-hidden
          style={{ right: anchor.width / 2 - ARROW / 2 }}
          className="absolute -top-[5px] z-10 size-3 rotate-45 border-l border-t border-border bg-popover"
        />
      ) : null}
      <AnimatePresence mode="popLayout">
        {toasts.map((t) => (
          <ToastCard key={t.id} item={t} reduced={reduced} onDismiss={() => dismiss(t.id)} />
        ))}
      </AnimatePresence>
    </div>,
    document.body,
  );
}

/** Wraps the notification bell; toasts morph out of and back into it. */
export function ToastOrigin({ children }: { children: ReactNode }) {
  return (
    <span ref={setToastOriginElement} className="inline-flex">
      {children}
    </span>
  );
}
