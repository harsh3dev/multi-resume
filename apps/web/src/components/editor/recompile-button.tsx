"use client";

import { Refresh2 } from "iconsax-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { AppIcon } from "@/components/ui/icon";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useEditor } from "@/lib/store/editor";
import { toast } from "@/lib/store/toast";

/**
 * Header action, available on every tab: saves any pending field edits and
 * unsaved Source-tab text, re-parses the document and recompiles the PDF.
 */
export function RecompileButton() {
  const [busy, setBusy] = useState(false);
  const dirtySource = useEditor((s) => s.sourceDraft !== null);

  async function run() {
    setBusy(true);
    try {
      const { compileError } = await useEditor.getState().recompile();
      if (compileError) toast.error("Compile failed", compileError);
      else toast.success("Recompiled", "The preview is up to date.");
    } catch (e) {
      toast.error("Could not recompile", e instanceof Error ? e.message : undefined);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button variant={dirtySource ? "default" : "outline"} size="sm" loading={busy} onClick={() => void run()}>
            <AppIcon icon={Refresh2} />
            Recompile
          </Button>
        }
      />
      <TooltipContent>
        {dirtySource ? "Apply your source edits and recompile" : "Recompile the PDF"}
      </TooltipContent>
    </Tooltip>
  );
}
