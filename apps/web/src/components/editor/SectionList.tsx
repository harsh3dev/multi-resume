"use client";

import { DndContext, MeasuringStrategy, type DragEndEvent, type DragStartEvent } from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { DocumentText, Eye, EyeSlash } from "iconsax-react";
import { useLayoutEffect, useRef, useState, type ReactNode } from "react";

import { BulletItem } from "@/components/editor/BulletItem";
import { getBulletActions } from "@/components/editor/bullet-actions";
import { EntryCard } from "@/components/editor/EntryCard";
import { AddBulletRow } from "@/components/editor/add-bullet-row";
import { DragHandle } from "@/components/editor/drag-handle";
import { TextField } from "@/components/editor/fields";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { EmptyState } from "@/components/ui/empty-state";
import { IconButton } from "@/components/ui/icon-button";
import { cn } from "@/lib/utils";
import { useEditor } from "@/lib/store/editor";
import { collapsibleIds, HEADER_ID, useSectionsUi } from "@/lib/store/sections-ui";
import type { BulletList, Header, Section, SectionNode } from "@/types/generated";

// SectionList renders all sections as collapsible accordions with
// drag-to-reorder. Drops emit ops via the editor store; the server is the
// source of truth after each flush.
export function SectionList({ sections, header }: { sections: Section[]; header: Header | null }) {
  const { applyLocal } = useEditor();
  const hasHeader = !!header && (!!header.name || header.items.length > 0);
  const allIds = collapsibleIds(header, sections);
  // Open state lives in a store so Expand/Collapse can sit in the tab bar.
  const open = useSectionsUi((s) => s.open);
  const setOpen = useSectionsUi((s) => s.setOpen);
  const idsKey = allIds.join("|");
  useLayoutEffect(() => {
    useSectionsUi.getState().setIds(allIds);
    // allIds is derived from idsKey
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [idsKey]);

  // A section being dragged collapses (so a tall one is easy to move) and
  // re-opens on drop or cancel, if it was open.
  const reopen = useRef<string | null>(null);
  function onSectionDragStart(e: DragStartEvent) {
    const id = String(e.active.id);
    if (!open.includes(id)) return;
    reopen.current = id;
    setOpen((o) => o.filter((x) => x !== id));
  }
  function restoreOpen() {
    const id = reopen.current;
    reopen.current = null;
    if (id) setOpen((o) => (o.includes(id) ? o : [...o, id]));
  }

  function onSectionDragEnd(e: DragEndEvent) {
    restoreOpen();
    const { active, over } = e;
    if (!over || active.id === over.id) return;
    const order = sections.map((s) => s.id);
    const from = order.indexOf(String(active.id));
    const to = order.indexOf(String(over.id));
    if (from < 0 || to < 0) return;
    order.splice(to, 0, order.splice(from, 1)[0]);
    applyLocal({ op: "reorder", parentId: "root", order });
  }

  // Templates vary: some yield only a header, or nothing we can edit.
  if (!hasHeader && sections.length === 0) {
    return (
      <EmptyState
        icon={DocumentText}
        title="Nothing to edit here yet"
        description="No editable fields were detected in this template. You can still change it in the Source tab."
      />
    );
  }

  return (
    <div className="flex flex-col gap-1 p-2">
      <Accordion multiple value={open} onValueChange={(v) => setOpen(v as string[])} className="gap-3">
        {hasHeader && header ? <HeaderView header={header} /> : null}
        {/* Always re-measure: the dragged section shrinks while the drag runs. */}
        <DndContext
          measuring={{ droppable: { strategy: MeasuringStrategy.Always } }}
          onDragStart={onSectionDragStart}
          onDragEnd={onSectionDragEnd}
          onDragCancel={restoreOpen}
        >
          <SortableContext items={sections.map((s) => s.id)} strategy={verticalListSortingStrategy}>
            {sections.map((s) => (
              <SectionView key={s.id} section={s} />
            ))}
          </SortableContext>
        </DndContext>
      </Accordion>
      {sections.length === 0 ? (
        <p className="px-1 text-xs text-muted-foreground">
          Only the header was detected in this template. The rest can be edited in the Source tab.
        </p>
      ) : null}
    </div>
  );
}

// HeaderView edits the resume header (name + contact line items). Fields go
// through the same `edit` op as everything else, addressed by field id.
function HeaderView({ header }: { header: Header }) {
  const { applyLocal } = useEditor();
  return (
    <div className="border-b border-border pb-3">
      <AccordionItem value={HEADER_ID} className="border-0">
        <AccordionTrigger className="items-center rounded-md px-3 py-3 transition-colors hover:bg-accent/60 hover:no-underline">
          <span className="flex-1 text-left">Header</span>
        </AccordionTrigger>
        <AccordionContent>
          <div className="flex flex-col gap-2">
            {header.name ? (
              <TextField
                field={header.name}
                label="Name"
                onChange={(v) => applyLocal({ op: "edit", id: header.name!.id, value: v })}
              />
            ) : null}
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              {header.items.map((f, i) => (
                <TextField
                  key={f.id}
                  field={f}
                  label={f.slot ? String(f.slot) : `Contact ${i + 1}`}
                  onChange={(v) => applyLocal({ op: "edit", id: f.id, value: v })}
                />
              ))}
            </div>
          </div>
        </AccordionContent>
      </AccordionItem>
    </div>
  );
}

function SectionView({ section }: { section: Section }) {
  const { applyLocal } = useEditor();
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: section.id });
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn(
        "border-b border-border pb-3",
        isDragging && "relative z-20 rounded-lg border-b-0 bg-background shadow-lg",
        section.hidden && "opacity-60",
      )}
    >
      <AccordionItem value={section.id} className="border-0">
        <div className="relative flex items-stretch rounded-md transition-colors hover:bg-accent/60">
          <DragHandle label="Drag section to reorder" {...attributes} {...listeners} />
          {/* Trigger fills the row (chevron far right); the hide button sits
              just left of the chevron, so the title gets right padding. */}
          <AccordionTrigger className="flex-1 items-center py-3 pl-3 pr-2 hover:no-underline">
            <span className="min-w-0 flex-1 truncate pr-10 text-left">{section.title.value || "Untitled section"}</span>
          </AccordionTrigger>
          <div className="absolute right-10 top-1/2 -translate-y-1/2">
            <IconButton
              icon={section.hidden ? Eye : EyeSlash}
              label={section.hidden ? "Show section" : "Hide section"}
              size="icon-xs"
              onClick={() => applyLocal({ op: section.hidden ? "show" : "hide", id: section.id })}
            />
          </div>
        </div>
        <AccordionContent>
          <div className="flex flex-col gap-3">
            <TextField
              field={section.title}
              label="Section title"
              onChange={(v) => applyLocal({ op: "edit", id: section.title.id, value: v })}
            />
            <div className="flex flex-col gap-2">
              {section.items.map((item) => (
                <ItemView key={itemID(item)} item={item} />
              ))}
            </div>
          </div>
        </AccordionContent>
      </AccordionItem>
    </div>
  );
}

