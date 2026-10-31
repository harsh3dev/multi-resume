"use client";

import { Trash } from "iconsax-react";
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
import { AppIcon } from "@/components/ui/icon";
import type { ResumeSummary } from "@/app/(app)/dashboard/page";
import { apiFetch } from "@/lib/api/client";
import { toast } from "@/lib/store/toast";

export function DeleteResumeDialog({
  resume,
  open,
  onOpenChange,
  onDeleted,
}: {
  resume: ResumeSummary;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
}) {
  const [deleting, setDeleting] = useState(false);

  async function remove() {
    setDeleting(true);
    try {
      const res = await apiFetch(`/api/resumes/${resume.id}`, { method: "DELETE" });
      if (!res.ok && res.status !== 204) {
        throw new Error("delete failed");
      }
      toast.success("Resume deleted", resume.title);
      onOpenChange(false);
      onDeleted();
    } catch {
      toast.error("Could not delete resume", "Please try again.");
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
            <DialogTitle>Delete {resume.title}</DialogTitle>
            <DialogDescription>
              This permanently removes the resume and all its variations. This
              action cannot be undone.
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
