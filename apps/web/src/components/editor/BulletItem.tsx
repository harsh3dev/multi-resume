"use client";

import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";

import type { BulletAction } from "@/components/editor/bullet-actions";
import { DragHandle } from "@/components/editor/drag-handle";
import { TextField } from "@/components/editor/fields";
import { IconButton } from "@/components/ui/icon-button";
import { cn } from "@/lib/utils";
import type { Bullet } from "@/types/generated";

/**
 * A bullet card plus its action bar. The bar is a separate, full-width piece
 * tucked behind the card: while the bullet has focus (typing, or using the
 * bar's own buttons) it slides down out from underneath, bottom corners
 * rounded. Actions come from `actions`, laid out by their `align`.
 */
export function BulletItem({
  bullet,
  onEdit,
  actions,
}: {
  bullet: Bullet;
  onEdit: (value: string) => void;
  actions: BulletAction[];
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: bullet.id,
  });
  const left = actions.filter((a) => a.align === "left");
  const right = actions.filter((a) => a.align === "right");

  return (
    // The wrapper is the sortable node and the focus scope: card and bar move
    // together, and either one holding focus keeps the bar open.
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn("group/bullet relative", isDragging && "z-20", bullet.hidden && "opacity-50")}
    >
      <div
        className={cn(
          "relative z-10 flex items-stretch rounded-lg border border-transparent bg-card transition-colors",
          // Focused. Dark: the lighter field-focus tone with a sunken inner shadow.
          // Light: the field tone, lifted by a soft outer shadow (an inner shadow
          // blurs its top edge and turns it grey-on-grey against the bar below).
          "group-focus-within/bullet:bg-field group-focus-within/bullet:shadow-sm dark:group-focus-within/bullet:bg-field-focus dark:group-focus-within/bullet:shadow-none dark:group-focus-within/bullet:inset-shadow-track",
          isDragging && "shadow-lg",
        )}
      >
        <DragHandle label="Drag to reorder" {...attributes} {...listeners} />
        <div className="min-w-0 flex-1 px-1 py-2">
          {bullet.hidden ? (
            <span className="text-xs text-muted-foreground">Hidden — in the bullet bank</span>
          ) : null}
          <BulletText bullet={bullet} onEdit={onEdit} />
        </div>
      </div>

      {/* Behind the card (z-0). Rows animate 0fr -> 1fr for the height; the
          bar itself slides from -100% so it appears to come from behind. */}
      <div className="relative z-0 grid grid-rows-[0fr] transition-[grid-template-rows,margin] duration-200 ease-out group-focus-within/bullet:-mt-2 group-focus-within/bullet:grid-rows-[1fr] motion-reduce:transition-none">
        <div className="overflow-hidden">
          <div className="-translate-y-full transition-transform duration-200 ease-out group-focus-within/bullet:translate-y-0 motion-reduce:transition-none">
            <div className="flex items-center justify-between gap-1 rounded-b-lg border border-t-0 border-border bg-chin px-1.5 pt-3 pb-1 inset-shadow-chin">
              <ActionGroup actions={left} />
              <ActionGroup actions={right} />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function ActionGroup({ actions }: { actions: BulletAction[] }) {
  return (
    <div className="flex items-center gap-0.5">
      {actions.map((a) => (
        <IconButton
          key={a.id}
          icon={a.icon}
          label={a.label}
          size="icon-xs"
          className={cn(a.tone === "destructive" && "text-destructive hover:text-destructive")}
          // Keep the bullet focused while the button is pressed.
          onMouseDown={(e) => e.preventDefault()}
          onClick={a.run}
        />
      ))}
    </div>
  );
}

function BulletText({ bullet, onEdit }: { bullet: Bullet; onEdit: (v: string) => void }) {
  // TextField expects a Field; bullets carry a Text (no id/slot).
  const field = { id: bullet.id, value: bullet.text.value, mode: bullet.text.mode, span: bullet.text.span, url: bullet.text.url };
  return <TextField field={field} multiline variant="bare" label="Bullet text" onChange={onEdit} />;
}
