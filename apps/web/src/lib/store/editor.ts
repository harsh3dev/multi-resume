"use client";

import { create } from "zustand";

import { apiJSON } from "@/lib/api/client";
import { applyOptimistic, applyOptimisticAll } from "@/lib/optimistic";
import { toast } from "@/lib/store/toast";
import type { Operation, Resume } from "@/types/generated";

type Status = "clean" | "dirty" | "saving" | "compiling" | "error";

type OpsResponse = {
  revision: number;
  tree: Resume;
  sourceTex?: string;
  pdf?: { key: string; url?: string };
  compileError?: string;
};

type EditorState = {
  draftId: string;
  /** What the UI shows: `confirmedTree` plus optimistic (unconfirmed) reorders. */
  tree: Resume | null;
  /** The last tree the server confirmed; the rollback target if a save fails. */
  confirmedTree: Resume | null;
  /** The saved .tex, kept in step with every edit so the Source tab can follow. */
  sourceTex: string;
  /**
   * Unsaved text typed in the Source tab (null = none). `base` is the saved
   * source it was started from, to detect the draft moving on underneath.
   */
  sourceDraft: { text: string; base: string } | null;
  revision: number;
  pendingOps: Map<string, Operation>;
  lastGoodPdfUrl: string | null;
  compileError: string | null;
  status: Status;

  loadDraft: (draftId: string, tree: Resume, revision: number, sourceTex?: string) => void;
  /** Forget the current draft entirely (used when opening a different one). */
  reset: () => void;
  applyLocal: (op: Operation) => void;
  flush: () => Promise<void>;
  setPdf: (url: string | null) => void;
  refetch: () => Promise<void>;
  setSourceDraft: (text: string | null) => void;
  /**
   * Flushes pending field edits, then saves the Source tab's text (or the
   * current source if untouched), re-parses and recompiles. Resolves with the
   * compile error, if any; rejects on request failure.
   */
  recompile: () => Promise<{ compileError?: string }>;
  /** Pulls only the saved .tex (used when an ops response omits it). */
  refreshSource: () => Promise<void>;
};

let debounceTimer: ReturnType<typeof setTimeout> | null = null;
let inFlight: AbortController | null = null;

/** Cancel anything scheduled or in flight for the previous draft. */
function cancelPending() {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = null;
  if (inFlight) inFlight.abort();
  inFlight = null;
}

