"use client";

import CodeMirror from "@uiw/react-codemirror";
import { StreamLanguage } from "@codemirror/language";
import { stex } from "@codemirror/legacy-modes/mode/stex";
import { useTheme } from "next-themes";

import { Button } from "@/components/ui/components/actions/button";
import { useEditor } from "@/lib/store/editor";

// SourceTab edits raw .tex. The text lives in the editor store (not here), so
// it follows edits made in the other tabs and survives tab switches. It is
// applied with the header's Recompile button.
export function SourceTab() {
  const serverSource = useEditor((s) => s.sourceTex);
  const draft = useEditor((s) => s.sourceDraft);
  const setSourceDraft = useEditor((s) => s.setSourceDraft);
  const { resolvedTheme } = useTheme();

  // You have unsaved source edits AND the resume changed elsewhere.
  const outdated = draft !== null && draft.base !== serverSource;

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 p-3">
      {outdated ? (
        <div
          role="alert"
          className="flex items-center justify-between gap-2 rounded-lg border border-warning/40 bg-warning/10 p-2 text-xs text-warning"
        >
          The resume changed elsewhere while you had unsaved source edits. Recompiling will overwrite those changes.
          <Button size="xs" variant="outline" onClick={() => setSourceDraft(null)}>
            Reload source
          </Button>
        </div>
      ) : null}
      <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-border font-mono text-xs">
        <CodeMirror
          value={draft?.text ?? serverSource}
          height="100%"
          minHeight="320px"
          theme={resolvedTheme === "dark" ? "dark" : "light"}
          extensions={[StreamLanguage.define(stex)]}
          onChange={setSourceDraft}
        />
      </div>
    </div>
  );
}
