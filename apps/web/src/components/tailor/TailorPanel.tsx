"use client";

import { useMemo, useState } from "react";
import { experimental_useObject as useObject } from "@ai-sdk/react";

import { MagicStar } from "iconsax-react";

import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { AppIcon } from "@/components/ui/icon";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { buildNodeIndex } from "@/lib/node-index";
import { toast } from "@/lib/store/toast";
import { cn } from "@/lib/utils";
import { apiFetch, apiJSON } from "@/lib/api/client";
import { tailorChangesSchema } from "@/lib/schemas/tailor";
import { createClient } from "@/lib/supabase/client";
import { useEditor } from "@/lib/store/editor";
import type { Operation } from "@/types/generated";

type Decision = "pending" | "accepted" | "rejected";

// TailorPanel: paste a JD, stream AI change proposals grouped by the section
// they touch, tick the ones you want, and apply exactly those through the
// normal engine path. Unticked suggestions are recorded as rejected.
export function TailorPanel() {
  const { draftId, revision, tree } = useEditor();
  const [jd, setJd] = useState("");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [applied, setApplied] = useState<Set<number>>(new Set());
  const [runId, setRunId] = useState<string | null>(null);
  const [applying, setApplying] = useState(false);

  const { object, submit, isLoading } = useObject({
    api: `/api/drafts/${draftId}/tailor`,
    schema: tailorChangesSchema,
    headers: async () => {
      const supabase = createClient();
      const {
        data: { session },
      } = await supabase.auth.getSession();
      return { Authorization: `Bearer ${session?.access_token ?? ""}` };
    },
  });

  // Streamed objects are partial until the stream completes (deep-partial).
  type PartialChange = {
    op?: "edit" | "hide" | "show";
    id?: string;
    value?: string;
    why?: string;
    flags?: (string | undefined)[];
  };
  const changes: (PartialChange | undefined)[] = (object?.changes ?? []) as (PartialChange | undefined)[];
  const streamedRunId = (object as { runId?: string } | undefined)?.runId;
  if (streamedRunId && streamedRunId !== runId) setRunId(streamedRunId);

  function toOp(c: PartialChange): Operation | null {
    if (!c.op || !c.id) return null;
    if (c.op === "edit") return { op: "edit", id: c.id, value: c.value ?? "" };
    return { op: c.op, id: c.id };
  }

  // Which section/entry each suggestion touches, from the live tree.
  const nodeIndex = useMemo(() => buildNodeIndex(tree), [tree]);

  // A suggestion can be applied once it is complete (streamed in full).
  const selectable = (i: number) => {
    const c = changes[i];
    return !!c && toOp(c) !== null && !applied.has(i);
  };

  // Group by section, in order of first appearance. Suggestions whose target
  // has not streamed in yet go in a trailing, unlabelled group.
  const groups = useMemo(() => {
    const byKey = new Map<string, { title: string | null; items: number[] }>();
    changes.forEach((c, i) => {
      if (!c) return;
      const info = c.id ? nodeIndex.get(c.id) : undefined;
      const key = !c.id ? "__pending__" : (info?.section ?? "__unknown__");
      const title = key === "__pending__" ? null : key === "__unknown__" ? "Other" : key;
      if (!byKey.has(key)) byKey.set(key, { title, items: [] });
      byKey.get(key)!.items.push(i);
    });
    return [...byKey.values()];
    // `changes` is a fresh array each render; its identity is the stream snapshot
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [object, nodeIndex]);

  function toggle(i: number, on: boolean) {
    setSelected((cur) => {
      const next = new Set(cur);
      if (on) next.add(i);
      else next.delete(i);
      return next;
    });
  }
  const selectableIds = changes.map((_, i) => i).filter(selectable);

  async function applySelected() {
    setApplying(true);
    const ops: Operation[] = [];
    const decisionRows: { index: number; decision: Decision }[] = [];
    changes.forEach((c, i) => {
      if (!c) return;
      const take = selected.has(i) && selectable(i);
      decisionRows.push({ index: i, decision: take || applied.has(i) ? "accepted" : "rejected" });
      if (take) {
        const op = toOp(c);
        if (op) ops.push(op);
      }
    });
    try {
      if (ops.length > 0) {
        await apiJSON(`/api/drafts/${draftId}/ops`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ revision, ops }),
        });
        await useEditor.getState().refetch();
      }
      if (runId) {
        await apiFetch(`/api/tailor-runs/${runId}/decisions`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ decisions: decisionRows }),
        });
      }
      setApplied((cur) => new Set([...cur, ...selected]));
      setSelected(new Set());
      toast.success(
        ops.length > 0 ? "Changes applied" : "Nothing to apply",
        ops.length > 0 ? `${ops.length} ${ops.length === 1 ? "change" : "changes"} applied.` : undefined,
      );
    } catch (e) {
      toast.error("Could not apply changes", e instanceof Error ? e.message : undefined);
    } finally {
      setApplying(false);
    }
  }

  const done = changes.length > 0 && !isLoading;

  return (
    <div className="flex flex-col gap-4 p-4">
      <Field
        label="Job description"
        description="The AI proposes edits that match this job. Tick the ones you want; only those are applied."
      >
        <Textarea
          className="min-h-32"
          placeholder="Paste the job description here…"
          value={jd}
          onChange={(e) => setJd(e.target.value)}
        />
      </Field>
      <Button
        loading={isLoading}
        disabled={jd.trim().length < 20}
        onClick={() => {
          setSelected(new Set());
          setApplied(new Set());
          submit({ jd });
        }}
      >
        <AppIcon icon={MagicStar} />
        Tailor to this job
      </Button>

      {object?.error ? (
        <p role="alert" className="text-xs text-destructive">
          Tailoring failed mid-stream; shown changes are still reviewable.
        </p>
      ) : null}

      {changes.length === 0 && !isLoading ? (
        <EmptyState
          icon={MagicStar}
          title="No suggestions yet"
          description="Paste a job description and tailor to see proposed changes here."
        />
      ) : (
        <div className="flex flex-col gap-5">
          {groups.map((group) => (
            <section key={group.title ?? "__pending__"} className="flex flex-col gap-2">
              {group.title ? (
                <h3 className="flex items-center gap-2 px-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                  {group.title}
                  <span className="font-normal normal-case tracking-normal">
                    · {group.items.length} {group.items.length === 1 ? "change" : "changes"}
                  </span>
                </h3>
              ) : null}
              {group.items.map((i) => {
                const c = changes[i]!;
                const info = c.id ? nodeIndex.get(c.id) : undefined;
                const isSelected = selected.has(i);
                const isApplied = applied.has(i);
                const disabled = isLoading || isApplied || !selectable(i);
                return (
                  <label
                    key={i}
                    className={cn(
                      "flex gap-3 rounded-lg p-3 text-sm transition-colors",
                      disabled ? "cursor-default" : "cursor-pointer",
                      isSelected ? "bg-primary/10" : "bg-card",
                      !disabled && !isSelected && "hover:bg-accent/50",
                      isApplied && "opacity-60",
                    )}
                  >
                    <Checkbox
                      className="mt-0.5"
                      checked={isSelected || isApplied}
                      disabled={disabled}
                      onCheckedChange={(on) => toggle(i, on)}
                      aria-label={`Apply this change${info?.context ? ` to ${info.context}` : ""}`}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="mb-1.5 flex flex-wrap items-center gap-1.5">
                        <Badge variant="secondary">{c.op ?? "…"}</Badge>
                        {info?.context ? (
                          <span className="truncate text-xs text-muted-foreground">{info.context}</span>
                        ) : null}
                        {(c.flags ?? []).map((f, fi) =>
                          f ? (
                            <Badge key={fi} variant={f === "new_claim" ? "destructive" : "warning"}>
                              {f}
                            </Badge>
                          ) : null,
                        )}
                        {isApplied ? <Badge variant="success">Applied</Badge> : null}
                      </div>
                      {c.value ? (
                        <p>{c.value}</p>
                      ) : info?.text ? (
                        // hide/show carry no new text: show what they act on
                        <p className="text-muted-foreground">{info.text}</p>
                      ) : null}
                      {c.why ? <p className="mt-1 text-xs text-muted-foreground">{c.why}</p> : null}
                    </div>
                  </label>
                );
              })}
            </section>
          ))}
          {isLoading ? <Skeleton className="h-20 w-full rounded-lg" aria-busy /> : null}
        </div>
      )}

      {done ? (
        // Stays in view while scrolling a long list of suggestions.
        <div className="sticky bottom-0 -mx-4 -mb-4 flex items-center gap-2 border-t border-border bg-background/95 px-4 py-3 backdrop-blur">
          <span className="mr-auto text-xs text-muted-foreground">
            {selected.size} of {selectableIds.length} selected
          </span>
          <Button
            variant="ghost"
            size="sm"
            disabled={selectableIds.length === 0 || selectableIds.every((i) => selected.has(i))}
            onClick={() => setSelected(new Set(selectableIds))}
          >
            Select all
          </Button>
          <Button variant="ghost" size="sm" disabled={selected.size === 0} onClick={() => setSelected(new Set())}>
            Clear
          </Button>
          <Button size="sm" loading={applying} disabled={selected.size === 0} onClick={() => void applySelected()}>
            Apply {selected.size > 0 ? selected.size : ""} selected
          </Button>
        </div>
      ) : null}
    </div>
  );
}
