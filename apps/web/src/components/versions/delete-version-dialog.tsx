"use client";

import { Trash } from "@/icons";
import { useState } from "react";

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
import { AppIcon } from "@/icons";
import { apiFetch } from "@/lib/api/client";
import { toast } from "@/lib/store/toast";

export function DeleteVersionDialog({
  version,
  open,
  onOpenChange,
  onDeleted,
}: {
  version: { id: string; name: string };
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
}) {
  const [deleting, setDeleting] = useState(false);

  async function remove() {
    setDeleting(true);
    try {
      const res = await apiFetch(`/api/variations/${version.id}`, { method: "DELETE" });
      if (!res.ok && res.status !== 204) throw new Error("delete failed");
      toast.success("Version deleted", version.name);
      onOpenChange(false);
      onDeleted();
    } catch {
      toast.error("Could not delete version", "Please try again.");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader className="flex-row items-center gap-3">
          <DialogIcon className="bg-destructive/10 text-destructive">
            <AppIcon icon={Trash} size="lg" variant="Bulk" />
          </DialogIcon>
          <div className="flex flex-col gap-1">
            <DialogTitle>Delete {version.name}</DialogTitle>
            <DialogDescription>
              This permanently removes this version and its content. This action cannot be undone.
            </DialogDescription>
          </div>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="destructive" loading={deleting} onClick={() => void remove()}>
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
