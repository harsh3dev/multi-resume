"use client";

import { Edit2 } from "@/icons";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/components/actions/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogIcon,
  DialogTitle,
} from "@/components/ui/components/overlays/dialog";
import { Field } from "@/components/ui/components/forms/field";
import { AppIcon } from "@/icons";
import { Input } from "@/components/ui/components/forms/input";

/**
 * Generic "rename" dialog (resumes and versions). `onSubmit` resolves to close
 * the dialog and throws to keep it open for a retry (the caller shows why).
 */
export function RenameDialog({
  open,
  onOpenChange,
  title,
  description,
  label,
  initialValue,
  maxLength,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  label: string;
  initialValue: string;
  maxLength: number;
  onSubmit: (value: string) => Promise<void>;
}) {
  const [value, setValue] = useState(initialValue);
  const [saving, setSaving] = useState(false);

  // Start from the current name each time it opens.
  useEffect(() => {
    if (open) setValue(initialValue);
  }, [open, initialValue]);

  const trimmed = value.trim();
  const error = trimmed.length > maxLength ? `Use ${maxLength} characters or fewer.` : undefined;
  const unchanged = trimmed === initialValue.trim();

  async function submit() {
    if (!trimmed || error || unchanged) return;
    setSaving(true);
    try {
      await onSubmit(trimmed);
      onOpenChange(false);
    } catch {
      // The caller already toasted; keep the dialog open.
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <DialogHeader className="flex-row items-center gap-3">
            <DialogIcon>
              <AppIcon icon={Edit2} size="lg" variant="Bulk" />
            </DialogIcon>
            <div className="flex flex-col gap-1">
              <DialogTitle>{title}</DialogTitle>
              {description ? <DialogDescription>{description}</DialogDescription> : null}
            </div>
          </DialogHeader>
          <Field label={label} error={error}>
            <Input autoFocus value={value} onChange={(e) => setValue(e.target.value)} />
          </Field>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" loading={saving} disabled={!trimmed || !!error || unchanged}>
              Rename
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