function itemID(item: SectionNode): string {
  return item.id;
}

function ItemView({ item }: { item: SectionNode }) {
  const { applyLocal } = useEditor();
  switch (item.kind) {
    case "bullets":
      return <BulletListView list={item} />;
    case "entry":
      return (
        <div className="flex flex-col">
          <EntryCard
            entry={item}
            onEditField={(fieldId, v) => applyLocal({ op: "edit", id: fieldId, value: v })}
            onHide={() => applyLocal({ op: item.hidden ? "show" : "hide", id: item.id })}
          />
          {/* The entry's bullets hang under it as a tree: each row has a dashed
              guide and connector (see TreeRow), starting at the entry card. */}
          {item.bullets ? (
            <div className={cn("ml-3", item.hidden && "opacity-50")}>
              <BulletListView list={item.bullets} nested />
            </div>
          ) : null}
        </div>
      );
    case "skill":
      return (
        <div className="flex items-center gap-2 rounded-lg border border-border p-2">
          <div className="w-40">
            <TextField
              field={item.label}
              label="Skill label"
              onChange={(v) => applyLocal({ op: "edit", id: item.label.id, value: v })}
            />
          </div>
          <div className="flex-1">
            <TextField
              field={item.value}
              label="Skill value"
              onChange={(v) => applyLocal({ op: "edit", id: item.value.id, value: v })}
            />
          </div>
        </div>
      );
    case "paragraph":
      return (
        <TextField
          field={item.text}
          multiline
          label="Paragraph"
          onChange={(v) => applyLocal({ op: "edit", id: item.text.id, value: v })}
        />
      );
    case "opaque":
      return (
        <pre className="rounded-lg border border-border bg-muted p-2 font-mono text-xs text-muted-foreground">
          Read-only region ({item.reason})
        </pre>
      );
    default:
      // A node kind this UI doesn't know (e.g. from a newer backend): show it
      // as read-only instead of silently dropping content.
      return (
        <pre className="rounded-lg border border-border bg-muted p-2 font-mono text-xs text-muted-foreground">
          Read-only region (unsupported content)
        </pre>
      );
  }
}

