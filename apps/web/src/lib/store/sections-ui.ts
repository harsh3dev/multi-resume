import { create } from "zustand";

import type { Header, Section } from "@/types/generated";

/**
 * Which Fields-tab sections are open. Lives in a store (not in SectionList) so
 * the Expand/Collapse controls can sit in the tab bar, outside the list, and
 * so the open state survives switching tabs.
 */
interface SectionsUi {
  /** Every collapsible item (header + sections) currently in the tree. */
  ids: string[];
  open: string[];
  /** Sync with the tree: new ids start open, vanished ids are forgotten. */
  setIds: (ids: string[]) => void;
  setOpen: (next: string[] | ((open: string[]) => string[])) => void;
  expandAll: () => void;
  collapseAll: () => void;
}

export const HEADER_ID = "__header__";

/** The collapsible items for a tree: the header (if it has anything) + sections. */
export function collapsibleIds(header: Header | null, sections: Section[]): string[] {
  const hasHeader = !!header && (!!header.name || header.items.length > 0);
  return [...(hasHeader ? [HEADER_ID] : []), ...sections.map((s) => s.id)];
}

export const useSectionsUi = create<SectionsUi>((set) => ({
  ids: [],
  open: [],
  setIds: (ids) =>
    set((s) => {
      if (ids.length === s.ids.length && ids.every((id, i) => id === s.ids[i])) return s;
      const known = new Set(s.ids);
      return {
        ids,
        open: [...s.open.filter((id) => ids.includes(id)), ...ids.filter((id) => !known.has(id))],
      };
    }),
  setOpen: (next) => set((s) => ({ open: typeof next === "function" ? next(s.open) : next })),
  expandAll: () => set((s) => ({ open: s.ids })),
  collapseAll: () => set({ open: [] }),
}));
