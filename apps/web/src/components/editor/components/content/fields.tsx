"use client";

import { RichText } from "@/components/editor/components/panels/rich-text";
import type { Field } from "@/types/generated";

// TextField edits one tree field through the WYSIWYG editor. The value stays
// mini-markdown (**bold**, *italic*, [text](url)); the store never sees
// TipTap's document.
export function TextField({
  field,
  onChange,
  multiline,
  label = "Text",
  variant,
}: {
  field: Field;
  onChange: (value: string) => void;
  multiline?: boolean;
  label?: string;
  variant?: "box" | "bare";
}) {
  if (field.mode === "raw") {
    return (
      <pre
        className="overflow-auto rounded-lg border border-border bg-muted p-2 font-mono text-xs text-muted-foreground"
        title="Raw LaTeX (read-only)"
      >
        {field.value}
      </pre>
    );
  }
  return <RichText value={field.value} onChange={onChange} multiline={multiline} label={label} variant={variant} />;
}
