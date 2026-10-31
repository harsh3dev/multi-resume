import { create } from "zustand";

/**
 * Toast store. Toasts are created with the `toast` helper and rendered by
 * <Toaster />, morphing out of the element registered via
 * setToastOriginElement (the notification bell in the editor header).
 */

export type ToastType = "success" | "error" | "info";

export interface ToastItem {
  id: number;
  type: ToastType;
  title: string;
  description?: string;
}

interface ToastState {
  toasts: ToastItem[];
  push: (t: Omit<ToastItem, "id">) => void;
  dismiss: (id: number) => void;
}

const MAX_VISIBLE = 3;
let nextId = 1;

export const useToastStore = create<ToastState>((set) => ({
  toasts: [],
  push: (t) =>
    set((s) => ({
      toasts: [...s.toasts, { ...t, id: nextId++ }].slice(-MAX_VISIBLE),
    })),
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}));

function push(type: ToastType, title: string, description?: string) {
  useToastStore.getState().push({ type, title, description });
}

export const toast = {
  success: (title: string, description?: string) => push("success", title, description),
  error: (title: string, description?: string) => push("error", title, description),
  info: (title: string, description?: string) => push("info", title, description),
};

// --- origin element (notification bell) ---

let originElement: HTMLElement | null = null;

/** Called by <ToastOrigin />; toasts expand from this element. */
export function setToastOriginElement(el: HTMLElement | null) {
  originElement = el;
}

export function getToastOriginRect(): DOMRect | null {
  return originElement?.getBoundingClientRect() ?? null;
}
