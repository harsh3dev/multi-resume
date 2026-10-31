"use client";

import { Save2 } from "iconsax-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogIcon,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field } from "@/components/ui/field";
import { AppIcon } from "@/components/ui/icon";
import { Input } from "@/components/ui/input";

const MAX_NAME = 80;

export function NameVersionDialog({
  open,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Resolve to close the dialog; throw to keep it open. */
  onSubmit: (name: string) => Promise<void>;
}) {
  const [name, setName] = useState("Tailored");
  const [saving, setSaving] = useState(false);
  const trimmed = name.trim();
  const error = trimmed.length > MAX_NAME ? `Use ${MAX_NAME} characters or fewer.` : undefined;

  async function submit() {
    if (!trimmed || error) return;
    setSaving(true);
    try {
      await onSubmit(trimmed);
      onOpenChange(false);
    } catch {
      // The caller already toasted; keep the dialog open for a retry.
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
              <AppIcon icon={Save2} size="lg" variant="Bulk" />
            </DialogIcon>
            <div className="flex flex-col gap-1">
              <DialogTitle>Save as version</DialogTitle>
              <DialogDescription>
                Freezes the current draft so you can download or branch from it later.
              </DialogDescription>
            </div>
          </DialogHeader>
          <Field label="Version name" error={error}>
            <Input autoFocus value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" loading={saving} disabled={!trimmed || !!error}>
              Save version
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
