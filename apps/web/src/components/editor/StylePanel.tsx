"use client";

import { useEffect, useState } from "react";

import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { useSupportedFonts } from "@/lib/fonts";
import { useEditor } from "@/lib/store/editor";
import type { StyleSetting } from "@/types/generated";

// StylePanel renders only the settings the parser found in the file.
export function StylePanel({ styles }: { styles: StyleSetting[] }) {
  if (styles.length === 0) {
    return (
      <p className="p-4 text-sm text-muted-foreground">
        No editable style settings were detected in this template.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-4 p-4">
      {styles.map((s) =>
        s.key === "fontFamily" ? (
          <FontFamilyField key={s.id} setting={s} />
        ) : (
          <TextStyleField key={s.id} setting={s} />
        ),
      )}
    </div>
  );
}

// Edits are debounced and round-trip through the server, so the field keeps a
// local draft and only adopts the server value when it is not being edited.
function useDraft(serverValue: string) {
  const [draft, setDraft] = useState(serverValue);
  const [focused, setFocused] = useState(false);
  useEffect(() => {
    if (!focused) setDraft(serverValue);
  }, [serverValue, focused]);
  return { draft, setDraft, setFocused };
}

function TextStyleField({ setting }: { setting: StyleSetting }) {
  const { applyLocal } = useEditor();
  const { draft, setDraft, setFocused } = useDraft(setting.value);
  return (
    <Field label={labelFor(setting.key)}>
      <Input
        value={draft}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onChange={(e) => {
          setDraft(e.target.value);
          applyLocal({ op: "setStyle", id: setting.id, value: e.target.value });
        }}
      />
    </Field>
  );
}

function FontFamilyField({ setting }: { setting: StyleSetting }) {
  const { applyLocal } = useEditor();
  const fonts = useSupportedFonts();
  const [value, setValue] = useState(setting.value);
  useEffect(() => setValue(setting.value), [setting.value]);

  if (fonts === null) {
    return (
      <Field label={labelFor(setting.key)}>
        <Skeleton className="h-8 w-full" />
      </Field>
    );
  }

  // Only package fonts the compiler is known to have are selectable. A
  // template with a custom font name keeps a plain text field.
  if (!fonts.some((f) => f.value === setting.value)) {
    return (
      <TextStyleField setting={setting} />
    );
  }

  return (
    <Field label={labelFor(setting.key)} description="Changing the font recompiles the preview.">
      <Select
        value={value}
        onValueChange={(v) => {
          if (!v) return;
          setValue(v);
          applyLocal({ op: "setStyle", id: setting.id, value: v });
        }}
      >
        <SelectTrigger className="w-full">
          <SelectValue>{fonts.find((f) => f.value === value)?.label ?? value}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {fonts.map((f) => (
            <SelectItem key={f.value} value={f.value}>
              {f.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  );
}

function labelFor(key: string): string {
  if (key.startsWith("color:")) return `Color: ${key.slice(6)}`;
  switch (key) {
    case "pageSize":
      return "Page size";
    case "margin":
      return "Margin";
    case "fontSize":
      return "Font size";
    case "fontFamily":
      return "Font family";
    case "sectionSpacing":
      return "Section spacing";
    case "bulletSpacing":
      return "Bullet spacing";
    case "headingFormat":
      return "Heading format";
    default:
      return key;
  }
}
