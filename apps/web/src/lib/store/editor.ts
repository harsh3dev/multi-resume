"use client";

import { create } from "zustand";

import { apiJSON } from "@/lib/api/client";
import { applyOptimistic, applyOptimisticAll } from "@/lib/store/optimistic";
import { toast } from "@/lib/store/toast";
import type { Operation, Resume } from "@/types/generated";

type Status = "clean" | "dirty" | "saving" | "compiling" | "error";

/** Response of the save routes (ops, source). They never compile. */
type OpsResponse = {
  revision: number;
  tree: Resume;
  sourceTex?: string;
};

/** Response of POST /compile. */
type CompileResponse = {
  revision: number;
  pdfStale: boolean;
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
  /**
   * The shown PDF does not reflect the latest edits. Edits autosave to the
   * server cheaply, but the PDF only refreshes when the user presses Sync PDF
   * (compiling is the expensive step). The UI presents this as "unsynced".
   */
  pdfStale: boolean;
  status: Status;

  loadDraft: (
    draftId: string,
    tree: Resume,
    revision: number,
    sourceTex?: string,
    pdfStale?: boolean,
  ) => void;
  /** Forget the current draft entirely (used when opening a different one). */
  reset: () => void;
  applyLocal: (op: Operation) => void;
  flush: () => Promise<void>;
  setPdf: (url: string | null) => void;
  refetch: () => Promise<void>;
  setSourceDraft: (text: string | null) => void;
  /**
   * The user-facing "Sync PDF": flushes pending field edits, saves the Source
   * tab's text if it was edited, then compiles the saved draft. Resolves with
   * the compile error, if any; rejects on request failure. Concurrent calls
   * share one in-flight run.
   */
  recompile: () => Promise<{ compileError?: string }>;
  /** Pulls only the saved .tex (used when an ops response omits it). */
  refreshSource: () => Promise<void>;
};

let debounceTimer: ReturnType<typeof setTimeout> | null = null;
let inFlight: AbortController | null = null;
let compileRun: Promise<{ compileError?: string }> | null = null;
let compileAbort: AbortController | null = null;

/**
 * True while the user has anything the preview doesn't show yet: pending
 * edits, an unsaved Source tab, or a saved draft whose PDF is out of date.
 */
export const selectNeedsSave = (s: Pick<EditorState, "pdfStale" | "sourceDraft" | "pendingOps">) =>
  s.pdfStale || s.sourceDraft !== null || s.pendingOps.size > 0;

/** Cancel anything scheduled or in flight for the previous draft. */
function cancelPending() {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = null;
  if (inFlight) inFlight.abort();
  inFlight = null;
  if (compileAbort) compileAbort.abort();
  compileAbort = null;
  compileRun = null;
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
  pdfStale: false,
  status: "clean",

  loadDraft: (draftId, tree, revision, sourceTex, pdfStale) => {
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
      pdfStale: pdfStale ?? false,
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
      pdfStale: false,
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
      pdfStale: true,
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
        // Saved, not compiled: the PDF (and any earlier compile error) stays as is.
        pdfStale: true,
        status: "clean",
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

  recompile: () => {
    if (compileRun) return compileRun; // a double click joins the run in flight
    const run = (async () => {
      if (debounceTimer) clearTimeout(debounceTimer);
      await get().flush(); // field edits first, so they are not lost
      const { draftId } = get();
      if (!draftId) return {};
      set({ status: "compiling" });
      compileAbort = new AbortController();
      const signal = compileAbort.signal;
      try {
        // Source-tab text is saved (parsed) first; untouched source needs no request.
        const { sourceDraft, revision } = get();
        if (sourceDraft) {
          const saved = await apiJSON<OpsResponse>(`/api/drafts/${draftId}/source`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ source: sourceDraft.text, revision }),
            signal,
          });
          if (get().draftId !== draftId) return {};
          set({
            tree: saved.tree,
            confirmedTree: saved.tree,
            revision: saved.revision,
            sourceTex: sourceDraft.text,
            sourceDraft: null,
            pdfStale: true,
          });
        }
        const res = await apiJSON<CompileResponse>(`/api/drafts/${draftId}/compile`, {
          method: "POST",
          signal,
        });
        if (get().draftId !== draftId) return {}; // a different resume is open now
        set({
          // A save that landed mid-compile leaves the draft stale on the server.
          pdfStale: res.pdfStale || get().pendingOps.size > 0,
          compileError: res.compileError ?? null,
          status: res.compileError ? "error" : "clean",
          lastGoodPdfUrl: res.pdf?.url ?? get().lastGoodPdfUrl,
        });
        return { compileError: res.compileError };
      } catch (e) {
        // Leaving the draft aborts the request (which also stops the compile).
        if (e instanceof DOMException && e.name === "AbortError") return {};
        if (get().draftId === draftId) set({ status: "error" });
        throw e;
      }
    })();
    compileRun = run;
    const clear = () => {
      if (compileRun === run) {
        compileRun = null;
        compileAbort = null;
      }
    };
    run.then(clear, clear);
    return run;
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
      pdfStale?: boolean;
    }>(`/api/drafts/${draftId}`);
    if (get().draftId !== draftId) return; // stale response for a draft we left
    set({
      tree: res.tree,
      confirmedTree: res.tree,
      revision: res.revision,
      sourceTex: res.sourceTex ?? get().sourceTex,
      compileError: res.compileError ?? null,
      pdfStale: res.pdfStale ?? get().pdfStale,
      // Presigned PDF URLs expire; a refetch also renews the link.
      lastGoodPdfUrl: res.pdfUrl ?? get().lastGoodPdfUrl,
    });
  },
}));