export const useEditor = create<EditorState>((set, get) => ({
  draftId: "",
  tree: null,
  confirmedTree: null,
  sourceTex: "",
  sourceDraft: null,
  revision: 0,
  pendingOps: new Map(),
  lastGoodPdfUrl: null,
  compileError: null,
  status: "clean",

  loadDraft: (draftId, tree, revision, sourceTex) => {
    cancelPending();
    set((s) => ({
      draftId,
      tree,
      confirmedTree: tree,
      revision,
      sourceTex: sourceTex ?? s.sourceTex,
      sourceDraft: null,
      pendingOps: new Map(),
      status: "clean",
      compileError: null,
    }));
  },

  reset: () => {
    cancelPending();
    set({
      draftId: "",
      tree: null,
      confirmedTree: null,
      sourceTex: "",
      sourceDraft: null,
      revision: 0,
      pendingOps: new Map(),
      lastGoodPdfUrl: null,
      compileError: null,
      status: "clean",
    });
  },

  applyLocal: (op) => {
    const { pendingOps } = get();
    // Merge per-field edits: latest value wins for the same target.
    const key = "id" in op ? `${op.op}:${op.id}` : `${op.op}:${crypto.randomUUID()}`;
    const next = new Map(pendingOps);
    next.set(key, op);
    // Reorders move on screen right away; the server confirms (or we roll back).
    const tree = get().tree;
    set({
      pendingOps: next,
      status: "dirty",
      ...(tree && op.op === "reorder" ? { tree: applyOptimistic(tree, op) } : {}),
    });
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      void get().flush();
    }, 600);
  },

  flush: async () => {
    const { draftId, pendingOps, revision } = get();
    if (pendingOps.size === 0 || !draftId) return;
    if (inFlight) inFlight.abort(); // newer send cancels the older request
    inFlight = new AbortController();
    const ops = Array.from(pendingOps.values());
    set({ status: "saving", pendingOps: new Map() });
    try {
      const res = await apiJSON<OpsResponse>(`/api/drafts/${draftId}/ops`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ revision, ops }),
        signal: inFlight.signal,
      });
      // The user may have opened another resume while this was in flight.
      if (get().draftId !== draftId) return;
      // Ignore responses older than the shown revision (out-of-order arrival).
      if (res.revision < get().revision) return;
      set({
        confirmedTree: res.tree,
        // Keep reorders made while this request was in flight on screen.
        tree: applyOptimisticAll(res.tree, get().pendingOps.values()),
        revision: res.revision,
        sourceTex: res.sourceTex ?? get().sourceTex,
        compileError: res.compileError ?? null,
        status: res.compileError ? "error" : "clean",
        lastGoodPdfUrl: res.pdf?.url ?? get().lastGoodPdfUrl,
      });
      if (res.sourceTex === undefined) void get().refreshSource().catch(() => {});
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      if (get().draftId !== draftId) return; // not our draft any more: touch nothing
      if (e instanceof Error && e.message === "stale_revision") {
        await get().refetch(); // two-tab guard: another tab saved first
        return;
      }
      // Roll back optimistic moves to the last confirmed tree (re-showing any
      // still-unsent reorders), and say so.
      const confirmed = get().confirmedTree;
      set({
        ...(confirmed ? { tree: applyOptimisticAll(confirmed, get().pendingOps.values()) } : {}),
        status: "error",
        compileError: "save failed — your text is still here, retrying is safe",
      });
      toast.error("Couldn't save your change", "The edit was rolled back. Please try again.");
    }
  },

  setPdf: (url) => set({ lastGoodPdfUrl: url, status: "clean" }),

  setSourceDraft: (text) =>
    set((s) => {
      if (text === null || text === s.sourceTex) return { sourceDraft: null };
      return { sourceDraft: { text, base: s.sourceDraft?.base ?? s.sourceTex } };
    }),

  recompile: async () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    await get().flush(); // field edits first, so they are not lost
    const { draftId, revision, sourceDraft, sourceTex } = get();
    if (!draftId) return {};
    const source = sourceDraft?.text ?? sourceTex;
    set({ status: "compiling" });
    try {
      const res = await apiJSON<{
        revision: number;
        tree: Resume;
        pdf?: { key: string; url?: string };
        compileError?: string;
      }>(`/api/drafts/${draftId}/source`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ source, revision }),
      });
      if (get().draftId !== draftId) return {}; // a different resume is open now
      set({
        tree: res.tree,
        confirmedTree: res.tree,
        revision: res.revision,
        sourceTex: source,
        sourceDraft: null,
        compileError: res.compileError ?? null,
        status: res.compileError ? "error" : "clean",
        lastGoodPdfUrl: res.pdf?.url ?? get().lastGoodPdfUrl,
      });
      return { compileError: res.compileError };
    } catch (e) {
      if (get().draftId === draftId) set({ status: "error" });
      throw e;
    }
  },

  refreshSource: async () => {
    const { draftId, revision } = get();
    if (!draftId) return;
    const res = await apiJSON<{ revision: number; sourceTex?: string }>(`/api/drafts/${draftId}`);
    if (get().draftId !== draftId) return;
    // Only adopt it if no newer save landed meanwhile.
    if (res.sourceTex !== undefined && res.revision === get().revision && res.revision >= revision) {
      set({ sourceTex: res.sourceTex });
    }
  },

  refetch: async () => {
    const { draftId } = get();
    if (!draftId) return;
    const res = await apiJSON<{
      tree: Resume;
      revision: number;
      sourceTex?: string;
      compileError?: string;
      pdfUrl?: string;
    }>(`/api/drafts/${draftId}`);
    if (get().draftId !== draftId) return; // stale response for a draft we left
    set({
      tree: res.tree,
      confirmedTree: res.tree,
      revision: res.revision,
      sourceTex: res.sourceTex ?? get().sourceTex,
      compileError: res.compileError ?? null,
      // Presigned PDF URLs expire; a refetch also renews the link.
      lastGoodPdfUrl: res.pdfUrl ?? get().lastGoodPdfUrl,
    });
  },
}));
