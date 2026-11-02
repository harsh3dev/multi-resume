"use client";

import { ArrowDown2, DocumentDownload, Edit2, Layer, Trash } from "@/icons";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { RenameDialog } from "@/components/rename-dialog";
import { Badge } from "@/components/ui/components/display/badge";
import { Button, buttonVariants } from "@/components/ui/components/actions/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/components/actions/dropdown-menu";
import { EmptyState } from "@/components/ui/components/display/empty-state";
import { AppIcon } from "@/icons";
import { IconButton } from "@/components/ui/components/actions/icon-button";
import { Skeleton } from "@/components/ui/components/display/skeleton";
import { DeleteVersionDialog } from "@/components/versions/delete-version-dialog";
import { apiJSON } from "@/lib/api/client";
import { downloadDraft, type DownloadKind } from "@/lib/utils/download";
import { renameVersion } from "@/lib/utils/rename";
import { toast } from "@/lib/store/toast";

// A version IS a variation: a named, editable copy of the resume with its
// own draft. There is no separate snapshot concept.
type VersionEntry = {
  id: string;
  name: string;
  isDefault: boolean;
  draftId: string | null;
  createdAt: string;
};

export function VersionList({
  resumeId,
  variantId,
  onRenamed,
  onSaveAsVersion,
  versionsRevision,
}: {
  resumeId: string;
  variantId: string;
  /** Called after a version is renamed, so the page can refresh the header. */
  onRenamed?: (id: string, name: string) => void;
  onSaveAsVersion: () => void;
  versionsRevision: number;
}) {
  const [versions, setVersions] = useState<VersionEntry[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<VersionEntry | null>(null);
  const [toRename, setToRename] = useState<VersionEntry | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await apiJSON<{ variations: VersionEntry[] }>(`/api/resumes/${resumeId}/variations`);
      // Default ("Original") first, then creation order.
      setVersions([...data.variations].sort((a, b) => Number(b.isDefault) - Number(a.isDefault)));
      setLoadError(null);
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : "failed to load versions");
    }
  }, [resumeId]);

  useEffect(() => {
    void load();
  }, [load, versionsRevision]);

  async function download(v: VersionEntry, kind: DownloadKind) {
    if (!v.draftId) return;
    setBusyId(v.id);
    try {
      await downloadDraft(v.draftId, kind, v.name);
      toast.success("Download ready", `${v.name}.${kind}`);
    } catch (e) {
      toast.error("Download failed", e instanceof Error ? e.message : undefined);
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="flex flex-col gap-3 p-4">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          Each version is its own editable copy of this resume.
        </p>
        <Button size="sm" onClick={onSaveAsVersion}>
          Save as version
        </Button>
      </div>

      {loadError ? <p className="text-xs text-destructive">{loadError}</p> : null}

      {versions === null && !loadError ? (
        <div className="flex flex-col gap-2" aria-busy>
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </div>
      ) : versions && versions.length === 0 ? (
        <EmptyState
          icon={Layer}
          title="No versions yet"
          description="Save the current draft as a version to keep an editable copy."
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {versions?.map((v) => {
            const current = v.id === variantId;
            return (
              <li
                key={v.id}
                className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-border bg-card p-3 transition-colors hover:bg-accent/50"
              >
                <div className="flex min-w-0 flex-col gap-1">
                  <span className="truncate text-sm font-medium">{v.name}</span>
                  <div className="flex flex-wrap items-center gap-1.5">
                    {current ? <Badge>Editing</Badge> : null}
                    {v.isDefault ? <Badge variant="outline">Default</Badge> : null}
                    <span className="text-xs text-muted-foreground">{v.createdAt}</span>
                  </div>
                </div>
                <div className="flex items-center gap-1.5">
                  {!current ? (
                    <Link
                      href={`/resume/${resumeId}/${v.id}`}
                      className={buttonVariants({ variant: "ghost", size: "sm" })}
                    >
                      Open
                    </Link>
                  ) : null}
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      render={
                        <Button variant="outline" size="sm" loading={busyId === v.id} disabled={!v.draftId}>
                          <AppIcon icon={DocumentDownload} />
                          Download
                          <AppIcon icon={ArrowDown2} size={12} />
                        </Button>
                      }
                    />
                    <DropdownMenuContent align="end" className="w-40">
                      <DropdownMenuItem onClick={() => void download(v, "pdf")}>PDF</DropdownMenuItem>
                      <DropdownMenuItem onClick={() => void download(v, "tex")}>
                        LaTeX (.tex)
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                  <IconButton icon={Edit2} label="Rename version" onClick={() => setToRename(v)} />
                  <IconButton
                    icon={Trash}
                    label={
                      v.isDefault
                        ? "The default version can't be deleted"
                        : current
                          ? "Open another version to delete this one"
                          : "Delete version"
                    }
                    disabled={v.isDefault || current}
                    onClick={() => setToDelete(v)}
                  />
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {toRename ? (
        <RenameDialog
          open
          onOpenChange={(o) => !o && setToRename(null)}
          title="Rename version"
          label="Version name"
          maxLength={120}
          initialValue={toRename.name}
          onSubmit={async (name) => {
            await renameVersion(toRename.id, name);
            onRenamed?.(toRename.id, name);
            await load();
          }}
        />
      ) : null}
      {toDelete ? (
        <DeleteVersionDialog
          version={toDelete}
          open
          onOpenChange={(o) => !o && setToDelete(null)}
          onDeleted={() => void load()}
        />
      ) : null}
    </div>
  );
}
