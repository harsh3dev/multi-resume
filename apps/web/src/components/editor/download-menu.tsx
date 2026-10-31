"use client";

import { ArrowDown2, DocumentDownload } from "iconsax-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AppIcon } from "@/components/ui/icon";
import { downloadDraft, type DownloadKind } from "@/lib/download";
import { useEditor } from "@/lib/store/editor";
import { toast } from "@/lib/store/toast";

const FORMATS: { kind: DownloadKind; label: string }[] = [
  { kind: "pdf", label: "PDF" },
  { kind: "tex", label: "LaTeX (.tex)" },
];

export function DownloadMenu({ name }: { name: string }) {
  const [busy, setBusy] = useState(false);

  async function run(kind: DownloadKind) {
    const { draftId, pendingOps, flush } = useEditor.getState();
    setBusy(true);
    try {
      // Make sure the file reflects the latest edits.
      if (pendingOps.size > 0) await flush();
      await downloadDraft(draftId, kind, name);
      toast.success("Download ready", `${name}.${kind}`);
    } catch (e) {
      toast.error("Download failed", e instanceof Error ? e.message : undefined);
    } finally {
      setBusy(false);
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="outline" size="sm" loading={busy}>
            <AppIcon icon={DocumentDownload} />
            Download
            <AppIcon icon={ArrowDown2} size={12} />
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
