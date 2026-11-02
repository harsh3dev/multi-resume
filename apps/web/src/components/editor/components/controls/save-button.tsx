"use client";

import { Refresh2 } from "@/icons";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/components/actions/button";
import { AppIcon } from "@/icons";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/components/actions/tooltip";
import { selectNeedsSave, useEditor } from "@/lib/store/editor";
import { toast } from "@/lib/store/toast";

/** Message shown for a failed save request, in the user's terms. */
export function saveErrorMessage(e: unknown): string | undefined {
  if (e instanceof Error && e.message === "rate_limited") {
    return "You're syncing too quickly. Wait a moment and try again.";
  }
  return e instanceof Error ? e.message : undefined;
}

/**
 * Preview-toolbar action, available on every tab. Edits autosave to the server, but the
 * PDF preview only refreshes here (compiling is the costly step), so to the
 * user "preview out of date" reads as "unsynced changes" and this button is
 * highlighted until it's pressed. Also bound to Ctrl/Cmd+S.
 */
export function SaveButton() {
  const [busy, setBusy] = useState(false);
  const needsSave = useEditor(selectNeedsSave);
  const running = useRef(false);

  async function run() {
    if (running.current) return;
    running.current = true;
    setBusy(true);
    try {
      const { compileError } = await useEditor.getState().recompile();
      if (compileError) toast.error("Sync failed", compileError);
      else toast.success("Synced", "The preview is up to date.");
    } catch (e) {
      toast.error("Could not sync", saveErrorMessage(e));
    } finally {
      running.current = false;
      setBusy(false);
    }
  }

  // Ctrl/Cmd+S saves instead of opening the browser's "save page" dialog.
  const runRef = useRef(run);
  runRef.current = run;
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && !e.altKey && e.key.toLowerCase() === "s") {
        e.preventDefault();
        void runRef.current();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant={needsSave ? "default" : "outline"}
            size="sm"
            loading={busy}
            aria-label="Sync PDF"
            className="shrink-0"
            onClick={() => void run()}
          >
            <AppIcon icon={Refresh2} />
            <span className="max-sm:hidden">Sync PDF</span>
          </Button>
        }
      />
      <TooltipContent>
        {needsSave ? "Sync your changes to the PDF preview (Ctrl/⌘ S)" : "The PDF is up to date"}
      </TooltipContent>
    </Tooltip>
  );
}
