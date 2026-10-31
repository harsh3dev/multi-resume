"use client";

import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Eye, EyeSlash } from "iconsax-react";

import { DragHandle } from "@/components/editor/drag-handle";
import { TextField } from "@/components/editor/fields";
import { IconButton } from "@/components/ui/icon-button";
import { cn } from "@/lib/utils";
import type { Entry, Field } from "@/types/generated";

export function EntryCard({
  entry,
  onEditField,
  onHide,
}: {
  entry: Entry;
  onEditField: (fieldId: string, value: string) => void;
  onHide: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: entry.id });
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn(
        "flex items-stretch rounded-lg bg-card",
        isDragging && "relative z-20 bg-background shadow-lg",
        entry.hidden && "opacity-50",
      )}
    >
      <DragHandle label="Drag to reorder" {...attributes} {...listeners} />
      <div className="grid min-w-0 flex-1 grid-cols-1 gap-2 p-3 sm:grid-cols-2">
        {/* `slot` is a formatting hint (bold/italic/left/right), not a name, so
            fields are labelled by position. Any number of fields is fine. */}
        {entry.fields.map((f: Field, i: number) => (
          <TextField
            key={f.id}
            field={f}
            label={`Field ${i + 1}`}
            onChange={(v) => onEditField(f.id, v)}
          />
        ))}
      </div>
      <div className="flex shrink-0 flex-col items-center justify-start p-1">
        <IconButton
          icon={entry.hidden ? Eye : EyeSlash}
          label={entry.hidden ? "Show entry" : "Hide entry"}
          size="icon-xs"
          onClick={onHide}
        />
      </div>
    </div>
  );
}