/**
 * One row of an entry's bullet "tree": a dashed vertical guide segment, plus a
 * short dashed connector into the bullet. The last bullet's guide stops at its
 * connector (an L shape) instead of running on. Outside an entry (`nested`
 * false) rows render bare.
 */
function TreeRow({
  nested,
  guide,
  connector = false,
  children,
}: {
  nested: boolean;
  /** "full": runs the row's whole height; "to-connector": stops at the connector. */
  guide: "full" | "to-connector";
  connector?: boolean;
  children: ReactNode;
}) {
  if (!nested) return <>{children}</>;
  return (
    <div className="relative pl-2.5">
      <span
        aria-hidden
        className={cn(
          "pointer-events-none absolute top-0 left-0 border-l border-dashed border-muted-foreground/40",
          guide === "full" ? "bottom-0" : "h-5",
        )}
      />
      {connector ? (
        <span
          aria-hidden
          className="pointer-events-none absolute top-5 left-0 w-2.5 border-t border-dashed border-muted-foreground/40"
        />
      ) : null}
      {children}
    </div>
  );
}

export function BulletListView({ list, nested = false }: { list: BulletList; nested?: boolean }) {
  const { applyLocal } = useEditor();
  // Which block the pointer is over: a bullet or an insertion row. Insertion
  // row k sits before bullet k (row n is after the last bullet), so hovering
  // bullet i reveals rows i and i+1.
  const [hot, setHot] = useState<{ kind: "bullet" | "row"; index: number } | null>(null);
  const rowVisible = (k: number) =>
    hot !== null && (hot.kind === "row" ? hot.index === k : hot.index === k || hot.index === k - 1);
  function onDragEnd(e: DragEndEvent) {
    const { active, over } = e;
    if (!over || active.id === over.id) return;
    const order = list.bullets.map((b) => b.id);
    const from = order.indexOf(String(active.id));
    const to = order.indexOf(String(over.id));
    if (from < 0 || to < 0) return;
    order.splice(to, 0, order.splice(from, 1)[0]);
    applyLocal({ op: "reorder", parentId: list.id, order });
  }
  return (
    <DndContext onDragEnd={onDragEnd}>
      <SortableContext items={list.bullets.map((b) => b.id)} strategy={verticalListSortingStrategy}>
        <div className="flex flex-col" onMouseLeave={() => setHot(null)}>
          <TreeRow nested={nested} guide={list.bullets.length === 0 ? "to-connector" : "full"}>
            <AddBulletRow
              alwaysVisible={list.bullets.length === 0}
              visible={rowVisible(0)}
              onHover={(h) => setHot(h ? { kind: "row", index: 0 } : null)}
              label={list.bullets.length === 0 ? "Add first bullet" : "Add bullet at start"}
              onAdd={() => applyLocal({ op: "add", listId: list.id, after: null, value: "New bullet" })}
            />
          </TreeRow>
          {list.bullets.map((b, i) => (
            <TreeRow
              key={b.id}
              nested={nested}
              connector
              guide={i === list.bullets.length - 1 ? "to-connector" : "full"}
            >
              <div onMouseEnter={() => setHot({ kind: "bullet", index: i })}>
                <BulletItem
                  bullet={b}
                  onEdit={(v) => applyLocal({ op: "edit", id: b.id, value: v })}
                  actions={getBulletActions(b, {
                    hide: () => applyLocal({ op: b.hidden ? "show" : "hide", id: b.id }),
                    remove: () => applyLocal({ op: "delete", id: b.id }),
                  })}
                />
              </div>
              <AddBulletRow
                label="Add bullet after this one"
                visible={rowVisible(i + 1)}
                onHover={(h) => setHot(h ? { kind: "row", index: i + 1 } : null)}
                onAdd={() => applyLocal({ op: "add", listId: list.id, after: b.id, value: "New bullet" })}
              />
            </TreeRow>
          ))}
        </div>
      </SortableContext>
    </DndContext>
  );
}
