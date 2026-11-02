"use client";

import { ArrowDown2, DocumentDownload } from "@/icons";
import { useState } from "react";

import { Button } from "@/components/ui/components/actions/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/components/actions/dropdown-menu";
import { AppIcon } from "@/icons";
import { downloadDraft, type DownloadKind } from "@/lib/utils/download";
import { saveErrorMessage } from "@/components/editor/components/controls/save-button";
import { selectNeedsSave, useEditor } from "@/lib/store/editor";
import { toast } from "@/lib/store/toast";

const FORMATS: { kind: DownloadKind; label: string }[] = [
  { kind: "pdf", label: "PDF" },
  { kind: "tex", label: "LaTeX (.tex)" },
];

export function DownloadMenu({ name }: { name: string }) {
  const [busy, setBusy] = useState(false);

  async function run(kind: DownloadKind) {
    setBusy(true);
    try {
      // The file must match what the user sees as synced. A PDF export with
      // unsynced changes syncs first (which refreshes the preview too); a .tex
      // export only needs the pending edits flushed.
      if (kind === "pdf" && selectNeedsSave(useEditor.getState())) {
        const { compileError } = await useEditor.getState().recompile();
        if (compileError) {
          toast.error("Sync failed", compileError);
          return;
        }
      } else if (useEditor.getState().pendingOps.size > 0) {
        await useEditor.getState().flush();
      }
      await downloadDraft(useEditor.getState().draftId, kind, name);
      toast.success("Download ready", `${name}.${kind}`);
    } catch (e) {
      toast.error("Download failed", saveErrorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="outline" size="sm" loading={busy} aria-label="Download" className="shrink-0">
            <AppIcon icon={DocumentDownload} />
            <span className="max-sm:hidden">Download</span>
            <AppIcon icon={ArrowDown2} size={12} className="max-sm:hidden" />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-44">
        {FORMATS.map((f) => (
          <DropdownMenuItem key={f.kind} onClick={() => void run(f.kind)}>
            {f.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
