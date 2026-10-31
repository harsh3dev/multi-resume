"use client";

import { AnimatePresence, motion } from "framer-motion";
import { ArrowDown2, DocumentText, Edit2, Trash } from "iconsax-react";
import Link from "next/link";
import { useCallback, useState } from "react";

import { DeleteResumeDialog } from "@/components/dashboard/delete-resume-dialog";
import { RenameDialog } from "@/components/rename-dialog";
import { DeleteVersionDialog } from "@/components/versions/delete-version-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { AppIcon } from "@/components/ui/icon";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { ResumeSummary } from "@/app/(app)/dashboard/page";
import { IconButton } from "@/components/ui/icon-button";
import { apiJSON } from "@/lib/api/client";
import { renameResume, renameVersion } from "@/lib/rename";
import { cn } from "@/lib/utils";

export type VariationSummary = {
  id: string;
  name: string;
  isDefault: boolean;
  draftId: string | null;
  createdAt: string;
};

export function ResumeRow({
  resume,
  onChanged,
}: {
  resume: ResumeSummary;
  onChanged: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [variations, setVariations] = useState<VariationSummary[] | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [versionToRename, setVersionToRename] = useState<VariationSummary | null>(null);
  const [versionToDelete, setVersionToDelete] = useState<VariationSummary | null>(null);

  const loadVariations = useCallback(async () => {
    try {
      const data = await apiJSON<{ variations: VariationSummary[] }>(
        `/api/resumes/${resume.id}/variations`,
      );
      // Original (default) first, then the rest in creation order.
      setVariations([...data.variations].sort((a, b) => Number(b.isDefault) - Number(a.isDefault)));
    } catch {
      setVariations((cur) => cur ?? []);
    }
  }, [resume.id]);

  async function toggle() {
    const next = !open;
    setOpen(next);
    // Load on every expand: show what is cached while revalidating, so
    // variations created elsewhere appear without a page reload.
    if (next) await loadVariations();
  }

  return (
    <li className="overflow-hidden rounded-xl border border-border bg-card">
      <div className="flex items-center gap-2 p-3">
        <button
          type="button"
          onClick={() => void toggle()}
          aria-expanded={open}
          className="flex min-w-0 flex-1 items-center gap-3 rounded-lg text-left outline-none transition-colors hover:bg-accent/50 focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <AppIcon
            icon={ArrowDown2}
            className={cn("text-muted-foreground transition-transform duration-200", !open && "-rotate-90")}
          />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-medium">{resume.title}</span>
            <span className="block text-xs text-muted-foreground">{resume.createdAt}</span>
          </span>
          <Badge variant="secondary">{resume.compiler}</Badge>
        </button>
        <IconButton
          icon={Edit2}
          label={`Rename ${resume.title}`}
          onClick={() => setRenameOpen(true)}
        />
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`Delete ${resume.title}`}
                onClick={() => setDeleteOpen(true)}
              />
            }
          >
            <AppIcon icon={Trash} />
          </TooltipTrigger>
          <TooltipContent>Delete resume</TooltipContent>
        </Tooltip>
      </div>

      <AnimatePresence initial={false}>
        {open ? (
          <motion.div
            key="variations"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ type: "spring", bounce: 0, duration: 0.25 }}
            className="overflow-hidden"
          >
            <div className="border-t border-border bg-muted/30 p-2">
              {variations === null ? (
                <div className="flex flex-col gap-2 p-1">
                  <Skeleton className="h-9 w-full" />
                  <Skeleton className="h-9 w-full" />
                </div>
              ) : variations.length === 0 ? (
                <p className="p-2 text-xs text-muted-foreground">
                  No versions found for this resume.
                </p>
              ) : (
                <ul className="flex flex-col">
                  {variations.map((v) => (
                    // The link and the actions are siblings: buttons can't live inside a link.
                    <li key={v.id} className="flex items-center gap-1 rounded-lg transition-colors hover:bg-accent/50">
                      <Link
                        href={`/resume/${resume.id}/${v.id}`}
                        className="flex min-w-0 flex-1 items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm transition-colors hover:text-accent-foreground"
                      >
                        <AppIcon icon={DocumentText} className="text-muted-foreground" />
                        <span className="flex-1 truncate">{v.name}</span>
                        {v.isDefault ? <Badge variant="outline">Default</Badge> : null}
                      </Link>
                      <IconButton icon={Edit2} label={`Rename ${v.name}`} onClick={() => setVersionToRename(v)} />
                      <IconButton
                        icon={Trash}
                        label={v.isDefault ? "The default version can't be deleted" : `Delete ${v.name}`}
                        disabled={v.isDefault}
                        onClick={() => setVersionToDelete(v)}
                      />
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </motion.div>
        ) : null}
      </AnimatePresence>

      <DeleteResumeDialog
        resume={resume}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={onChanged}
      />
      <RenameDialog
        open={renameOpen}
        onOpenChange={setRenameOpen}
        title="Rename resume"
        description="This is the name shown on your dashboard."
        label="Resume name"
        maxLength={200}
        initialValue={resume.title}
        onSubmit={async (title) => {
          await renameResume(resume.id, title);
          onChanged();
        }}
      />
      {versionToRename ? (
        <RenameDialog
          open
          onOpenChange={(o) => !o && setVersionToRename(null)}
          title="Rename version"
          label="Version name"
          maxLength={120}
          initialValue={versionToRename.name}
          onSubmit={async (name) => {
            await renameVersion(versionToRename.id, name);
            await loadVariations();
          }}
        />
      ) : null}
      {versionToDelete ? (
        <DeleteVersionDialog
          version={versionToDelete}
          open
          onOpenChange={(o) => !o && setVersionToDelete(null)}
          onDeleted={() => void loadVariations()}
        />
      ) : null}
    </li>
  );
}
