import { apiJSON } from "@/lib/api/client";

export type DownloadKind = "pdf" | "tex";

const MIME: Record<DownloadKind, string> = {
  pdf: "application/pdf",
  tex: "text/x-tex",
};

function safeName(name: string): string {
  const cleaned = name.replace(/[^\w.-]+/g, "_").replace(/^_+|_+$/g, "");
  return cleaned || "resume";
}

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}

async function pdfBlob(res: Response): Promise<Blob> {
  const blob = await res.blob();
  return new Blob([blob], { type: MIME.pdf });
}

/** Download the current draft; re-reads it so the file matches what is saved. */
export async function downloadDraft(draftId: string, kind: DownloadKind, name = "resume") {
  const draft = await apiJSON<{ sourceTex: string; pdfUrl?: string }>(`/api/drafts/${draftId}`);
  if (kind === "tex") {
    saveBlob(new Blob([draft.sourceTex], { type: MIME.tex }), `${safeName(name)}.tex`);
    return;
  }
  if (!draft.pdfUrl) throw new Error("no PDF has been compiled yet");
  const res = await fetch(draft.pdfUrl);
  if (!res.ok) throw new Error(`download failed (${res.status})`);
  saveBlob(await pdfBlob(res), `${safeName(name)}.pdf`);
}
